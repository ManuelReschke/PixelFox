package main

import (
	"fmt"
	"log"

	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
	"github.com/ManuelReschke/PixelFox/internal/pkg/router"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/monitor"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/template/html/v2"
)

func main() {
	app := NewApplication()
	err := app.Listen(fmt.Sprintf("%s:%s", env.GetEnv("APP_HOST", "localhost"), env.GetEnv("APP_PORT", "4000")))
	log.Fatal(err)
}

func NewApplication() *fiber.App {
	env.SetupEnvFile()
	database.SetupDatabase()
	engine := html.New("./views", ".html")
	app := fiber.New(fiber.Config{Views: engine})
	app.Use(recover.New(), logger.New())
	app.Get("/metrics", monitor.New())
	app.Static("/", "./web/assets")
	router.InstallRouter(app)

	return app
}
