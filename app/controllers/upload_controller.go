package controllers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	fiberlog "github.com/gofiber/fiber/v2/log"
	"github.com/google/uuid"
	"github.com/sujit-baniya/flash"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/app/repository"
	"github.com/ManuelReschke/PixelFox/internal/pkg/cache"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/entitlements"
	"github.com/ManuelReschke/PixelFox/internal/pkg/jobqueue"
	"github.com/ManuelReschke/PixelFox/internal/pkg/statistics"
	"github.com/ManuelReschke/PixelFox/internal/pkg/storage"
	"github.com/ManuelReschke/PixelFox/internal/pkg/upload"
	"github.com/ManuelReschke/PixelFox/internal/pkg/usercontext"
)

type uploadWorkflow struct {
	c              *fiber.Ctx
	userCtx        usercontext.UserContext
	imageRepo      repository.ImageRepository
	storageManager *storage.StorageManager
}

type persistedUpload struct {
	image        *models.Image
	selectedPool *models.StoragePool
}

var errUploadResponseHandled = errors.New("upload response already handled")

func HandleUpload(c *fiber.Ctx) error {
	return newUploadWorkflow(c).run()
}

func newUploadWorkflow(c *fiber.Ctx) *uploadWorkflow {
	return &uploadWorkflow{
		c:              c,
		userCtx:        usercontext.GetUserContext(c),
		imageRepo:      repository.GetGlobalFactory().GetImageRepository(),
		storageManager: storage.NewStorageManager(),
	}
}

func (w *uploadWorkflow) run() error {
	if !w.userCtx.IsLoggedIn {
		return w.c.Status(fiber.StatusUnauthorized).SendString("Unauthorized")
	}

	if !models.GetAppSettings().IsImageUploadEnabled() {
		return respondUploadError(w.c, fiber.StatusForbidden, "Der Bild-Upload ist derzeit deaktiviert", "/")
	}

	form, file, err := w.parseUploadForm()
	if err != nil {
		if errors.Is(err, errUploadResponseHandled) {
			return nil
		}
		return err
	}
	defer form.RemoveAll()

	if err := w.validateEntitlements(file); err != nil {
		if errors.Is(err, errUploadResponseHandled) {
			return nil
		}
		return err
	}

	fileExt, src, fileHash, err := w.prepareSource(file)
	if err != nil {
		if errors.Is(err, errUploadResponseHandled) {
			return nil
		}
		return err
	}
	defer src.Close()

	duplicate, unlock := w.detectDuplicate(w.userCtx.UserID, fileHash)
	defer unlock()
	if duplicate != nil {
		fiberlog.Infof("[Upload] Duplicate file detected for user %d, redirecting to existing image %s", w.userCtx.UserID, duplicate.UUID)
		return respondDuplicateUpload(w.c, duplicate)
	}

	persisted, err := w.persistUpload(file, src, fileExt, fileHash)
	if err != nil {
		if errors.Is(err, errUploadResponseHandled) {
			return nil
		}
		return err
	}

	w.afterPersist(persisted)
	return w.respondSuccess(file.Filename, persisted.image.UUID)
}

func (w *uploadWorkflow) parseUploadForm() (*multipart.Form, *multipart.FileHeader, error) {
	form, err := w.c.MultipartForm()
	if err != nil {
		fiberlog.Errorf("Error parsing multipart form: %v", err)
		return nil, nil, markHandledResponse(respondUploadError(w.c, fiber.StatusBadRequest, fmt.Sprintf("Fehler beim Hochladen: %s", err), "/"))
	}

	files := form.File["file"]
	if len(files) == 0 {
		return nil, nil, markHandledResponse(respondUploadError(w.c, fiber.StatusBadRequest, "Keine Datei hochgeladen", "/"))
	}

	return form, files[0], nil
}

func (w *uploadWorkflow) validateEntitlements(file *multipart.FileHeader) error {
	maxBytes := entitlements.MaxUploadBytes(entitlements.Plan(w.userCtx.Plan))
	if file.Size > maxBytes {
		msg := fmt.Sprintf("Die Datei ist zu groß für dein Paket. Maximal erlaubt: %s.", formatBytes(maxBytes))
		return markHandledResponse(respondUploadError(w.c, fiber.StatusRequestEntityTooLarge, msg, "/flash/upload-too-large"))
	}

	quota := entitlements.StorageQuotaBytes(entitlements.Plan(w.userCtx.Plan))
	if quota <= 0 {
		return nil
	}

	var used int64
	db := database.GetDB()
	if db != nil {
		db.Model(&models.Image{}).Where("user_id = ?", w.userCtx.UserID).Select("COALESCE(SUM(file_size), 0)").Row().Scan(&used)
	}
	if used+file.Size > quota {
		remaining := quota - used
		if remaining < 0 {
			remaining = 0
		}
		msg := fmt.Sprintf("Speicherlimit erreicht. Frei: %s, benötigt: %s.", formatBytes(remaining), formatBytes(file.Size))
		return markHandledResponse(respondUploadError(w.c, fiber.StatusRequestEntityTooLarge, msg, "/flash/upload-too-large"))
	}

	return nil
}

