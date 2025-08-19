package controllers

import (
	"strconv"

	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/sujit-baniya/flash"

	"github.com/ManuelReschke/PixelFox/app/repository"
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
	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage := 20
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

	// Render image management page
	imageManagement := admin_views.ImageManagement(images, page, totalPages)
	home := views.Home(" | Image Management", isLoggedIn(c), false, flash.Get(c), imageManagement, true, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleAdminImageSearch searches for images using repository
func (aic *AdminImagesController) HandleAdminImageSearch(c *fiber.Ctx, query string) error {
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

	// Render results
	imageManagement := admin_views.ImageManagement(images, 1, 1)
	home := views.Home(" | Image Search", isLoggedIn(c), false, flash.Get(c), imageManagement, true, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleAdminImageEdit renders the image edit page using repository pattern
func (aic *AdminImagesController) HandleAdminImageEdit(c *fiber.Ctx) error {
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
	home := views.Home(" | Bild bearbeiten", isLoggedIn(c), false, flash.Get(c), imageEdit, true, nil)

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

// handleError handles errors consistently
func (aic *AdminImagesController) handleError(c *fiber.Ctx, message string, err error) error {
	fm := fiber.Map{
		"type":    "error",
		"message": message,
	}

	return flash.WithError(c, fm).Redirect("/admin/images")
}
