package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/contrib/swagger"
	"github.com/gofiber/fiber/v2"
	fiberlog "github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/basicauth"
	"github.com/gofiber/fiber/v2/middleware/favicon"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/monitor"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/template/html/v2"

	"github.com/ManuelReschke/PixelFox/app/repository"
	"github.com/ManuelReschke/PixelFox/internal/pkg/cache"
	"github.com/ManuelReschke/PixelFox/internal/pkg/constants"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
	"github.com/ManuelReschke/PixelFox/internal/pkg/jobqueue"
	"github.com/ManuelReschke/PixelFox/internal/pkg/router"
)

func main() {
	app := NewApplication()

	// Start job queue manager
	jobManager := jobqueue.GetManager()
	jobManager.Start()

	// Setup graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		log.Println("Gracefully shutting down...")
		jobManager.Stop()
		app.Shutdown()
	}()

	// Start server
	log.Printf("Starting server on %s:%s", env.GetEnv("APP_HOST", "localhost"), env.GetEnv("APP_PORT", "4000"))
	err := app.Listen(fmt.Sprintf("%s:%s", env.GetEnv("APP_HOST", "localhost"), env.GetEnv("APP_PORT", "4000")))
	if err != nil {
		log.Printf("Server stopped: %v", err)
	}
}

func NewApplication() *fiber.App {
	env.SetupEnvFile()

	// Set log level based on environment
	if !env.IsDev() {
		fiberlog.SetLevel(fiberlog.LevelError)
	}

	database.SetupDatabase()
	cache.SetupCache()

	// Initialize repository factory
	repository.InitializeFactory(database.GetDB())

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
		File:         basePath + "public/icons/favicon.ico",
		URL:          "/favicon.ico",
		CacheControl: "public, max-age=604800",
	}))

	// recovery and logging
	app.Use(recover.New(), logger.New())

	// fiber metrics
	metricsPW := env.GetEnv("PROTECTED_ROUTE_METRICS_PW", "")
	app.Get("/metrics", basicauth.New(basicauth.Config{
		Users: map[string]string{
			"admin": metricsPW,
		},
	}), monitor.New())

	// static files
	app.Static("/", basePath+"public", fiber.Static{
		CacheDuration: 15 * time.Second,
		Compress:      true,
	})

	// static uploads
	app.Static(constants.UploadsRoute, basePath+"uploads", fiber.Static{
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
