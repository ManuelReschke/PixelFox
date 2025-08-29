package controllers

import (
	"fmt"
	"strconv"
	"time"

	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/sujit-baniya/flash"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/app/repository"
	"github.com/ManuelReschke/PixelFox/internal/pkg/usercontext"
	"github.com/ManuelReschke/PixelFox/internal/pkg/viewmodel"
	"github.com/ManuelReschke/PixelFox/views"
	"github.com/ManuelReschke/PixelFox/views/admin_views"
)

// ============================================================================
// ADMIN NEWS CONTROLLER - Repository Pattern
// ============================================================================

// AdminNewsController handles admin news-related HTTP requests using repository pattern
type AdminNewsController struct {
	newsRepo repository.NewsRepository
}

// NewAdminNewsController creates a new admin news controller with repository
func NewAdminNewsController(newsRepo repository.NewsRepository) *AdminNewsController {
	return &AdminNewsController{
		newsRepo: newsRepo,
	}
}

// handleError is a helper method for consistent error handling
func (anc *AdminNewsController) handleError(c *fiber.Ctx, message string, err error) error {
	fm := fiber.Map{
		"type":    "error",
		"message": message + ": " + err.Error(),
	}
	return flash.WithError(c, fm).Redirect("/admin/news")
}

// HandleAdminNews renders the news management page using repository pattern
func (anc *AdminNewsController) HandleAdminNews(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	// Get all news articles using repository
	newsList, err := anc.newsRepo.GetAllWithoutPagination()
	if err != nil {
		return anc.handleError(c, "Fehler beim Laden der News-Artikel", err)
	}

	// Render the news management page
	newsManagement := admin_views.NewsManagement(newsList)
	home := views.HomeCtx(c, " | News-Verwaltung", userCtx.IsLoggedIn, false, flash.Get(c), newsManagement, userCtx.IsAdmin, &viewmodel.OpenGraph{
		Title:       "News-Verwaltung - PixelFox Admin",
		Description: "Verwaltung der News-Artikel",
		Image:       "/img/pixelfox-logo.png",
		URL:         "/admin/news",
	})

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleAdminNewsCreate renders the news creation page using repository pattern
func (anc *AdminNewsController) HandleAdminNewsCreate(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	// Render the news creation page
	newsCreate := admin_views.NewsCreate()
	home := views.HomeCtx(c, " | Neuen News-Artikel erstellen", userCtx.IsLoggedIn, false, flash.Get(c), newsCreate, userCtx.IsAdmin, &viewmodel.OpenGraph{
		Title:       "News erstellen - PixelFox Admin",
		Description: "Erstellen eines neuen News-Artikels",
		Image:       "/img/pixelfox-logo.png",
		URL:         "/admin/news/create",
	})

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleAdminNewsStore handles news creation using repository pattern
func (anc *AdminNewsController) HandleAdminNewsStore(c *fiber.Ctx) error {
	// Get user from session
	userID := uint64(c.Locals(USER_ID).(uint))

	// Parse form
	title := c.FormValue("title")
	content := c.FormValue("content")
	newsSlug := c.FormValue("slug")
	published := c.FormValue("published") == "1"

	// Validate form
	if title == "" || content == "" || newsSlug == "" {
		fm := fiber.Map{
			"type":    "error",
			"message": "Titel, Slug und Inhalt sind erforderlich",
		}
		return flash.WithError(c, fm).Redirect("/admin/news/create")
	}

	// Check if slug already exists using repository
	slugExists, err := anc.newsRepo.SlugExists(newsSlug)
	if err != nil {
		return anc.handleError(c, "Fehler beim Prüfen des Slugs", err)
	}

	if slugExists {
		// Slug already exists, append timestamp
		newsSlug = fmt.Sprintf("%s-%d", newsSlug, time.Now().Unix())
	}

	// Create news article
	news := &models.News{
		Title:     title,
		Content:   content,
		Slug:      newsSlug,
		Published: published,
		UserID:    userID,
	}

	// Save using repository
	if err := anc.newsRepo.Create(news); err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "Fehler beim Erstellen des News-Artikels: " + err.Error(),
		}
		return flash.WithError(c, fm).Redirect("/admin/news/create")
	}

	// Success message
	fm := fiber.Map{
		"type":    "success",
		"message": "News-Artikel erfolgreich erstellt",
	}

	return flash.WithSuccess(c, fm).Redirect("/admin/news")
}

