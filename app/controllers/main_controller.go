package controllers

import (
	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/statistics"
	"github.com/ManuelReschke/PixelFox/internal/pkg/usercontext"
	"github.com/ManuelReschke/PixelFox/views"
	pages "github.com/ManuelReschke/PixelFox/views/pages"
	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/sujit-baniya/flash"
)

func HandleStart(c *fiber.Ctx) error {
	// Get user context - all user data centrally managed
	userCtx := usercontext.GetUserContext(c)
	csrfToken := c.Locals("csrf").(string)

	stats := statistics.GetStatisticsData()

	page := views.HomeIndex(userCtx.IsLoggedIn, csrfToken, stats)
	home := views.HomeCtx(c, "", userCtx.IsLoggedIn, false, flash.Get(c), page, userCtx.IsAdmin, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))

	return handler(c)
}

func HandleAbout(c *fiber.Ctx) error {
	// Get user context - all user data centrally managed
	userCtx := usercontext.GetUserContext(c)

	page := views.AboutPage()
	home := views.HomeCtx(c, "", userCtx.IsLoggedIn, false, flash.Get(c), page, userCtx.IsAdmin, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))

	return handler(c)
}

func HandleContact(c *fiber.Ctx) error {
	// Get user context - all user data centrally managed
	userCtx := usercontext.GetUserContext(c)

	page := views.ContactPage()
	home := views.HomeCtx(c, "", userCtx.IsLoggedIn, false, flash.Get(c), page, userCtx.IsAdmin, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))

	return handler(c)
}

func HandlePricing(c *fiber.Ctx) error {
	// Get user context - all user data centrally managed
	userCtx := usercontext.GetUserContext(c)

	page := views.PricingPage()
	home := views.HomeCtx(c, "", userCtx.IsLoggedIn, false, flash.Get(c), page, userCtx.IsAdmin, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))

	return handler(c)
}

func HandleJobs(c *fiber.Ctx) error {
	// Get user context - all user data centrally managed
	userCtx := usercontext.GetUserContext(c)

	page := pages.JobsPage()
	home := views.HomeCtx(c, "", userCtx.IsLoggedIn, false, flash.Get(c), page, userCtx.IsAdmin, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))

	return handler(c)
}

func HandleDocsAPI(c *fiber.Ctx) error {
	// Get user context - all user data centrally managed
	userCtx := usercontext.GetUserContext(c)

	page := views.APIPage()
	home := views.HomeCtx(c, "", userCtx.IsLoggedIn, false, flash.Get(c), page, userCtx.IsAdmin, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))

	return handler(c)
}

func HandlePageDisplay(c *fiber.Ctx) error {
	// Get slug from params
	slug := c.Params("slug")
	if slug == "" {
		return c.Status(fiber.StatusNotFound).Render("error/404", nil)
	}

	// Get page from database
	db := database.GetDB()
	page, err := models.FindPageBySlug(db, slug)
	if err != nil {
		return c.Status(fiber.StatusNotFound).Render("error/404", nil)
	}

	// Get user context - all user data centrally managed
	userCtx := usercontext.GetUserContext(c)

	// Create page view
	pageView := views.PageDisplay(*page)
	home := views.HomeCtx(c, " | "+page.Title, userCtx.IsLoggedIn, false, flash.Get(c), pageView, userCtx.IsAdmin, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))

	return handler(c)
}
