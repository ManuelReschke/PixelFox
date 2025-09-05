package controllers

import (
	"strconv"

	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/sujit-baniya/flash"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/app/repository"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/jobqueue"
	"github.com/ManuelReschke/PixelFox/internal/pkg/usercontext"
	"github.com/ManuelReschke/PixelFox/views"
	"github.com/ManuelReschke/PixelFox/views/admin_views"
)

// AdminImagesController handles admin image management using Repository Pattern
type AdminImagesController struct {
	imageRepo repository.ImageRepository
}

// NewAdminImagesController creates a new admin images controller with repository dependency
func NewAdminImagesController(imageRepo repository.ImageRepository) *AdminImagesController {
	return &AdminImagesController{
		imageRepo: imageRepo,
	}
}

// Global admin images controller instance
var adminImagesController *AdminImagesController

// InitializeAdminImagesController initializes the global admin images controller with repositories
func InitializeAdminImagesController() {
	repos := repository.GetGlobalRepositories()
	adminImagesController = NewAdminImagesController(repos.Image)
}

// GetAdminImagesController returns the global admin images controller instance
func GetAdminImagesController() *AdminImagesController {
	if adminImagesController == nil {
		InitializeAdminImagesController()
	}
	return adminImagesController
}

// HandleAdminImages renders the image management page with repository pattern
func (aic *AdminImagesController) HandleAdminImages(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage := 50
	offset := (page - 1) * perPage

	// Get total image count
	totalImages, err := aic.imageRepo.Count()
	if err != nil {
		return aic.handleError(c, "Failed to get image count", err)
	}

	// Get images using repository
	images, err := aic.imageRepo.List(offset, perPage)
	if err != nil {
		return aic.handleError(c, "Failed to get images", err)
	}

	// Calculate pagination
	totalPages := int(totalImages) / perPage
	if int(totalImages)%perPage > 0 {
		totalPages++
	}

	// CSRF token for POST actions; route is not under CSRF middleware, so token may be absent
	var csrfToken string
	if v := c.Locals("csrf"); v != nil {
		if t, ok := v.(string); ok {
			csrfToken = t
		}
	}

	// Render image management page
	imageManagement := admin_views.ImageManagement(images, page, totalPages, csrfToken)
	home := views.HomeCtx(c, " | Image Management", userCtx.IsLoggedIn, false, flash.Get(c), imageManagement, userCtx.IsAdmin, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleAdminImageSearch searches for images using repository
func (aic *AdminImagesController) HandleAdminImageSearch(c *fiber.Ctx, query string) error {
	userCtx := usercontext.GetUserContext(c)
	// Search images using repository
	images, err := aic.imageRepo.Search(query)
	if err != nil {
		return aic.handleError(c, "Image search failed", err)
	}

	// Set search result message
	fm := fiber.Map{
		"type":    "info",
		"message": "Search results for '" + query + "': " + strconv.Itoa(len(images)) + " images found",
	}
	flash.WithInfo(c, fm)

	// CSRF token may be absent on this route; guard the lookup
	var csrfToken string
	if v := c.Locals("csrf"); v != nil {
		if t, ok := v.(string); ok {
			csrfToken = t
		}
	}

	// Render results
	imageManagement := admin_views.ImageManagement(images, 1, 1, csrfToken)
	home := views.HomeCtx(c, " | Image Search", userCtx.IsLoggedIn, false, flash.Get(c), imageManagement, userCtx.IsAdmin, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleAdminImageEdit renders the image edit page using repository pattern
func (aic *AdminImagesController) HandleAdminImageEdit(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	imageUUID := c.Params("uuid")
	if imageUUID == "" {
		return c.Redirect("/admin/images")
	}

	// Get image using repository
	image, err := aic.imageRepo.GetByUUID(imageUUID)
	if err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "Bild nicht gefunden",
		}
		return flash.WithError(c, fm).Redirect("/admin/images")
	}

	// Render image edit page
	imageEdit := admin_views.ImageEdit(*image)
	home := views.HomeCtx(c, " | Bild bearbeiten", userCtx.IsLoggedIn, false, flash.Get(c), imageEdit, userCtx.IsAdmin, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleAdminImageUpdate handles image update using repository pattern
func (aic *AdminImagesController) HandleAdminImageUpdate(c *fiber.Ctx) error {
	imageUUID := c.Params("uuid")
	if imageUUID == "" {
		return c.Redirect("/admin/images")
	}

	// Get image using repository
	image, err := aic.imageRepo.GetByUUID(imageUUID)
	if err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "Bild nicht gefunden",
		}
		return flash.WithError(c, fm).Redirect("/admin/images")
	}

	// Get form data
	title := c.FormValue("title")
	description := c.FormValue("description")
	isPublic := c.FormValue("is_public") == "on"

	// Update image
	image.Title = title
	image.Description = description
	image.IsPublic = isPublic

	// Save using repository
	if err := aic.imageRepo.Update(image); err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "Fehler beim Aktualisieren des Bildes: " + err.Error(),
		}
		return flash.WithError(c, fm).Redirect("/admin/images/edit/" + imageUUID)
	}

	// Success message
	fm := fiber.Map{
		"type":    "success",
		"message": "Bild erfolgreich aktualisiert",
	}

	return flash.WithSuccess(c, fm).Redirect("/admin/images")
}

