package router

import (
	"github.com/ManuelReschke/PixelFox/app/controllers"
	"github.com/ManuelReschke/PixelFox/internal/pkg/middleware"
	"github.com/gofiber/fiber/v2"
	gothfiber "github.com/shareed2k/goth_fiber"
)

func (h HttpRouter) registerPublicRoutes(app *fiber.App) {
	// API routes moved to ApiRouter (internal/pkg/router/api_router.go)
	app.Get("/docs/api", loggedInMiddleware, controllers.HandleDocsAPI)

	// Public news + static pages
	app.Get("/news", loggedInMiddleware, controllers.HandleNewsIndex)
	app.Get("/news/:slug", loggedInMiddleware, controllers.HandleNewsShow)
	app.Get("/about", loggedInMiddleware, controllers.HandleAbout)
	app.Get("/contact", loggedInMiddleware, controllers.HandleContact)
	app.Get("/pricing", loggedInMiddleware, controllers.HandlePricing)
	app.Get("/jobs", loggedInMiddleware, controllers.HandleJobs)

	// Public image pages
	app.Get("/images/:uuid/status", loggedInMiddleware, controllers.HandleImageProcessingStatus)
	app.Get("/image/:uuid", loggedInMiddleware, controllers.HandleImageViewer)

	// Short share URLs
	app.Get("/i/:sharelink", loggedInMiddleware, controllers.HandleShareLink)
	app.Get("/a/:sharelink", loggedInMiddleware, controllers.HandleAlbumShareLink)

	// Public page display
	app.Get("/page/:slug", loggedInMiddleware, controllers.HandlePageDisplay)

	// Flash helpers
	app.Get("/flash/upload-rate-limit", loggedInMiddleware, controllers.HandleFlashUploadRateLimit)
	app.Get("/flash/upload-duplicate", loggedInMiddleware, controllers.HandleFlashUploadDuplicate)
	app.Get("/flash/upload-error", loggedInMiddleware, controllers.HandleFlashUploadError)
	app.Get("/flash/upload-too-large", loggedInMiddleware, controllers.HandleFlashUploadTooLarge)
	app.Get("/flash/upload-unsupported-type", loggedInMiddleware, controllers.HandleFlashUploadUnsupportedType)

	// Auth
	app.Post("/logout", middleware.RequireAuth, controllers.HandleAuthLogout)

	// Social OAuth
	app.Get("/auth/:provider", gothfiber.BeginAuthHandler)
	app.Get("/auth/:provider/callback", controllers.HandleOAuthCallback)

	// Billing provider webhooks (no CSRF, signature-verified in controller)
	app.Post("/webhooks/patreon", controllers.HandlePatreonWebhook)
}
