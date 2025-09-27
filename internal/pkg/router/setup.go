package router

import (
	"github.com/gofiber/fiber/v2"
)

func InstallRouter(app *fiber.App) {
	// Install HttpRouter first to initialize session store, oauth providers,
	// and the global UserContext middleware. Then register API routes which
	// depend on that middleware (e.g., requireAuthMiddleware).
	setup(app, NewHttpRouter(), NewApiRouter())
}
func setup(app *fiber.App, router ...Router) {
	for _, r := range router {
		r.InstallRouter(app)
	}
}
