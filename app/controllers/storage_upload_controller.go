package controllers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	fiberlog "github.com/gofiber/fiber/v2/log"
	"github.com/google/uuid"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/app/repository"
	"github.com/ManuelReschke/PixelFox/internal/pkg/cache"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/entitlements"
	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
	"github.com/ManuelReschke/PixelFox/internal/pkg/imageprocessor"
	"github.com/ManuelReschke/PixelFox/internal/pkg/jobqueue"
	"github.com/ManuelReschke/PixelFox/internal/pkg/security"
	"github.com/ManuelReschke/PixelFox/internal/pkg/storage"
	"github.com/ManuelReschke/PixelFox/internal/pkg/upload"
)

// HandleStorageUploadHead is a lightweight reachability probe for the upload endpoint
func HandleStorageUploadHead(c *fiber.Ctx) error {
	// No body, just indicate endpoint is alive
	return c.SendStatus(fiber.StatusNoContent)
}

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
	token := readToken(c)
	if token == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized", "message": "missing token"})
	}
	// Use env secret
	claims, err := security.VerifyUploadToken(token, env.GetEnv("UPLOAD_TOKEN_SECRET", ""))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized", "message": "invalid token"})
	}

	// IP-based rate limit: apply only for anonymous/unauthenticated tokens.
	// Für autorisierte Uploads (mit gültigem Token/claims) verlassen wir uns auf das per‑User‑Limit.
	if claims == nil || claims.UserID == 0 {
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
					} else {
						ttl, _ := cli.TTL(ctx, rateKey).Result()
						if ttl <= 0 || ttl > 2*time.Minute {
							// Repair missing TTL and reset counter if it already exceeded the limit due to a stale key
							if int(n) > limit {
								_ = cli.Set(ctx, rateKey, 1, 60*time.Second).Err()
								n = 1
							} else {
								cli.Expire(ctx, rateKey, 60*time.Second)
							}
						}
					}
					// Expose debug headers for diagnostics
					ttl, _ := cli.TTL(ctx, rateKey).Result()
					c.Set("X-Rate-Limit-Scope", "ip")
					c.Set("X-Rate-Limit", fmt.Sprintf("%d/min", limit))
					c.Set("X-Rate-Count", fmt.Sprintf("%d", n))
					if ttl > 0 {
						c.Set("X-Rate-TTL", fmt.Sprintf("%ds", int(ttl.Seconds())))
					}
					if int(n) > limit {
						return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "too_many_requests", "message": "rate limit exceeded"})
					}
				}
			}
		}
	}

	// Per-User Rate Limit (in addition to IP limit)
	if claims != nil && claims.UserID > 0 {
		userLimit := models.GetAppSettings().GetUploadUserRateLimitPerMinute()
		// Premium: erlauben mindestens MaxFilesPerBatch innerhalb 60s
		// (macht Multi-Upload praktikabel, respektiert gleichzeitig globale Limits)
		if userLimit > 0 {
			if db := database.GetDB(); db != nil {
				if us, err := models.GetOrCreateUserSettings(db, claims.UserID); err == nil && us != nil {
					if mf := entitlements.MaxFilesPerBatch(entitlements.Plan(us.Plan)); mf > userLimit {
						userLimit = mf
					}
				}
			}
			cli := cache.GetClient()
			if cli != nil {
				ctx := context.Background()
				rateKey := fmt.Sprintf("rate:upload:user:%d", claims.UserID)
				n, err := cli.Incr(ctx, rateKey).Result()
				if err == nil {
					if n == 1 {
						cli.Expire(ctx, rateKey, 60*time.Second)
					} else {
						ttl, _ := cli.TTL(ctx, rateKey).Result()
						if ttl <= 0 || ttl > 2*time.Minute {
							// Repair missing TTL and reset counter if it already exceeded the limit due to a stale key
							if int(n) > userLimit {
								_ = cli.Set(ctx, rateKey, 1, 60*time.Second).Err()
								n = 1
							} else {
								cli.Expire(ctx, rateKey, 60*time.Second)
							}
						}
					}
					// Expose debug headers for diagnostics
					ttl, _ := cli.TTL(ctx, rateKey).Result()
					c.Set("X-Rate-Limit-Scope", "user")
					c.Set("X-Rate-Limit-UserID", fmt.Sprintf("%d", claims.UserID))
					c.Set("X-Rate-Limit", fmt.Sprintf("%d/min", userLimit))
					c.Set("X-Rate-Count", fmt.Sprintf("%d", n))
					if ttl > 0 {
						c.Set("X-Rate-TTL", fmt.Sprintf("%ds", int(ttl.Seconds())))
					}
					if int(n) > userLimit {
						return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "too_many_requests", "message": "Du hast dein Upload-Limit erreicht. Bitte warte kurz und versuche es erneut."})
					}
				}
			}
		}
	}

	// Parse file
	form, err := c.MultipartForm()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request", "message": "invalid multipart form"})
	}
	defer form.RemoveAll()
	files := form.File["file"]
	if len(files) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request", "message": "file missing"})
	}
	file := files[0]
	if file.Size > claims.MaxBytes {
		return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{"error": "payload_too_large", "message": "file too large for session"})
	}

	// Enforce per-plan storage quota for the user
	db := database.GetDB()
	if db != nil {
		us, _ := models.GetOrCreateUserSettings(db, claims.UserID)
		quota := entitlements.StorageQuotaBytes(entitlements.Plan(us.Plan))
		if quota > 0 {
			var used int64
			db.Model(&models.Image{}).Where("user_id = ?", claims.UserID).Select("COALESCE(SUM(file_size), 0)").Row().Scan(&used)
			if used+file.Size > quota {
				remaining := quota - used
				if remaining < 0 {
					remaining = 0
				}
				return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{
					"error":     "quota_exceeded",
					"message":   "storage quota exceeded",
					"remaining": remaining,
					"needed":    file.Size,
				})
			}
		}
	}

	// Validate filename extension and MIME by sniffing the first bytes
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request", "message": "invalid file name"})
	}
	sniff, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal_server_error", "message": "failed to open file"})
	}
	head := make([]byte, 512)
	n, _ := io.ReadFull(sniff, head)
	if n > 0 {
		head = head[:n]
	}
	sniff.Close()
	if _, verr := upload.ValidateImageBySniff(file.Filename, head); verr != nil {
		return c.Status(fiber.StatusUnsupportedMediaType).JSON(fiber.Map{"error": "unsupported_media_type", "message": verr.Error()})
	}

	// Open file to compute hash and then persist
	src, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal_server_error", "message": "failed to open file"})
	}
	defer src.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, src); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal_server_error", "message": "failed to read file"})
	}
	fileHash := hex.EncodeToString(hash.Sum(nil))
	src.Close()
	src, err = file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal_server_error", "message": "failed to reopen file"})
	}
	defer src.Close()

	// Duplicate detection for user with optimistic lock to avoid race-condition double inserts
	imgRepo := repository.GetGlobalFactory().GetImageRepository()
	// Acquire short-lived lock on (user_id,file_hash)
	cli := cache.GetClient()
	lockKey := fmt.Sprintf("lock:upload:%d:%s", claims.UserID, fileHash)
	if cli != nil {
		ctx := context.Background()
		if ok, _ := cli.SetNX(ctx, lockKey, "1", 60*time.Second).Result(); ok {
			defer func() { _ = cli.Del(ctx, lockKey).Err() }()
		} else {
			// Another upload of the same file is in-flight; brief wait-and-see for existing image
			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) {
				if existing, err := imgRepo.GetByUserIDAndFileHash(claims.UserID, fileHash); err == nil && existing != nil {
					// Enrich response with direct URL and variants if available
					enriched := buildUploadResponseExtras(existing)
					enriched["duplicate"] = true
					return c.JSON(enriched)
				}
				time.Sleep(200 * time.Millisecond)
			}
			// Fallthrough: continue without lock (best effort), duplicate check again below
		}
	}
	// Duplicate check before create (fast path)
	if existing, err := imgRepo.GetByUserIDAndFileHash(claims.UserID, fileHash); err == nil && existing != nil {
		enriched := buildUploadResponseExtras(existing)
		enriched["duplicate"] = true
		return c.JSON(enriched)
	}

	// Storage path
	sm := storage.NewStorageManager()
	pool, err := models.FindStoragePoolByID(database.GetDB(), claims.PoolID)
	if err != nil || pool == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request", "message": "invalid pool"})
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
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal_server_error", "message": "failed to store file"})
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
		// Handle concurrent duplicate insert gracefully (no DB constraint required)
		if existing, gerr := imgRepo.GetByUserIDAndFileHash(claims.UserID, fileHash); gerr == nil && existing != nil {
			enriched := buildUploadResponseExtras(existing)
			enriched["duplicate"] = true
			return c.JSON(enriched)
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal_server_error", "message": "failed to create image record"})
	}

	// Enqueue processing
	if err := jobqueue.ProcessImageUnified(&image); err != nil {
		fiberlog.Errorf("enqueue error: %v", err)
	}

	// Build minimal variant info (original only) for immediate use
	extras := buildUploadResponseExtras(&image)
	// Mark as non-duplicate for fresh uploads
	extras["duplicate"] = false
	return c.JSON(extras)
}

