package router

import (
	apiv1 "github.com/ManuelReschke/PixelFox/internal/api/v1"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

type ApiRouter struct {
}

func (h ApiRouter) InstallRouter(app *fiber.App) {
	api := app.Group("/api", limiter.New())
	api.Get("/", func(ctx *fiber.Ctx) error {
		return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
			"message": "Hello from api",
		})
	})

	// API v1 routes
	v1 := api.Group("/v1")
	apiServer := apiv1.NewAPIServer()
	apiv1.RegisterHandlers(v1, apiServer)
}

func NewApiRouter() *ApiRouter {
	return &ApiRouter{}
}
