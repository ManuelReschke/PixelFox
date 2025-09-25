package router

import (
	apiv1 "github.com/ManuelReschke/PixelFox/internal/api/v1"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"strings"
	"time"
)

type ApiRouter struct {
}

func (h ApiRouter) InstallRouter(app *fiber.App) {
	max := models.GetAppSettings().GetAPIRateLimitPerMinute()
	if max < 0 {
		max = 0
	}
	api := app.Group("/api", limiter.New(limiter.Config{
		Max:        max,
		Expiration: 60 * time.Second,
		Next: func(c *fiber.Ctx) bool {
			p := c.Path()
			// Skip limiter for internal storage endpoints and image status polling
			if strings.HasPrefix(p, "/api/internal/") {
				return true
			}
			if strings.HasPrefix(p, "/api/v1/image/status/") {
				return true
			}
			// If Max == 0 => unlimited
			if max == 0 {
				return true
			}
			return false
		},
	}))
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
