package controllers

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	fiberlog "github.com/gofiber/fiber/v2/log"

	"github.com/ManuelReschke/PixelFox/app/repository"
	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
	"github.com/ManuelReschke/PixelFox/internal/pkg/imageprocessor"
	"github.com/ManuelReschke/PixelFox/internal/pkg/security"
	"github.com/ManuelReschke/PixelFox/internal/pkg/storage"
	"github.com/ManuelReschke/PixelFox/internal/pkg/usercontext"
)

// HandleCreateUploadSession issues a direct-to-storage upload session (Phase 2)
// Request: JSON { "file_size": int64 }
// Response: { upload_url, token, pool_id, expires_at }
func HandleCreateUploadSession(c *fiber.Ctx) error {
	user := usercontext.GetUserContext(c)
	if !user.IsLoggedIn {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	var req struct {
		FileSize int64 `json:"file_size"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.FileSize <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "file_size must be > 0"})
	}

	// Select pool (hot-first)
	sm := storage.NewStorageManager()
	pool, err := sm.SelectPoolForUpload(req.FileSize)
	if err != nil || pool == nil {
		fiberlog.Error(fmt.Sprintf("select pool error: %v", err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "no storage pool available"})
	}
	if pool.UploadAPIURL == "" {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "pool missing upload_api_url"})
	}

	// Generate token
	secret := env.GetEnv("UPLOAD_TOKEN_SECRET", "")
	if secret == "" {
		fiberlog.Warn("UPLOAD_TOKEN_SECRET not set; refusing to issue upload session")
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "upload token secret not configured"})
	}
	ttl := 30 * time.Minute
	token, err := security.GenerateUploadToken(user.UserID, pool.ID, req.FileSize, ttl, secret)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create token"})
	}

	return c.JSON(fiber.Map{
		"upload_url": pool.UploadAPIURL,
		"token":      token,
		"pool_id":    pool.ID,
		"expires_at": time.Now().Add(ttl).Unix(),
	})
}

// HandleImageStatusJSON returns processing status for an image (JSON)
func HandleImageStatusJSON(c *fiber.Ctx) error {
	uuid := c.Params("uuid")
	if uuid == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "uuid missing"})
	}
	complete := imageprocessor.IsImageProcessingComplete(uuid)

	// try to fetch view url
	var viewURL string
	if complete {
		imgRepo := repository.GetGlobalFactory().GetImageRepository()
		if image, err := imgRepo.GetByUUID(uuid); err == nil && image != nil {
			viewURL = "/i/" + image.ShareLink
		}
	}
	return c.JSON(fiber.Map{"complete": complete, "view_url": viewURL})
}