func (w *uploadWorkflow) prepareSource(file *multipart.FileHeader) (string, multipart.File, string, error) {
	fileExt := strings.ToLower(filepath.Ext(file.Filename))

	pre, err := file.Open()
	if err != nil {
		fiberlog.Errorf("Error opening uploaded file for sniff: %v", err)
		return "", nil, "", markHandledResponse(w.c.Status(fiber.StatusInternalServerError).SendString("Fehler beim Verarbeiten der Datei"))
	}
	head := make([]byte, 512)
	n, _ := io.ReadFull(pre, head)
	if n > 0 {
		head = head[:n]
	}
	_ = pre.Close()
	if _, err := upload.ValidateImageBySniff(file.Filename, head); err != nil {
		return "", nil, "", markHandledResponse(respondUploadError(w.c, fiber.StatusUnsupportedMediaType, err.Error(), "/"))
	}

	hashSrc, err := file.Open()
	if err != nil {
		fiberlog.Errorf("Error opening uploaded file for hash: %v", err)
		return "", nil, "", markHandledResponse(respondUploadError(w.c, fiber.StatusInternalServerError, fmt.Sprintf("Fehler beim Öffnen der Datei: %s", err), "/"))
	}
	fileHash, err := calculateFileHash(hashSrc)
	_ = hashSrc.Close()
	if err != nil {
		fiberlog.Errorf("Error calculating file hash: %v", err)
		return "", nil, "", markHandledResponse(respondUploadError(w.c, fiber.StatusInternalServerError, "Fehler beim Verarbeiten der Datei", "/"))
	}

	src, err := file.Open()
	if err != nil {
		fiberlog.Errorf("Error reopening file: %v", err)
		return "", nil, "", markHandledResponse(w.c.Status(fiber.StatusInternalServerError).SendString("Fehler beim Verarbeiten der Datei"))
	}

	return fileExt, src, fileHash, nil
}

func (w *uploadWorkflow) detectDuplicate(userID uint, fileHash string) (*models.Image, func()) {
	unlock := func() {}
	var existingImage *models.Image

	if cli := cache.GetClient(); cli != nil {
		ctx := context.Background()
		lockKey := fmt.Sprintf("lock:upload:%d:%s", userID, fileHash)
		if ok, _ := cli.SetNX(ctx, lockKey, "1", 60*time.Second).Result(); ok {
			unlock = func() { _ = cli.Del(ctx, lockKey).Err() }
		} else {
			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) {
				if ex, err := w.imageRepo.GetByUserIDAndFileHash(userID, fileHash); err == nil && ex != nil {
					existingImage = ex
					break
				}
				time.Sleep(200 * time.Millisecond)
			}
		}
	}

	if existingImage == nil {
		if ex, err := w.imageRepo.GetByUserIDAndFileHash(userID, fileHash); err == nil && ex != nil {
			existingImage = ex
		}
	}

	return existingImage, unlock
}

func (w *uploadWorkflow) persistUpload(file *multipart.FileHeader, src multipart.File, fileExt, fileHash string) (*persistedUpload, error) {
	selectedPool, err := w.storageManager.SelectPoolForUpload(file.Size)
	if err != nil {
		fiberlog.Errorf("Error selecting storage pool: %v", err)
		return nil, markHandledResponse(respondUploadError(w.c, fiber.StatusInternalServerError, "Fehler bei der Speicherplatz-Auswahl", "/"))
	}

	imageUUID := uuid.New().String()
	relativePath := fmt.Sprintf("%d/%02d/%02d", time.Now().Year(), time.Now().Month(), time.Now().Day())
	fileName := fmt.Sprintf("%s%s", imageUUID, fileExt)

	fiberlog.Infof("[Upload] Selected %s storage pool '%s' for upload", selectedPool.StorageTier, selectedPool.Name)
	savePath := filepath.Join("original", relativePath, fileName)
	op, err := w.storageManager.SaveFile(src, savePath, selectedPool.ID)
	if err != nil || op == nil || !op.Success {
		if err == nil && op != nil {
			err = op.Error
		}
		fiberlog.Errorf("Error saving file to storage pool: %v", err)
		return nil, markHandledResponse(respondUploadError(w.c, fiber.StatusInternalServerError, fmt.Sprintf("Fehler beim Speichern der Datei: %s", err), "/"))
	}

	ipv4, ipv6 := GetClientIP(w.c)
	image := &models.Image{
		UUID:          imageUUID,
		UserID:        w.userCtx.UserID,
		StoragePoolID: selectedPool.ID,
		FileName:      fileName,
		FilePath:      filepath.Join("original", relativePath),
		FileSize:      file.Size,
		FileType:      fileExt,
		Title:         file.Filename,
		FileHash:      fileHash,
		IPv4:          ipv4,
		IPv6:          ipv6,
	}

	if err := w.imageRepo.Create(image); err != nil {
		fiberlog.Errorf("Error saving image to database: %v", err)
		if _, delErr := w.storageManager.DeleteFile(savePath, selectedPool.ID); delErr != nil {
			fiberlog.Warnf("Failed to cleanup stored file after DB error: %v", delErr)
		}
		return nil, w.handlePersistError(err, file.Filename, fileHash)
	}

	return &persistedUpload{
		image:        image,
		selectedPool: selectedPool,
	}, nil
}