// buildUploadResponseExtras constructs the enriched upload response including direct URL
// and available variant links when present. It preserves backward-compatible fields.
func buildUploadResponseExtras(img *models.Image) fiber.Map {
	// Ensure storage pool is loaded for correct absolute URL resolution
	if img != nil && img.StoragePoolID > 0 && img.StoragePool == nil {
		if pool, err := models.FindStoragePoolByID(database.GetDB(), img.StoragePoolID); err == nil && pool != nil {
			img.StoragePool = pool
		}
	}

	basePayload := fiber.Map{
		"image_uuid": img.UUID,
		"view_url":   "/i/" + img.ShareLink,
	}

	// Direct URL to original
	directOriginal := imageprocessor.GetImageAbsoluteURL(img, "original", "")
	if directOriginal != "" {
		basePayload["url"] = directOriginal
	}

	// Variants map
	variants := fiber.Map{}
	available := make([]string, 0, 3)

	// original family (always include at least original link)
	origFamily := fiber.Map{}
	if u := imageprocessor.GetImageAbsoluteURL(img, "original", ""); u != "" {
		origFamily["original"] = fiber.Map{"url": u}
	}
	if u := imageprocessor.GetImageAbsoluteURL(img, "original", "medium"); u != "" {
		origFamily["medium"] = fiber.Map{"url": u}
	}
	if u := imageprocessor.GetImageAbsoluteURL(img, "original", "small"); u != "" {
		origFamily["small"] = fiber.Map{"url": u}
	}
	if len(origFamily) > 0 {
		variants["original"] = origFamily
		available = append(available, "original")
	}

	// webp family
	webpFamily := fiber.Map{}
	if u := imageprocessor.GetImageAbsoluteURL(img, "webp", ""); u != "" {
		webpFamily["original"] = fiber.Map{"url": u}
	}
	if u := imageprocessor.GetImageAbsoluteURL(img, "webp", "medium"); u != "" {
		webpFamily["medium"] = fiber.Map{"url": u}
	}
	if u := imageprocessor.GetImageAbsoluteURL(img, "webp", "small"); u != "" {
		webpFamily["small"] = fiber.Map{"url": u}
	}
	if len(webpFamily) > 0 {
		variants["webp"] = webpFamily
		available = append(available, "webp")
	}

	// avif family
	avifFamily := fiber.Map{}
	if u := imageprocessor.GetImageAbsoluteURL(img, "avif", ""); u != "" {
		avifFamily["original"] = fiber.Map{"url": u}
	}
	if u := imageprocessor.GetImageAbsoluteURL(img, "avif", "medium"); u != "" {
		avifFamily["medium"] = fiber.Map{"url": u}
	}
	if u := imageprocessor.GetImageAbsoluteURL(img, "avif", "small"); u != "" {
		avifFamily["small"] = fiber.Map{"url": u}
	}
	if len(avifFamily) > 0 {
		variants["avif"] = avifFamily
		available = append(available, "avif")
	}

	if len(variants) > 0 {
		basePayload["variants"] = variants
	}
	if len(available) > 0 {
		basePayload["available_variants"] = available
	}

	// If not actually a duplicate (new upload path), correct the flag
	// Heuristic: when ShareLink equals computed view_url but request path sets this as new
	// The caller can override by resetting duplicate=false before returning.
	return basePayload
}

