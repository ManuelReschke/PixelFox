package controllers

import (
	"strings"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/usercontext"
	"github.com/ManuelReschke/PixelFox/internal/pkg/viewmodel"
	"github.com/ManuelReschke/PixelFox/views"
	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/sujit-baniya/flash"
)

// HandleNewsIndex renders the public news page
func HandleNewsIndex(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)

	// Get published news articles
	var newsList []models.News
	result := database.DB.Preload("User").Where("published = ?", true).Order("created_at DESC").Find(&newsList)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to fetch news articles")
	}

	// Render the news index page using HomeCtx layout
	newsContent := views.NewsContent(newsList)
	ogViewModel := &viewmodel.OpenGraph{
		Title:       "News - PixelFox",
		Description: "Aktuelle News und Updates von PixelFox",
		Image:       "/img/pixelfox-logo.png",
		URL:         "/news",
	}
	home := views.HomeCtx(c, " | News", userCtx.IsLoggedIn, false, flash.Get(c), newsContent, userCtx.IsAdmin, ogViewModel)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleNewsShow renders a single news article
func HandleNewsShow(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)

	// Get news slug from URL
	newsSlug := c.Params("slug")

	// Get news article
	var news models.News
	result := database.DB.Preload("User").Where("slug = ? AND published = ?", newsSlug, true).First(&news)
	if result.Error != nil {
		return c.Status(fiber.StatusNotFound).SendString("News article not found")
	}

	// Render the news show page using HomeCtx layout
	newsContent := views.NewsShowContent(news)
	ogViewModel := &viewmodel.OpenGraph{
		Title:       news.Title + " - PixelFox News",
		Description: stripHTMLAndTruncate(news.Content, 150),
		Image:       "/img/pixelfox-logo.png",
		URL:         "/news/" + news.Slug,
	}
	home := views.HomeCtx(c, " | "+news.Title, userCtx.IsLoggedIn, false, flash.Get(c), newsContent, userCtx.IsAdmin, ogViewModel)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// Helper function to strip HTML and truncate content for OpenGraph descriptions
func stripHTMLAndTruncate(html string, maxLength int) string {
	// Very basic HTML stripping - in a real app you'd want a proper HTML parser
	text := strings.ReplaceAll(html, "<br>", " ")
	text = strings.ReplaceAll(text, "<p>", "")
	text = strings.ReplaceAll(text, "</p>", " ")
	text = strings.ReplaceAll(text, "<div>", "")
	text = strings.ReplaceAll(text, "</div>", " ")

	// Remove other HTML tags
	var result strings.Builder
	var inTag bool
	for _, r := range text {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}

	// Truncate to maxLength
	stripped := result.String()
	if len(stripped) <= maxLength {
		return stripped
	}

	return stripped[:maxLength]
}
