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
	"github.com/ManuelReschke/PixelFox/internal/pkg/usercontext"
	"github.com/ManuelReschke/PixelFox/views"
	"github.com/ManuelReschke/PixelFox/views/admin_views"
)

// ============================================================================
// ADMIN PAGE CONTROLLER - Repository Pattern
// ============================================================================

// AdminPageController handles admin page-related HTTP requests using repository pattern
type AdminPageController struct {
	pageRepo repository.PageRepository
}

// NewAdminPageController creates a new admin page controller with repository
func NewAdminPageController(pageRepo repository.PageRepository) *AdminPageController {
	return &AdminPageController{
		pageRepo: pageRepo,
	}
}

// handleError is a helper method for consistent error handling
func (apc *AdminPageController) handleError(c *fiber.Ctx, message string, err error) error {
	fm := fiber.Map{
		"type":    "error",
		"message": message + ": " + err.Error(),
	}
	return flash.WithError(c, fm).Redirect("/admin/pages")
}

// HandleAdminPages renders the page management overview using repository pattern
func (apc *AdminPageController) HandleAdminPages(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	// Get all pages using repository
	pages, err := apc.pageRepo.GetAll()
	if err != nil {
		return apc.handleError(c, "Fehler beim Laden der Seiten", err)
	}
	csrfToken := c.Locals("csrf").(string)

	// Render page management
	pageManagement := admin_views.PageManagement(pages, csrfToken)
	home := views.HomeCtx(c, " | Seiten-Verwaltung", userCtx.IsLoggedIn, false, flash.Get(c), pageManagement, userCtx.IsAdmin, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleAdminPageCreate renders the page creation form using repository pattern
func (apc *AdminPageController) HandleAdminPageCreate(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	// Get CSRF token
	csrfToken := c.Locals("csrf").(string)

	// Create empty page for the form
	emptyPage := models.Page{}

	// Render page creation form (reuse PageEdit with isEdit=false)
	pageCreate := admin_views.PageEdit(emptyPage, false, csrfToken)
	home := views.HomeCtx(c, " | Neue Seite erstellen", userCtx.IsLoggedIn, false, flash.Get(c), pageCreate, userCtx.IsAdmin, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleAdminPageStore handles page creation using repository pattern
func (apc *AdminPageController) HandleAdminPageStore(c *fiber.Ctx) error {
	// Get form values
	title := c.FormValue("title")
	slug := c.FormValue("slug")
	content := c.FormValue("content")
	isActive := c.FormValue("is_active") == "on"

	// Validate required fields
	if title == "" || slug == "" {
		fm := fiber.Map{
			"type":    "error",
			"message": "Titel und Slug sind erforderlich",
		}
		return flash.WithError(c, fm).Redirect("/admin/pages/create")
	}

	// Check if slug already exists using repository
	slugExists, err := apc.pageRepo.SlugExists(slug)
	if err != nil {
		return apc.handleError(c, "Fehler beim Prüfen des Slugs", err)
	}

	if slugExists {
		fm := fiber.Map{
			"type":    "error",
			"message": "Eine Seite mit diesem Slug existiert bereits",
		}
		return flash.WithError(c, fm).Redirect("/admin/pages/create")
	}

	// Create new page
	page := &models.Page{
		Title:     title,
		Slug:      slug,
		Content:   content,
		IsActive:  isActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Save using repository
	if err := apc.pageRepo.Create(page); err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "Fehler beim Erstellen der Seite: " + err.Error(),
		}
		return flash.WithError(c, fm).Redirect("/admin/pages/create")
	}

	// Success message
	fm := fiber.Map{
		"type":    "success",
		"message": "Seite erfolgreich erstellt",
	}

	return flash.WithSuccess(c, fm).Redirect("/admin/pages")
}

// HandleAdminPageEdit renders the page edit form using repository pattern
func (apc *AdminPageController) HandleAdminPageEdit(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	pageID := c.Params("id")
	if pageID == "" {
		return c.Redirect("/admin/pages")
	}

	id, err := strconv.ParseUint(pageID, 10, 32)
	if err != nil {
		return c.Redirect("/admin/pages")
	}

	// Get page using repository
	page, err := apc.pageRepo.GetByID(uint(id))
	if err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "Seite nicht gefunden",
		}
		return flash.WithError(c, fm).Redirect("/admin/pages")
	}

	// Get CSRF token
	csrfToken := c.Locals("csrf").(string)

	// Render page edit form
	pageEdit := admin_views.PageEdit(*page, true, csrfToken)
	home := views.HomeCtx(c, " | Seite bearbeiten", userCtx.IsLoggedIn, false, flash.Get(c), pageEdit, userCtx.IsAdmin, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleAdminPageUpdate handles page updates using repository pattern
func (apc *AdminPageController) HandleAdminPageUpdate(c *fiber.Ctx) error {
	pageID := c.Params("id")
	if pageID == "" {
		return c.Redirect("/admin/pages")
	}

	id, err := strconv.ParseUint(pageID, 10, 32)
	if err != nil {
		return c.Redirect("/admin/pages")
	}

	// Get page using repository
	page, err := apc.pageRepo.GetByID(uint(id))
	if err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "Seite nicht gefunden",
		}
		return flash.WithError(c, fm).Redirect("/admin/pages")
	}

	// Get form values
	title := c.FormValue("title")
	slug := c.FormValue("slug")
	content := c.FormValue("content")
	isActive := c.FormValue("is_active") == "on"

	// Validate required fields
	if title == "" || slug == "" {
		fm := fiber.Map{
			"type":    "error",
			"message": "Titel und Slug sind erforderlich",
		}
		return flash.WithError(c, fm).Redirect("/admin/pages/edit/" + pageID)
	}

	// Check if slug already exists (excluding current page) using repository
	if slug != page.Slug {
		slugExists, err := apc.pageRepo.SlugExistsExceptID(slug, uint(id))
		if err != nil {
			return apc.handleError(c, "Fehler beim Prüfen des Slugs", err)
		}

		if slugExists {
			fm := fiber.Map{
				"type":    "error",
				"message": "Eine andere Seite mit diesem Slug existiert bereits",
			}
			return flash.WithError(c, fm).Redirect("/admin/pages/edit/" + pageID)
		}
	}

	// Update page
	page.Title = title
	page.Slug = slug
	page.Content = content
	page.IsActive = isActive
	page.UpdatedAt = time.Now()

	// Save using repository
	if err := apc.pageRepo.Update(page); err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "Fehler beim Aktualisieren der Seite: " + err.Error(),
		}
		return flash.WithError(c, fm).Redirect("/admin/pages/edit/" + pageID)
	}

	// Success message
	fm := fiber.Map{
		"type":    "success",
		"message": "Seite erfolgreich aktualisiert",
	}

	return flash.WithSuccess(c, fm).Redirect("/admin/pages")
}

