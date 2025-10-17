package router

import (
	apiv1 "github.com/ManuelReschke/PixelFox/internal/api/v1"

	"strings"
	"time"

	"github.com/ManuelReschke/PixelFox/app/controllers"
	"github.com/ManuelReschke/PixelFox/app/models"
	appmw "github.com/ManuelReschke/PixelFox/internal/pkg/middleware"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
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

	// Attach conditional API key auth only for protected endpoints
	// Keep /ping public
	apiv1.RegisterHandlersWithOptions(v1, apiServer, apiv1.FiberServerOptions{
		Middlewares: []apiv1.MiddlewareFunc{
			func(c *fiber.Ctx) error {
				p := c.Path()
				// Endpoints requiring API key
				if strings.HasPrefix(p, "/api/v1/user/") || strings.HasPrefix(p, "/api/v1/upload/") || strings.HasPrefix(p, "/api/v1/images/") {
					return appmw.APIKeyAuthMiddleware()(c)
				}
				return c.Next()
			},
		},
	})

	// Route is provided by generated handlers via RegisterHandlersWithOptions

	// Internal API routes (private app APIs)
	internalAPI := api.Group("/internal")
	// Apply CORS for internal endpoints to support preflight (OPTIONS) and cross-node upload/replication
	internalAPI.Use(cors.New(cors.Config{
		AllowOrigins:     "*",
		AllowHeaders:     "Authorization, Content-Type",
		AllowMethods:     "POST, PUT, HEAD, OPTIONS",
		AllowCredentials: false,
	}))
	// Storage upload endpoints
	// Use API-session auth that returns JSON 401 instead of browser redirects
	internalAPI.Post("/upload/sessions", appmw.RequireAPISessionAuth, controllers.HandleCreateUploadSession)
	internalAPI.Post("/upload/batches", appmw.RequireAPISessionAuth, controllers.HandleCreateUploadBatch)
	internalAPI.Post("/upload", controllers.HandleStorageDirectUpload)
	// Preflight handler for upload endpoint
	internalAPI.Options("/upload", controllers.HandleStorageUploadHead)
	internalAPI.Head("/upload", controllers.HandleStorageUploadHead)
	internalAPI.Put("/replicate", controllers.HandleStorageReplicate)

	// Session-based endpoints used by the web app (safe auth check via usercontext)
	//internalAuth := func(c *fiber.Ctx) error {
	//	if !usercontext.IsLoggedIn(c) {
	//		// Return 401 JSON for internal API
	//		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized", "message": "login required"})
	//	}
	//	// Ensure legacy locals are populated for downstream handlers that may expect them
	//	uc := usercontext.GetUserContext(c)
	//	c.Locals(controllers.FROM_PROTECTED, true)
	//	c.Locals(controllers.USER_NAME, uc.Username)
	//	c.Locals(controllers.USER_ID, uc.UserID)
	//	c.Locals(controllers.USER_IS_ADMIN, uc.IsAdmin)
	//	return c.Next()
	//}
	internalAPI.Get("/images/:uuid/status", controllers.HandleImageStatusJSON)
}

func NewApiRouter() *ApiRouter {
	return &ApiRouter{}
}
