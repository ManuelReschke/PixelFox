package main

import (
	"fmt"
	"log"
	"time"

	"github.com/gofiber/contrib/swagger"

	"github.com/ManuelReschke/PixelFox/internal/pkg/cache"
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
	cache.SetupCache()

	engine := html.New("./views", ".html")
	app := fiber.New(fiber.Config{
		Views:     engine,
		BodyLimit: 838860800, // 100 MiB or 104.5 MB
		// alternative:
		// StreamRequestBody: true
	})
	app.Use(recover.New(), logger.New())
	app.Get("/metrics", monitor.New())
	app.Static("/", "./public/assets", fiber.Static{
		CacheDuration: 15 * time.Second,
		Compress:      true,
	})
	// static uploads
	app.Static("/uploads", "./uploads", fiber.Static{
		CacheDuration: 10 * time.Second,
		Compress:      false,
		MaxAge:        604800, // 7 days
	})

	// SWAGGER / OPENAPI
	openAPICfg := swagger.Config{
		BasePath: "/docs/api/",
		FilePath: "./public/docs/v1/openapi.yml",
		Path:     "v1",
	}
	app.Use(swagger.New(openAPICfg))

	// ROUTER
	router.InstallRouter(app)

	return app
}
