package controllers

import (
	"strconv"
	"time"

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
	if c.Method() != fiber.MethodPost {
		return c.SendStatus(fiber.StatusMethodNotAllowed)
	}

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

	// Enqueue async delete job (files + deep cleanup)
	queue := jobqueue.GetManager().GetQueue()
	var ridPtr *uint
	if reportIDStr := c.Query("resolved_report_id", ""); reportIDStr != "" {
		if rid64, err := strconv.ParseUint(reportIDStr, 10, 64); err == nil {
			r := uint(rid64)
			ridPtr = &r
		}
	}
	uctx := usercontext.GetUserContext(c)
	var initiatedBy *uint
	if uctx.UserID > 0 {
		uid := uctx.UserID
		initiatedBy = &uid
	}
	if _, err := queue.EnqueueDeleteImageJob(image.ID, image.UUID, ridPtr, initiatedBy); err != nil {
		// If enqueue fails, fall back to immediate soft delete path
		// proceed but log via flash
		fm := fiber.Map{
			"type":    "error",
			"message": "Konnte Löschauftrag nicht einreihen, versuche direkt zu löschen",
		}
		flash.WithError(c, fm)
	}

	// Immediate soft-delete in DB to hide image from UI
	if err := aic.imageRepo.Delete(image.ID); err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "Fehler beim Löschen des Bildes: " + err.Error(),
		}
		return flash.WithError(c, fm).Redirect("/admin/images")
	}

	// If deletion originated from a report, mark that report as resolved
	if reportIDStr := c.Query("resolved_report_id", ""); reportIDStr != "" {
		if rid, err := strconv.ParseUint(reportIDStr, 10, 64); err == nil {
			db := database.GetDB()
			userCtx := usercontext.GetUserContext(c)
			resolvedBy := userCtx.UserID
			now := time.Now()
			_ = db.Model(&models.ImageReport{}).
				Where("id = ?", uint(rid)).
				Updates(map[string]interface{}{
					"status":         models.ReportStatusResolved,
					"resolved_by_id": resolvedBy,
					"resolved_at":    now,
				}).Error
		}
	}

	// Success message
	fm := fiber.Map{
		"type":    "success",
		"message": "Bild erfolgreich gelöscht",
	}

	// Redirect: if deletion came from a report, go back to reports; else images list
	if c.Query("resolved_report_id", "") != "" {
		return flash.WithSuccess(c, fm).Redirect("/admin/reports")
	}

	return flash.WithSuccess(c, fm).Redirect("/admin/images")
}

// handleError handles errors consistently
func (aic *AdminImagesController) handleError(c *fiber.Ctx, message string, err error) error {
	fm := fiber.Map{
		"type":    "error",
		"message": message,
	}

	return flash.WithError(c, fm).Redirect("/admin/images")
}