// HandleAdminPageDelete handles page deletion using repository pattern
func (apc *AdminPageController) HandleAdminPageDelete(c *fiber.Ctx) error {
	if c.Method() != fiber.MethodPost {
		return c.SendStatus(fiber.StatusMethodNotAllowed)
	}

	pageID := c.Params("id")
	if pageID == "" {
		return c.Redirect("/admin/pages")
	}

	id, err := strconv.ParseUint(pageID, 10, 32)
	if err != nil {
		return c.Redirect("/admin/pages")
	}

	// Verify page exists before deletion
	_, err = apc.pageRepo.GetByID(uint(id))
	if err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "Seite nicht gefunden",
		}
		return flash.WithError(c, fm).Redirect("/admin/pages")
	}

	// Delete page using repository
	if err := apc.pageRepo.Delete(uint(id)); err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "Fehler beim Löschen der Seite: " + err.Error(),
		}
		return flash.WithError(c, fm).Redirect("/admin/pages")
	}

	// Success message
	fm := fiber.Map{
		"type":    "success",
		"message": "Seite erfolgreich gelöscht",
	}

	return flash.WithSuccess(c, fm).Redirect("/admin/pages")
}

// ============================================================================
// GLOBAL ADMIN PAGE CONTROLLER INSTANCE - Singleton Pattern
// ============================================================================

var adminPageController *AdminPageController

// InitializeAdminPageController initializes the global admin page controller
func InitializeAdminPageController() {
	pageRepo := repository.GetGlobalFactory().GetPageRepository()
	adminPageController = NewAdminPageController(pageRepo)
}

// GetAdminPageController returns the global admin page controller instance
func GetAdminPageController() *AdminPageController {
	if adminPageController == nil {
		InitializeAdminPageController()
	}
	return adminPageController
}
