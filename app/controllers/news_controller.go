package controllers

import (
	"fmt"
	"time"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/viewmodel"
	"github.com/ManuelReschke/PixelFox/views"
	"github.com/ManuelReschke/PixelFox/views/admin_views"
	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/sujit-baniya/flash"
)

// HandleAdminNews renders the news management page
func HandleAdminNews(c *fiber.Ctx) error {
	// Get all news articles
	var newsList []models.News
	result := database.DB.Preload("User").Order("created_at DESC").Find(&newsList)
	if result.Error != nil {
		c.Locals("error", "Failed to fetch news articles")
		return c.Redirect("/admin")
	}

	// Render the news management page
	newsManagement := admin_views.NewsManagement(newsList)
	home := views.Home(" | News-Verwaltung", isLoggedIn(c), false, fiber.Map(flash.Get(c)), newsManagement, true, &viewmodel.OpenGraph{
		Title:       "News-Verwaltung - PixelFox Admin",
		Description: "Verwaltung der News-Artikel",
		Image:       "/img/pixelfox-logo.png",
		URL:         "/admin/news",
	})

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleAdminNewsCreate renders the news creation page
func HandleAdminNewsCreate(c *fiber.Ctx) error {
	// Render the news creation page
	newsCreate := admin_views.NewsCreate()
	home := views.Home(" | Neuen News-Artikel erstellen", isLoggedIn(c), false, fiber.Map(flash.Get(c)), newsCreate, true, &viewmodel.OpenGraph{
		Title:       "News erstellen - PixelFox Admin",
		Description: "Erstellen eines neuen News-Artikels",
		Image:       "/img/pixelfox-logo.png",
		URL:         "/admin/news/create",
	})

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleAdminNewsStore handles the news creation form submission
func HandleAdminNewsStore(c *fiber.Ctx) error {
	// Get user from session
	userID := uint64(c.Locals(USER_ID).(uint))

	// Parse form
	title := c.FormValue("title")
	content := c.FormValue("content")
	newsSlug := c.FormValue("slug")
	published := c.FormValue("published") == "1"

	// Validate form
	if title == "" || content == "" || newsSlug == "" {
		c.Locals("error", "Titel, Slug und Inhalt sind erforderlich")
		return c.Redirect("/admin/news/create")
	}

	// Check if slug already exists
	var existingNews models.News
	result := database.DB.Where("slug = ?", newsSlug).First(&existingNews)
	if result.Error == nil {
		// Slug already exists, append timestamp
		newsSlug = fmt.Sprintf("%s-%d", newsSlug, time.Now().Unix())
	}

	// Create news article
	news := models.News{
		Title:     title,
		Content:   content,
		Slug:      newsSlug,
		Published: published,
		UserID:    userID,
	}

	// Save to database
	result = database.DB.Create(&news)
	if result.Error != nil {
		c.Locals("error", "Fehler beim Erstellen des News-Artikels")
		return c.Redirect("/admin/news/create")
	}

	// Redirect to news list with success message
	c.Locals("success", "News-Artikel erfolgreich erstellt")
	return c.Redirect("/admin/news")
}

// HandleAdminNewsEdit renders the news edit page
func HandleAdminNewsEdit(c *fiber.Ctx) error {
	// Get news ID from URL
	id := c.Params("id")

	// Get news article
	var news models.News
	result := database.DB.First(&news, id)
	if result.Error != nil {
		c.Locals("error", "News-Artikel nicht gefunden")
		return c.Redirect("/admin/news")
	}

	// Render the news edit page
	newsEdit := admin_views.NewsEdit(news)
	home := views.Home(" | News-Artikel bearbeiten", isLoggedIn(c), false, fiber.Map(flash.Get(c)), newsEdit, true, &viewmodel.OpenGraph{
		Title:       "News bearbeiten - PixelFox Admin",
		Description: "Bearbeiten eines News-Artikels",
		Image:       "/img/pixelfox-logo.png",
		URL:         "/admin/news/edit/" + id,
	})

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleAdminNewsUpdate handles the news update form submission
func HandleAdminNewsUpdate(c *fiber.Ctx) error {
	// Get news ID from URL
	id := c.Params("id")

	// Get news article
	var news models.News
	result := database.DB.First(&news, id)
	if result.Error != nil {
		c.Locals("error", "News-Artikel nicht gefunden")
		return c.Redirect("/admin/news")
	}

	// Parse form
	title := c.FormValue("title")
	content := c.FormValue("content")
	newsSlug := c.FormValue("slug")
	published := c.FormValue("published") == "1"

	// Validate form
	if title == "" || content == "" || newsSlug == "" {
		c.Locals("error", "Titel, Slug und Inhalt sind erforderlich")
		return c.Redirect("/admin/news/edit/" + id)
	}

	// Check if slug changed and if it already exists
	if newsSlug != news.Slug {
		var existingNews models.News
		result := database.DB.Where("slug = ? AND id != ?", newsSlug, news.ID).First(&existingNews)
		if result.Error == nil {
			// Slug already exists, append timestamp
			newsSlug = fmt.Sprintf("%s-%d", newsSlug, time.Now().Unix())
		}
	}

	// Update news article
	news.Title = title
	news.Content = content
	news.Slug = newsSlug
	news.Published = published

	// Save to database
	result = database.DB.Save(&news)
	if result.Error != nil {
		c.Locals("error", "Fehler beim Aktualisieren des News-Artikels")
		return c.Redirect("/admin/news/edit/" + id)
	}

	// Redirect to news list with success message
	c.Locals("success", "News-Artikel erfolgreich aktualisiert")
	return c.Redirect("/admin/news")
}

// HandleAdminNewsDelete handles news deletion
func HandleAdminNewsDelete(c *fiber.Ctx) error {
	// Get news ID from URL
	id := c.Params("id")

	// Get news article
	var news models.News
	result := database.DB.First(&news, id)
	if result.Error != nil {
		c.Locals("error", "News-Artikel nicht gefunden")
		return c.Redirect("/admin/news")
	}

	// Delete news article
	result = database.DB.Delete(&news)
	if result.Error != nil {
		c.Locals("error", "Fehler beim Löschen des News-Artikels")
		return c.Redirect("/admin/news")
	}

	// Redirect to news list with success message
	c.Locals("success", "News-Artikel erfolgreich gelöscht")
	return c.Redirect("/admin/news")
}

// HandleNewsIndex renders the public news page
func HandleNewsIndex(c *fiber.Ctx) error {
	// Get published news articles
	var newsList []models.News
	result := database.DB.Preload("User").Where("published = ?", true).Order("created_at DESC").Find(&newsList)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to fetch news articles")
	}

	// Render the news index page
	newsIndex := views.NewsIndex(newsList, isLoggedIn(c), fiber.Map(flash.Get(c)))

	handler := adaptor.HTTPHandler(templ.Handler(newsIndex))
	return handler(c)
}

// HandleNewsShow renders a single news article
func HandleNewsShow(c *fiber.Ctx) error {
	// Get news slug from URL
	newsSlug := c.Params("slug")

	// Get news article
	var news models.News
	result := database.DB.Preload("User").Where("slug = ? AND published = ?", newsSlug, true).First(&news)
	if result.Error != nil {
		return c.Status(fiber.StatusNotFound).SendString("News article not found")
	}

	// Render the news show page
	newsShow := views.NewsShow(news, isLoggedIn(c), fiber.Map(flash.Get(c)))

	handler := adaptor.HTTPHandler(templ.Handler(newsShow))
	return handler(c)
}
