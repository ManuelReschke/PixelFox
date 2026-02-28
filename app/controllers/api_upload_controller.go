package controllers

import (
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	fiberlog "github.com/gofiber/fiber/v2/log"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/app/repository"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/entitlements"
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
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized", "message": "Missing or invalid authentication"})
	}

	var req struct {
		FileSize int64 `json:"file_size"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request", "message": "Invalid request body"})
	}
	payload, status, errCode, errMsg := createDirectUploadSession(user, req.FileSize)
	if errMsg != "" {
		return c.Status(status).JSON(fiber.Map{"error": errCode, "message": errMsg})
	}
	return c.Status(status).JSON(payload)
}

// HandleCreateUploadSessionAPI issues a direct upload session via API key authentication.
func HandleCreateUploadSessionAPI(c *fiber.Ctx) error {
	user := usercontext.GetUserContext(c)
	if !user.IsLoggedIn {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized", "message": "Missing or invalid API key"})
	}

	var req struct {
		FileSize int64 `json:"file_size"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request", "message": "Invalid request body"})
	}

	payload, status, errCode, errMsg := createDirectUploadSession(user, req.FileSize)
	if errMsg != "" {
		return c.Status(status).JSON(fiber.Map{"error": errCode, "message": errMsg})
	}
	return c.Status(status).JSON(payload)
}

// HandleImageStatusJSON returns processing status for an image (JSON)
func HandleImageStatusJSON(c *fiber.Ctx) error {
	uuid := c.Params("uuid")
	if uuid == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request", "message": "uuid missing"})
	}
	complete := imageprocessor.IsImageProcessingComplete(uuid)

	// try to fetch view url
	var viewURL *string
	if complete {
		imgRepo := repository.GetGlobalFactory().GetImageRepository()
		if image, err := imgRepo.GetByUUID(uuid); err == nil && image != nil {
			u := "/i/" + image.ShareLink
			viewURL = &u
		}
	}
	return c.JSON(fiber.Map{"complete": complete, "view_url": viewURL})
}

func createDirectUploadSession(user usercontext.UserContext, requestedSize int64) (fiber.Map, int, string, string) {
	if requestedSize <= 0 {
		return nil, fiber.StatusBadRequest, "bad_request", "file_size must be > 0"
	}

	// Determine effective plan from DB to avoid stale/empty session values
	planName := strings.ToLower(strings.TrimSpace(user.Plan))
	if db := database.GetDB(); db != nil {
		if us, err := models.GetOrCreateUserSettings(db, user.UserID); err == nil && us != nil && strings.TrimSpace(us.Plan) != "" {
			planName = strings.ToLower(strings.TrimSpace(us.Plan))
		}
	}
	if planName == "" {
		planName = string(entitlements.PlanFree)
	}
	plan := entitlements.Plan(planName)

	sm := storage.NewStorageManager()
	pool, err := sm.SelectPoolForUpload(requestedSize)
	if err != nil || pool == nil {
		fiberlog.Error(fmt.Sprintf("select pool error: %v", err))
		return nil, fiber.StatusInternalServerError, "no_storage_pool", "no storage pool available"
	}
	if pool.UploadAPIURL == "" {
		return nil, fiber.StatusServiceUnavailable, "pool_misconfigured", "pool missing upload_api_url"
	}

	planLimit := entitlements.MaxUploadBytes(plan)
	// Defensive: never return forbidden due to a misconfigured/empty plan; fall back to free
	if planLimit <= 0 {
		planLimit = entitlements.MaxUploadBytes(entitlements.PlanFree)
	}
	maxBytes := requestedSize
	if maxBytes <= 0 || maxBytes > planLimit {
		maxBytes = planLimit
	}
	if maxBytes <= 0 {
		// Defensive fallback: never forbid here; cap to plan limit (free as minimum)
		maxBytes = entitlements.MaxUploadBytes(entitlements.PlanFree)
		if maxBytes <= 0 {
			maxBytes = 1024 * 1024 // 1 MiB minimal safety cap
		}
	}

	if quota := entitlements.StorageQuotaBytes(plan); quota > 0 {
		var used int64
		if db := database.GetDB(); db != nil {
			db.Model(&models.Image{}).
				Where("user_id = ?", user.UserID).
				Select("COALESCE(SUM(file_size), 0)").Row().Scan(&used)
		}
		remaining := quota - used
		if remaining <= 0 {
			return nil, fiber.StatusRequestEntityTooLarge, "quota_exceeded", "storage quota exceeded"
		}
		if maxBytes > remaining {
			maxBytes = remaining
		}
	}

	secret := env.GetEnv("UPLOAD_TOKEN_SECRET", "")
	if secret == "" {
		fiberlog.Warn("UPLOAD_TOKEN_SECRET not set; refusing to issue upload session")
		return nil, fiber.StatusServiceUnavailable, "service_unavailable", "upload token secret not configured"
	}

	ttl := 30 * time.Minute
	token, err := security.GenerateUploadToken(user.UserID, pool.ID, maxBytes, ttl, secret)
	if err != nil {
		return nil, fiber.StatusInternalServerError, "token_creation_failed", "failed to create token"
	}

	uploadURL := resolvePublicUploadURL(pool)

	return fiber.Map{
		"upload_url": uploadURL,
		"token":      token,
		"pool_id":    pool.ID,
		"expires_at": time.Now().Add(ttl).Unix(),
		"max_bytes":  maxBytes,
	}, fiber.StatusOK, "", ""
}

// resolvePublicUploadURL returns a client-facing upload URL that never exposes
// internal API routes.
func resolvePublicUploadURL(pool *models.StoragePool) string {
	const publicUploadPath = "/api/v1/upload"

	if pool == nil {
		return publicUploadPath
	}

	if pb := strings.TrimSpace(pool.PublicBaseURL); pb != "" {
		lower := strings.ToLower(pb)
		if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
			return strings.TrimRight(pb, "/") + publicUploadPath
		}
	}

	uploadURL := strings.TrimSpace(pool.UploadAPIURL)
	if uploadURL == "" {
		return publicUploadPath
	}

	if strings.Contains(uploadURL, "/api/internal/upload") {
		return strings.Replace(uploadURL, "/api/internal/upload", publicUploadPath, 1)
	}

	if strings.Contains(uploadURL, publicUploadPath) {
		return uploadURL
	}

	if strings.HasSuffix(uploadURL, "/upload") {
		return strings.TrimSuffix(uploadURL, "/upload") + publicUploadPath
	}

	return uploadURL
}