func (w *uploadWorkflow) handlePersistError(createErr error, uploadFileName, fileHash string) error {
	if strings.Contains(strings.ToLower(createErr.Error()), "duplicate") {
		if existing, err := w.imageRepo.GetByUserIDAndFileHash(w.userCtx.UserID, fileHash); err == nil && existing != nil {
			fm := fiber.Map{
				"type":           "info",
				"message":        "Du hast dieses Bild bereits hochgeladen!",
				"existing_image": existing.UUID,
				"existing_title": existing.Title,
			}
			flash.WithInfo(w.c, fm)
			return markHandledResponse(w.c.Redirect("/image/" + existing.UUID))
		}
	}

	flash.WithError(w.c, fiber.Map{
		"type":    "error",
		"message": fmt.Sprintf("Datei konnte nicht gespeichert werden: %s", uploadFileName),
	})
	if isHTMXRequest(w.c) {
		return markHandledResponse(w.c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Fehler beim Speichern: %s", createErr)))
	}
	return markHandledResponse(w.c.Redirect("/"))
}

func (w *uploadWorkflow) afterPersist(persisted *persistedUpload) {
	fiberlog.Infof("[Upload] Enqueueing unified image processing for %s", persisted.image.UUID)
	if err := jobqueue.ProcessImageUnified(persisted.image); err != nil {
		fiberlog.Errorf("Error enqueueing unified image processing for %s: %v", persisted.image.UUID, err)
	}

	go statistics.UpdateStatisticsCache()
}

func (w *uploadWorkflow) respondSuccess(fileName, imageUUID string) error {
	if isHTMXRequest(w.c) {
		flash.WithSuccess(w.c, fiber.Map{
			"type":    "success",
			"message": fmt.Sprintf("Datei erfolgreich hochgeladen: %s", fileName),
		})

		redirectPath := fmt.Sprintf("/image/%s", imageUUID)
		w.c.Set("HX-Redirect", redirectPath)
		return w.c.SendString(fmt.Sprintf("Datei erfolgreich hochgeladen: %s", fileName))
	}

	return w.c.Redirect(fmt.Sprintf("/image/%s", imageUUID))
}

func isHTMXRequest(c *fiber.Ctx) bool {
	return c.Get("HX-Request") == "true"
}

func respondUploadError(c *fiber.Ctx, status int, message, redirectPath string) error {
	flash.WithError(c, fiber.Map{
		"type":    "error",
		"message": message,
	})
	if isHTMXRequest(c) {
		return c.Status(status).SendString(message)
	}
	return c.Redirect(redirectPath)
}

func respondDuplicateUpload(c *fiber.Ctx, existingImage *models.Image) error {
	if isHTMXRequest(c) {
		duplicateTitle := existingImage.Title
		if duplicateTitle == "" {
			duplicateTitle = existingImage.FileName
		}

		htmlResponse := fmt.Sprintf(`
			<div class="alert alert-info shadow-lg mb-4">
				<div>
					<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="stroke-current flex-shrink-0 w-6 h-6">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
					</svg>
					<div>
						<h3 class="font-bold">Bild bereits vorhanden!</h3>
						<div class="text-xs">Du hast dieses Bild bereits hochgeladen: "%s"</div>
					</div>
				</div>
				<div class="flex-none">
					<a href="/image/%s" class="btn btn-sm btn-outline">
						📷 Bild ansehen
					</a>
				</div>
			</div>
		`, duplicateTitle, existingImage.UUID)

		return c.Status(fiber.StatusOK).Type("text/html").SendString(htmlResponse)
	}

	flash.WithInfo(c, fiber.Map{
		"type":           "info",
		"message":        "Du hast dieses Bild bereits hochgeladen!",
		"existing_image": existingImage.UUID,
		"existing_title": existingImage.Title,
	})
	return c.Redirect("/image/" + existingImage.UUID)
}

func calculateFileHash(file io.Reader) (string, error) {
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func markHandledResponse(err error) error {
	if err != nil {
		return err
	}
	return errUploadResponseHandled
}