// HandleAdminNewsEdit renders the news edit page using repository pattern
func (anc *AdminNewsController) HandleAdminNewsEdit(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	// Get news ID from URL
	idParam := c.Params("id")
	if idParam == "" {
		return c.Redirect("/admin/news")
	}

	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		return c.Redirect("/admin/news")
	}

	// Get news article using repository
	news, err := anc.newsRepo.GetByID(uint(id))
	if err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "News-Artikel nicht gefunden",
		}
		return flash.WithError(c, fm).Redirect("/admin/news")
	}

	// Render the news edit page
	newsEdit := admin_views.NewsEdit(*news)
	home := views.HomeCtx(c, " | News-Artikel bearbeiten", userCtx.IsLoggedIn, false, flash.Get(c), newsEdit, userCtx.IsAdmin, &viewmodel.OpenGraph{
		Title:       "News bearbeiten - PixelFox Admin",
		Description: "Bearbeiten eines News-Artikels",
		Image:       "/img/pixelfox-logo.png",
		URL:         "/admin/news/edit/" + idParam,
	})

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleAdminNewsUpdate handles news update using repository pattern
func (anc *AdminNewsController) HandleAdminNewsUpdate(c *fiber.Ctx) error {
	// Get news ID from URL
	idParam := c.Params("id")
	if idParam == "" {
		return c.Redirect("/admin/news")
	}

	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		return c.Redirect("/admin/news")
	}

	// Get news article using repository
	news, err := anc.newsRepo.GetByID(uint(id))
	if err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "News-Artikel nicht gefunden",
		}
		return flash.WithError(c, fm).Redirect("/admin/news")
	}

	// Parse form
	title := c.FormValue("title")
	content := c.FormValue("content")
	newsSlug := c.FormValue("slug")
	published := c.FormValue("published") == "1"

	// Validate form
	if title == "" || content == "" || newsSlug == "" {
		fm := fiber.Map{
			"type":    "error",
			"message": "Titel, Slug und Inhalt sind erforderlich",
		}
		return flash.WithError(c, fm).Redirect("/admin/news/edit/" + idParam)
	}

	// Check if slug changed and if it already exists
	if newsSlug != news.Slug {
		slugExists, err := anc.newsRepo.SlugExistsExceptID(newsSlug, uint(id))
		if err != nil {
			return anc.handleError(c, "Fehler beim Prüfen des Slugs", err)
		}

		if slugExists {
			// Slug already exists, append timestamp
			newsSlug = fmt.Sprintf("%s-%d", newsSlug, time.Now().Unix())
		}
	}

	// Update news article
	news.Title = title
	news.Content = content
	news.Slug = newsSlug
	news.Published = published

	// Save using repository
	if err := anc.newsRepo.Update(news); err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "Fehler beim Aktualisieren des News-Artikels: " + err.Error(),
		}
		return flash.WithError(c, fm).Redirect("/admin/news/edit/" + idParam)
	}

	// Success message
	fm := fiber.Map{
		"type":    "success",
		"message": "News-Artikel erfolgreich aktualisiert",
	}

	return flash.WithSuccess(c, fm).Redirect("/admin/news")
}

// HandleAdminNewsDelete handles news deletion using repository pattern
func (anc *AdminNewsController) HandleAdminNewsDelete(c *fiber.Ctx) error {
	// Get news ID from URL
	idParam := c.Params("id")
	if idParam == "" {
		return c.Redirect("/admin/news")
	}

	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		return c.Redirect("/admin/news")
	}

	// Verify news exists before deletion
	_, err = anc.newsRepo.GetByID(uint(id))
	if err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "News-Artikel nicht gefunden",
		}
		return flash.WithError(c, fm).Redirect("/admin/news")
	}

	// Delete news article using repository
	if err := anc.newsRepo.Delete(uint(id)); err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "Fehler beim Löschen des News-Artikels: " + err.Error(),
		}
		return flash.WithError(c, fm).Redirect("/admin/news")
	}

	// Success message
	fm := fiber.Map{
		"type":    "success",
		"message": "News-Artikel erfolgreich gelöscht",
	}

	return flash.WithSuccess(c, fm).Redirect("/admin/news")
}

// ============================================================================
// GLOBAL ADMIN NEWS CONTROLLER INSTANCE - Singleton Pattern
// ============================================================================

var adminNewsController *AdminNewsController

// InitializeAdminNewsController initializes the global admin news controller
func InitializeAdminNewsController() {
	newsRepo := repository.GetGlobalFactory().GetNewsRepository()
	adminNewsController = NewAdminNewsController(newsRepo)
}

// GetAdminNewsController returns the global admin news controller instance
func GetAdminNewsController() *AdminNewsController {
	if adminNewsController == nil {
		InitializeAdminNewsController()
	}
	return adminNewsController
}

//// ============================================================================
//// PUBLIC NEWS FUNCTIONS - These remain standalone and use direct DB access
//// ============================================================================
//
//// HandleNewsIndex renders the public news page
//func HandleNewsIndex(c *fiber.Ctx) error {
//	// Get published news articles
//	var newsList []models.News
//	result := database.DB.Preload("User").Where("published = ?", true).Order("created_at DESC").Find(&newsList)
//	if result.Error != nil {
//		return c.Status(fiber.StatusInternalServerError).SendString("Failed to fetch news articles")
//	}
//
//	// Render the news index page
//	newsIndex := views.NewsIndex(newsList, isLoggedIn(c), fiber.Map(flash.Get(c)))
//
//	handler := adaptor.HTTPHandler(templ.Handler(newsIndex))
//	return handler(c)
//}
//
//// HandleNewsShow renders a single news article
//func HandleNewsShow(c *fiber.Ctx) error {
//	// Get news slug from URL
//	newsSlug := c.Params("slug")
//
//	// Get news article
//	var news models.News
//	result := database.DB.Preload("User").Where("slug = ? AND published = ?", newsSlug, true).First(&news)
//	if result.Error != nil {
//		return c.Status(fiber.StatusNotFound).SendString("News article not found")
//	}
//
//	// Render the news show page
//	newsShow := views.NewsShow(news, isLoggedIn(c), fiber.Map(flash.Get(c)))
//
//	handler := adaptor.HTTPHandler(templ.Handler(newsShow))
//	return handler(c)
//}
