package controllers

import (
	"github.com/gofiber/fiber/v2"
)

func RenderHello(c *fiber.Ctx) error {
	// fromProtected := c.Locals(FROM_PROTECTED).(bool)
	//
	// hindex := views.HomeIndex(fromProtected)
	// home := views.Home("", fromProtected, false, flash.Get(c), hindex)
	//
	// handler := adaptor.HTTPHandler(templ.Handler(home))
	//
	// return handler(c)

	// return c.Render("index", fiber.Map{
	// 	"FiberTitle": "Hello From Fiber Html Engine Test",
	// })
}