// HandleAdminImageDelete handles image deletion using repository pattern
func (aic *AdminImagesController) HandleAdminImageDelete(c *fiber.Ctx) error {
	imageUUID := c.Params("uuid")
	if imageUUID == "" {
		return c.Redirect("/admin/images")
	}

	// Get image using repository
	image, err := aic.imageRepo.GetByUUID(imageUUID)
	if err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "Bild nicht gefunden",
		}
		return flash.WithError(c, fm).Redirect("/admin/images")
	}

	// Delete image using repository (this should handle file cleanup and variants)
	if err := aic.imageRepo.Delete(image.ID); err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "Fehler beim Löschen des Bildes: " + err.Error(),
		}
		return flash.WithError(c, fm).Redirect("/admin/images")
	}

	// Success message
	fm := fiber.Map{
		"type":    "success",
		"message": "Bild erfolgreich gelöscht",
	}

	return flash.WithSuccess(c, fm).Redirect("/admin/images")
}

// HandleAdminImageStartBackup enqueues a manual S3 backup job for an image if none exists
func (aic *AdminImagesController) HandleAdminImageStartBackup(c *fiber.Ctx) error {
	imageUUID := c.Params("uuid")
	if imageUUID == "" {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	// Find image
	image, err := aic.imageRepo.GetByUUID(imageUUID)
	if err != nil || image == nil {
		return c.SendStatus(fiber.StatusNotFound)
	}

	db := database.GetDB()
	if db == nil {
		return c.Status(fiber.StatusInternalServerError).SendString("DB not available")
	}

	// If backup already exists, do nothing (idempotent)
	if backups, err := models.FindCompletedBackupsByImageID(db, image.ID); err == nil && len(backups) > 0 {
		return c.SendStatus(fiber.StatusNoContent)
	}

	// Check admin toggle and s3 pool
	settings := models.GetAppSettings()
	if settings == nil || !settings.IsS3BackupEnabled() {
		return c.Status(fiber.StatusBadRequest).SendString("S3 backup disabled")
	}
	s3Pool, err := models.FindHighestPriorityS3Pool(db)
	if err != nil || s3Pool == nil {
		return c.Status(fiber.StatusBadRequest).SendString("No active S3 storage pool")
	}

	// Create backup record
	backup, err := models.CreateBackupRecord(db, image.ID, models.BackupProviderS3)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to create backup record")
	}

	// Enqueue S3 backup job
	queue := jobqueue.GetManager().GetQueue()
	if _, err := queue.EnqueueS3BackupJob(image.ID, image.UUID, image.FilePath, image.FileName, image.FileSize, backup.ID); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to enqueue backup job")
	}

	return c.SendStatus(fiber.StatusAccepted)
}

// HandleAdminImageDeleteBackup enqueues S3 delete jobs for completed backups of an image
func (aic *AdminImagesController) HandleAdminImageDeleteBackup(c *fiber.Ctx) error {
	imageUUID := c.Params("uuid")
	if imageUUID == "" {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	// Find image
	image, err := aic.imageRepo.GetByUUID(imageUUID)
	if err != nil || image == nil {
		return c.SendStatus(fiber.StatusNotFound)
	}

	db := database.GetDB()
	if db == nil {
		return c.Status(fiber.StatusInternalServerError).SendString("DB not available")
	}

	// Find all completed backups for this image
	backups, err := models.FindCompletedBackupsByImageID(db, image.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to fetch backups")
	}
	if len(backups) == 0 {
		return c.SendStatus(fiber.StatusNoContent)
	}

	// Enqueue delete jobs
	queue := jobqueue.GetManager().GetQueue()
	for _, backup := range backups {
		if _, err := queue.EnqueueS3DeleteJob(
			image.ID,
			image.UUID,
			backup.ObjectKey,
			backup.BucketName,
			backup.ID,
		); err != nil {
			// continue with others; report generic failure if needed
			// but do not block the entire action
			continue
		}
	}

	return c.SendStatus(fiber.StatusAccepted)
}

// handleError handles errors consistently
func (aic *AdminImagesController) handleError(c *fiber.Ctx, message string, err error) error {
	fm := fiber.Map{
		"type":    "error",
		"message": message,
	}

	return flash.WithError(c, fm).Redirect("/admin/images")
}
