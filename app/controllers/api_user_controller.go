package controllers

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/app/repository"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/entitlements"
	"github.com/ManuelReschke/PixelFox/internal/pkg/usercontext"
)

// HandleGetUserAccount returns account information for the authenticated user (API key or session).
func HandleGetUserAccount(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	if !userCtx.IsLoggedIn {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized", "message": "Missing or invalid authentication"})
	}

	repo := repository.GetGlobalFactory().GetUserRepository()
	account, err := repo.GetByID(userCtx.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not_found", "message": "User not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal_server_error", "message": "Failed to load user"})
	}

	stats, err := repo.GetStatsByUserID(userCtx.UserID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal_server_error", "message": "Failed to load statistics"})
	}

	db := database.GetDB()
	if db == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal_server_error", "message": "Database unavailable"})
	}
	settings, err := models.GetOrCreateUserSettings(db, userCtx.UserID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal_server_error", "message": "Failed to load user settings"})
	}

	plan := entitlements.Plan(settings.Plan)
	if plan == "" {
		plan = entitlements.PlanFree
	}

	maxUpload := entitlements.MaxUploadBytes(plan)
	quotaBytes := entitlements.StorageQuotaBytes(plan)
	var quotaValue interface{}
	var quotaRemaining interface{}
	if quotaBytes > 0 {
		quotaValue = quotaBytes
		remaining := quotaBytes - stats.StorageUsage
		if remaining < 0 {
			remaining = 0
		}
		quotaRemaining = remaining
	}

	canOrig, canWebp, canAvif := entitlements.AllowedThumbs(plan)
	appSettings := models.GetAppSettings()

	allowedFormats := make([]string, 0, 3)
	if canOrig && appSettings.IsThumbnailOriginalEnabled() {
		allowedFormats = append(allowedFormats, "original")
	}
	if canWebp && appSettings.IsThumbnailWebPEnabled() {
		allowedFormats = append(allowedFormats, "webp")
	}
	if canAvif && appSettings.IsThumbnailAVIFEnabled() {
		allowedFormats = append(allowedFormats, "avif")
	}

	response := fiber.Map{
		"id":                   account.ID,
		"username":             account.Name,
		"email":                account.Email,
		"status":               account.Status,
		"plan":                 settings.Plan,
		"is_admin":             account.Role == models.ROLE_ADMIN,
		"created_at":           account.CreatedAt.UTC().Format(time.RFC3339),
		"last_login_at":        formatTimePtr(account.LastLoginAt),
		"api_key_last_used_at": formatTimePtr(settings.APIKeyLastUsedAt),
		"stats": fiber.Map{
			"images": fiber.Map{
				"count":                   stats.ImageCount,
				"storage_used_bytes":      stats.StorageUsage,
				"storage_remaining_bytes": quotaRemaining,
			},
			"albums": fiber.Map{
				"count": stats.AlbumCount,
			},
		},
		"limits": fiber.Map{
			"max_upload_bytes":          maxUpload,
			"storage_quota_bytes":       quotaValue,
			"can_multi_upload":          entitlements.CanMultiUpload(plan),
			"image_upload_enabled":      appSettings.IsImageUploadEnabled(),
			"direct_upload_enabled":     appSettings.IsDirectUploadEnabled(),
			"allowed_thumbnail_formats": allowedFormats,
		},
		"preferences": fiber.Map{
			"thumbnail_original": settings.PrefThumbOriginal,
			"thumbnail_webp":     settings.PrefThumbWebP,
			"thumbnail_avif":     settings.PrefThumbAVIF,
		},
	}

	return c.JSON(response)
}

func formatTimePtr(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}