// HandleStorageReplicate accepts server-to-server replication of a single file into a target pool.
// Auth: Authorization: Bearer <REPLICATION_SECRET> or X-Replicate-Secret: <secret>
// Payload: multipart form with fields: pool_id (uint), stored_path (string: e.g. original/yyyy/mm/dd/uuid.ext), size (int64, optional), file (binary)
func HandleStorageReplicate(c *fiber.Ctx) error {
	secret := strings.TrimSpace(env.GetEnv("REPLICATION_SECRET", ""))
	if secret == "" {
		fiberlog.Warnf("[Replicate] Missing REPLICATION_SECRET; endpoint disabled")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized", "message": "replication disabled"})
	}
	// Auth check
	auth := c.Get("Authorization")
	ok := false
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		tok := strings.TrimSpace(auth[7:])
		ok = (tok == secret)
	}
	if !ok {
		if x := c.Get("X-Replicate-Secret"); strings.TrimSpace(x) == secret {
			ok = true
		}
	}
	if !ok {
		fiberlog.Warnf("[Replicate] Unauthorized attempt from %s", c.IP())
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized", "message": "invalid replication secret"})
	}

	form, err := c.MultipartForm()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request", "message": "invalid multipart form"})
	}
	defer form.RemoveAll()

	// pool_id
	var poolID uint64
	if vals, ok := form.Value["pool_id"]; ok && len(vals) > 0 {
		if pid, perr := strconv.ParseUint(strings.TrimSpace(vals[0]), 10, 64); perr == nil {
			poolID = pid
		} else {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request", "message": "invalid pool_id"})
		}
	} else {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request", "message": "missing pool_id"})
	}

	// stored_path (preferred) or relative_path + file_name
	var storedPath string
	if vals, ok := form.Value["stored_path"]; ok && len(vals) > 0 {
		storedPath = strings.TrimLeft(strings.TrimSpace(vals[0]), "/")
	} else {
		rel := ""
		if v, ok := form.Value["relative_path"]; ok && len(v) > 0 {
			rel = strings.Trim(strings.TrimSpace(v[0]), "/")
		}
		name := ""
		if v, ok := form.Value["file_name"]; ok && len(v) > 0 {
			name = strings.Trim(strings.TrimSpace(v[0]), "/")
		}
		if rel == "" || name == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request", "message": "missing stored_path"})
		}
		storedPath = path.Join(rel, name)
	}

	// Sanitize storedPath: must be a relative path within allowed prefixes
	cleanStored := path.Clean("/" + storedPath) // ensure leading slash for clean, then strip
	cleanStored = strings.TrimPrefix(cleanStored, "/")
	if strings.HasPrefix(cleanStored, "../") || strings.Contains(cleanStored, "/../") || strings.HasPrefix(cleanStored, "..") {
		fiberlog.Warnf("[Replicate] Rejected traversal path from %s: %s", c.IP(), storedPath)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request", "message": "invalid stored_path"})
	}
	// Optional: restrict to known roots
	if !(strings.HasPrefix(cleanStored, "original/") || strings.HasPrefix(cleanStored, "variants/")) {
		fiberlog.Warnf("[Replicate] Rejected invalid root from %s: %s", c.IP(), cleanStored)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request", "message": "invalid path root"})
	}
	storedPath = cleanStored

	// expected size (optional)
	var expectedSize int64 = -1
	if vals, ok := form.Value["size"]; ok && len(vals) > 0 {
		if s, perr := strconv.ParseInt(strings.TrimSpace(vals[0]), 10, 64); perr == nil {
			expectedSize = s
		}
	}

	files := form.File["file"]
	if len(files) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request", "message": "file missing"})
	}
	fh := files[0]
	// optional checksum
	var wantSum string
	if vals, ok := form.Value["sha256"]; ok && len(vals) > 0 {
		wantSum = strings.TrimSpace(vals[0])
		if wantSum != "" && len(wantSum) != 64 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request", "message": "invalid sha256"})
		}
	}

	// Capacity/health precheck if size known
	sm := storage.NewStorageManager()
	if expectedSize >= 0 {
		if pool, err := models.FindStoragePoolByID(database.GetDB(), uint(poolID)); err == nil && pool != nil {
			if !pool.IsHealthy() {
				fiberlog.Warnf("[Replicate] Target pool unhealthy (pool_id=%d, path=%s) from %s", poolID, storedPath, c.IP())
				return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "service_unavailable", "message": "target pool unhealthy"})
			}
			if !pool.CanAcceptFile(expectedSize) {
				fiberlog.Warnf("[Replicate] Insufficient capacity (pool_id=%d, path=%s, size=%d) from %s", poolID, storedPath, expectedSize, c.IP())
				return c.Status(fiber.StatusInsufficientStorage).JSON(fiber.Map{"error": "insufficient_storage", "message": "insufficient capacity"})
			}
		}
	}

	// Idempotency: if file already exists at destination and size matches, skip
	if exists, existingSize, existsErr := sm.FileExists(storedPath, uint(poolID)); existsErr == nil && exists {
		// Compare size if provided else with uploaded header size
		want := expectedSize
		if want < 0 {
			want = fh.Size
		}
		if want >= 0 && existingSize == want {
			fiberlog.Infof("[Replicate] Skip existing file (pool_id=%d, path=%s, size=%d) from %s", poolID, storedPath, want, c.IP())
			return c.JSON(fiber.Map{"status": "ok", "skipped": true, "reason": "exists"})
		}
	}

	// Open uploaded file and store
	src, err := fh.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal_server_error", "message": "failed to open file"})
	}
	defer src.Close()

	// Compute checksum while streaming to storage; delete on mismatch
	hasher := sha256.New()
	tee := io.TeeReader(src, hasher)
	if _, err := sm.SaveFile(tee, storedPath, uint(poolID)); err != nil {
		fiberlog.Errorf("Replicate SaveFile error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal_server_error", "message": "failed to store file"})
	}

	// Enforce checksum by admin setting
	requireChecksum := models.GetAppSettings().IsReplicationChecksumRequired()
	if requireChecksum && wantSum == "" {
		fiberlog.Warnf("[Replicate] Missing required checksum (pool_id=%d, path=%s) from %s", poolID, storedPath, c.IP())
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request", "message": "checksum required"})
	}

	if wantSum != "" {
		got := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(got, wantSum) {
			// Remove corrupted file and report error
			_, _ = sm.DeleteFile(storedPath, uint(poolID))
			fiberlog.Warnf("[Replicate] Checksum mismatch (pool_id=%d, path=%s) from %s", poolID, storedPath, c.IP())
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": "unprocessable_entity", "message": "checksum mismatch"})
		}
	}

	fiberlog.Infof("[Replicate] Stored file (pool_id=%d, path=%s, size=%d) from %s", poolID, storedPath, fh.Size, c.IP())
	return c.JSON(fiber.Map{"status": "ok"})
}
