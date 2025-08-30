package controllers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	fiberlog "github.com/gofiber/fiber/v2/log"
	"github.com/google/uuid"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/app/repository"
	"github.com/ManuelReschke/PixelFox/internal/pkg/cache"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
	"github.com/ManuelReschke/PixelFox/internal/pkg/jobqueue"
	"github.com/ManuelReschke/PixelFox/internal/pkg/security"
	"github.com/ManuelReschke/PixelFox/internal/pkg/storage"
	"github.com/ManuelReschke/PixelFox/internal/pkg/upload"
)

func readToken(c *fiber.Ctx) string {
	auth := c.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	t := c.FormValue("token")
	return strings.TrimSpace(t)
}

// HandleStorageDirectUpload verifies token and writes file into the designated pool
// Expects multipart form with field "file" and token via Authorization: Bearer <token> or form field "token"
func HandleStorageDirectUpload(c *fiber.Ctx) error {
	// IP-based rate limit based on settings (uploads per minute)
	if limit := models.GetAppSettings().GetUploadRateLimitPerMinute(); limit > 0 {
		ip := c.IP()
		if ip == "" {
			ip = "unknown"
		}
		rateKey := fmt.Sprintf("rate:upload:%s", ip)
		cli := cache.GetClient()
		if cli != nil {
			ctx := context.Background()
			n, err := cli.Incr(ctx, rateKey).Result()
			if err == nil {
				if n == 1 {
					cli.Expire(ctx, rateKey, 60*time.Second)
				}
				if int(n) > limit {
					return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "rate limit exceeded"})
				}
			}
		}
	}

	token := readToken(c)
	if token == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing token"})
	}
	// Use env secret
	claims, err := security.VerifyUploadToken(token, env.GetEnv("UPLOAD_TOKEN_SECRET", ""))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
	}

	// Per-User Rate Limit (in addition to IP limit)
	if userLimit := models.GetAppSettings().GetUploadUserRateLimitPerMinute(); userLimit > 0 && claims != nil && claims.UserID > 0 {
		cli := cache.GetClient()
		if cli != nil {
			ctx := context.Background()
			rateKey := fmt.Sprintf("rate:upload:user:%d", claims.UserID)
			n, err := cli.Incr(ctx, rateKey).Result()
			if err == nil {
				if n == 1 {
					cli.Expire(ctx, rateKey, 60*time.Second)
				}
				if int(n) > userLimit {
					return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "Du hast dein Upload-Limit erreicht. Bitte warte kurz und versuche es erneut."})
				}
			}
		}
	}

	// Parse file
	form, err := c.MultipartForm()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid multipart form"})
	}
	defer form.RemoveAll()
	files := form.File["file"]
	if len(files) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "file missing"})
	}
	file := files[0]
	if file.Size > claims.MaxBytes {
		return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{"error": "file too large for session"})
	}

	// Validate filename extension and MIME by sniffing the first bytes
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid file name"})
	}
	sniff, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to open file"})
	}
	head := make([]byte, 512)
	n, _ := io.ReadFull(sniff, head)
	if n > 0 {
		head = head[:n]
	}
	sniff.Close()
	if _, verr := upload.ValidateImageBySniff(file.Filename, head); verr != nil {
		return c.Status(fiber.StatusUnsupportedMediaType).JSON(fiber.Map{"error": verr.Error()})
	}

	// Open file to compute hash and then persist
	src, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to open file"})
	}
	defer src.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, src); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to read file"})
	}
	fileHash := hex.EncodeToString(hash.Sum(nil))
	src.Close()
	src, err = file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to reopen file"})
	}
	defer src.Close()

	// Duplicate detection for user
	imgRepo := repository.GetGlobalFactory().GetImageRepository()
	if existing, err := imgRepo.GetByUserIDAndFileHash(claims.UserID, fileHash); err == nil && existing != nil {
		return c.JSON(fiber.Map{"duplicate": true, "image_uuid": existing.UUID, "view_url": "/i/" + existing.ShareLink})
	}

	// Storage path
	sm := storage.NewStorageManager()
	pool, err := models.FindStoragePoolByID(database.GetDB(), claims.PoolID)
	if err != nil || pool == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid pool"})
	}

	// Build relative path and file name
	now := time.Now()
	relativePath := fmt.Sprintf("%d/%02d/%02d", now.Year(), now.Month(), now.Day())
	imageUUID := uuid.New().String()
	fileName := imageUUID + ext

	// Save file using StorageManager.SaveFile to ensure directory creation and usage update
	op, err := sm.SaveFile(src, filepath.Join("original", relativePath, fileName), pool.ID)
	if err != nil || !op.Success {
		fiberlog.Errorf("SaveFile error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to store file"})
	}

	// Persist image record
	mimeExt := ext // reuse
	ipv4, ipv6 := GetClientIP(c)
	image := models.Image{
		UUID:          imageUUID,
		UserID:        claims.UserID,
		StoragePoolID: pool.ID,
		FileName:      fileName,
		FilePath:      filepath.Join("original", relativePath),
		FileSize:      file.Size,
		FileType:      mimeExt,
		Title:         file.Filename,
		FileHash:      fileHash,
		IPv4:          ipv4,
		IPv6:          ipv6,
	}
	if err := imgRepo.Create(&image); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create image record"})
	}

	// Enqueue processing
	if err := jobqueue.ProcessImageUnified(&image); err != nil {
		fiberlog.Errorf("enqueue error: %v", err)
	}

	return c.JSON(fiber.Map{
		"image_uuid": image.UUID,
		"view_url":   "/i/" + image.ShareLink,
	})
}
