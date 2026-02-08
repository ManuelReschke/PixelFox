package router

import (
	"github.com/ManuelReschke/PixelFox/app/controllers"
	"github.com/ManuelReschke/PixelFox/internal/pkg/middleware"
	"github.com/ManuelReschke/PixelFox/internal/pkg/oauth"
	"github.com/ManuelReschke/PixelFox/internal/pkg/session"

	"github.com/gofiber/fiber/v2"
)

type HttpRouter struct {
}

func (h HttpRouter) InstallRouter(app *fiber.App) {
	// init session
	session.NewSessionStore()

	// init oauth providers
	oauth.Setup()

	// Apply UserContext middleware globally as first middleware
	app.Use(middleware.UserContextMiddleware)

	// Initialize admin controller with repositories
	controllers.InitializeAdminController()

	// Initialize admin news controller with repository
	controllers.InitializeAdminNewsController()

	// Initialize admin page controller with repository
	controllers.InitializeAdminPageController()

	// Initialize admin queue controller with repository
	controllers.InitializeAdminQueueController()

	// Initialize admin storage controller with repository
	controllers.InitializeAdminStorageController()

	// Initialize admin images controller with repository
	controllers.InitializeAdminImagesController()

	h.registerPublicRoutes(app)
	h.registerAdminRoutes(app)
	h.registerCSRFProtectedRoutes(app)
}

func NewHttpRouter() *HttpRouter {
	return &HttpRouter{}
}

func loggedInMiddleware(c *fiber.Ctx) error {
	// UserContextMiddleware already set all user context
	// This middleware now just passes through - no additional logic needed
	// All user information is available via usercontext.GetUserContext(c)
	return c.Next()
}

// Auth middlewares moved to internal/pkg/middleware/auth.go
