package middleware

import (
	"errors"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/app/repository"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/usercontext"
)

// APIKeyAuthMiddleware authenticates requests carrying a user API key header.
func APIKeyAuthMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiKey := extractAPIKeyFromHeader(c)
		if apiKey == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized", "message": "Missing API key"})
		}

		db := database.GetDB()
		if db == nil {
			log.Print("api key middleware: database unavailable")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal_server_error", "message": "Database unavailable"})
		}

		hash := models.HashAPIKey(apiKey)
		repo := repository.GetGlobalFactory().GetUserRepository()
		user, settings, err := repo.GetByAPIKeyHash(hash)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized", "message": "Invalid API key"})
			}
			log.Printf("api key lookup failed: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal_server_error", "message": "API key verification failed"})
		}

		if user.Status != models.STATUS_ACTIVE {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "forbidden", "message": "User inactive"})
		}

		if settings.Plan == "" {
			settings.Plan = "free"
		}

		// Refresh last-used timestamp best-effort.
		now := time.Now()
		if err := db.Model(&models.UserSettings{}).
			Where("id = ?", settings.ID).
			Updates(map[string]any{"api_key_last_used_at": now}).Error; err != nil {
			log.Printf("failed to update api key usage timestamp for user %d: %v", user.ID, err)
		}

		userCtx := usercontext.UserContext{
			UserID:     user.ID,
			Username:   user.Name,
			IsLoggedIn: true,
			IsAdmin:    user.Role == models.ROLE_ADMIN,
			Plan:       settings.Plan,
		}
		c.Locals("USER_CONTEXT", userCtx)
		c.Locals(usercontext.KeyFromProtected, true)
		c.Locals(usercontext.KeyUserID, user.ID)
		c.Locals(usercontext.KeyUsername, user.Name)
		c.Locals(usercontext.KeyIsAdmin, user.Role == models.ROLE_ADMIN)

		return c.Next()
	}
}

func extractAPIKeyFromHeader(c *fiber.Ctx) string {
	apiKey := strings.TrimSpace(c.Get("X-API-Key"))
	if apiKey != "" {
		return apiKey
	}
	auth := strings.TrimSpace(c.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	return ""
}
