package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gofiber/contrib/swagger"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/basicauth"
	"github.com/gofiber/fiber/v2/middleware/favicon"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/monitor"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/template/html/v2"

	"github.com/ManuelReschke/PixelFox/internal/pkg/cache"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
	"github.com/ManuelReschke/PixelFox/internal/pkg/router"
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

	// Define possible base paths
	basePaths := []string{
		"./",        // Current directory
		"../../",    // From cmd/pixelfox to project root
		"../../../", // Fallback
	}

	// Find the correct base path
	basePath := ""
	for _, path := range basePaths {
		if _, err := os.Stat(path + "views"); !os.IsNotExist(err) {
			basePath = path
			break
		}
	}

	if basePath == "" {
		panic("Could not find project root directory")
	}

	// init fiber app
	app := fiber.New(fiber.Config{
		Views:     html.New(basePath+"views", ".html"),
		BodyLimit: 838860800, // 100 MiB or 104.5 MB
		// alternative:
		// StreamRequestBody: true
	})

	// ignore and cache favicon
	app.Use(favicon.New(favicon.Config{
		File:         basePath + "public/assets/icons/favicon.ico",
		URL:          "/favicon.ico",
		CacheControl: "public, max-age=604800",
	}))

	// recovery and logging
	app.Use(recover.New(), logger.New())

	// fiber metrics
	app.Get("/metrics", basicauth.New(basicauth.Config{
		Users: map[string]string{
			"admin": "test",
		},
	}), monitor.New())

	// static files
	app.Static("/", basePath+"public/assets", fiber.Static{
		CacheDuration: 15 * time.Second,
		Compress:      true,
	})

	// static uploads
	app.Static("/uploads", basePath+"uploads", fiber.Static{
		CacheDuration: 10 * time.Second,
		Compress:      false,
		MaxAge:        604800, // 7 days
	})

	// SWAGGER / OPENAPI
	openAPICfg := swagger.Config{
		BasePath: "/docs/api/",
		FilePath: basePath + "public/docs/v1/openapi.yml",
		Path:     "v1",
	}
	app.Use(swagger.New(openAPICfg))

	// ROUTER
	router.InstallRouter(app)

	return app
}
