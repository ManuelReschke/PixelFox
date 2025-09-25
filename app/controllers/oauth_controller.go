package controllers

import (
	"errors"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	gothfiber "github.com/shareed2k/goth_fiber"
	"gorm.io/gorm"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/session"
)

// HandleOAuthCallback completes the provider flow and logs the user in
func HandleOAuthCallback(c *fiber.Ctx) error {
	// Complete OAuth with provider and obtain unified user
	u, err := gothfiber.CompleteUserAuth(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(fmt.Sprintf("OAuth failed: %v", err))
	}

	db := database.GetDB()

	// Try to find existing provider account
	var pa models.ProviderAccount
	res := db.Where("provider = ? AND provider_user_id = ?", u.Provider, u.UserID).First(&pa)

	var appUser models.User
	if errors.Is(res.Error, gorm.ErrRecordNotFound) {
		// Optional email match if provided
		if u.Email != "" {
			_ = db.Where("email = ?", u.Email).First(&appUser).Error
		}
		if appUser.ID == 0 {
			// Create new user; ensure password is set to a random placeholder since validation requires it
			// Use timestamp-based random string as placeholder (not used for login)
			placeholder := fmt.Sprintf("oauth_%d", time.Now().UnixNano())
			hash, _ := models.HashPassword(placeholder)
			email := u.Email
			if email == "" {
				// Ensure unique, non-empty email to satisfy unique index semantics in MySQL
				email = fmt.Sprintf("%s_%s@%s.oauth.local", u.Provider, u.UserID, u.Provider)
			}
			appUser = models.User{
				Name:      firstNonEmpty(u.Name, u.NickName, u.Email, "User"),
				Email:     email,
				Password:  hash,
				AvatarURL: u.AvatarURL,
				Status:    models.STATUS_ACTIVE,
			}
			if err := db.Create(&appUser).Error; err != nil {
				return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("create user failed: %v", err))
			}
		}
		var exp *time.Time
		if !u.ExpiresAt.IsZero() {
			t := u.ExpiresAt
			exp = &t
		}
		pa = models.ProviderAccount{
			UserID:         appUser.ID,
			Provider:       u.Provider,
			ProviderUserID: u.UserID,
			AccessToken:    u.AccessToken,
			RefreshToken:   u.RefreshToken,
			ExpiresAt:      exp,
		}
		if err := db.Create(&pa).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("link provider failed: %v", err))
		}
	} else if res.Error == nil {
		// Update tokens
		pa.AccessToken = u.AccessToken
		pa.RefreshToken = u.RefreshToken
		if !u.ExpiresAt.IsZero() {
			t := u.ExpiresAt
			pa.ExpiresAt = &t
		} else {
			pa.ExpiresAt = nil
		}
		if err := db.Save(&pa).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("update tokens failed: %v", err))
		}
		// Load related user
		if err := db.First(&appUser, pa.UserID).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("linked user not found")
		}
	} else {
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("db error: %v", res.Error))
	}

	// Create app session
	sess, err := session.GetSessionStore().Get(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("session init failed")
	}
	sess.Set(AUTH_KEY, true)
	sess.Set(USER_ID, appUser.ID)
	sess.Set(USER_NAME, appUser.Name)
	sess.Set(USER_IS_ADMIN, appUser.Role == "admin")
	// Cache user plan in session for navbar/entitlements
	if us, err := models.GetOrCreateUserSettings(db, appUser.ID); err == nil && us != nil {
		if us.Plan == "" {
			session.SetSessionValue(c, "user_plan", "free")
		} else {
			session.SetSessionValue(c, "user_plan", us.Plan)
		}
	}
	if err := sess.Save(); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("session save failed")
	}

	// Update last login timestamp
	_ = db.Model(&appUser).UpdateColumn("last_login_at", time.Now()).Error

	// Ensure HTMX boosted flows perform a full redirect and refresh head/meta
	c.Set("HX-Redirect", "/")
	return c.Redirect("/", fiber.StatusSeeOther)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
