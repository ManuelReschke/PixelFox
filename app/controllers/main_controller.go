package controllers

import (
	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
	"github.com/ManuelReschke/PixelFox/views"
	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/sujit-baniya/flash"
)

func RenderHello(c *fiber.Ctx) error {
	// fromProtected := c.Locals(FROM_PROTECTED).(bool)
	appENV := env.GetEnv("APP_ENV", "prod")
	isDEV := appENV == "dev"

	hindex := views.HomeIndex(false)
	home := views.Home("", false, false, flash.Get(c), hindex, isDEV)

	handler := adaptor.HTTPHandler(templ.Handler(home))

	return handler(c)

	// return c.Render("index", fiber.Map{
	// 	"FiberTitle": "Hello From Fiber Html Engine Test",
	// })
}
