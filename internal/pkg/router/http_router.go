package router

import (
	"github.com/ManuelReschke/PixelFox/app/controllers"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/csrf"
)

type HttpRouter struct {
}

func (h HttpRouter) InstallRouter(app *fiber.App) {
	group := app.Group("", cors.New(), csrf.New())
	group.Get("/", controllers.HandleStart)
	group.Get("/login", controllers.HandleAuthLogin)
	group.Get("/register", controllers.HandleAuthRegister)
}

func NewHttpRouter() *HttpRouter {
	return &HttpRouter{}
}
