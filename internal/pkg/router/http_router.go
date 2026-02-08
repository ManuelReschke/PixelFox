package router

import (
	"github.com/ManuelReschke/PixelFox/app/controllers"
	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
	"github.com/ManuelReschke/PixelFox/internal/pkg/middleware"
	"github.com/ManuelReschke/PixelFox/internal/pkg/oauth"
	"github.com/ManuelReschke/PixelFox/internal/pkg/session"
	"github.com/gofiber/fiber/v2/middleware/cors"

	"time"

	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/csrf"
	gothfiber "github.com/shareed2k/goth_fiber"
)

type HttpRouter struct {
}

func (h HttpRouter) InstallRouter(app *fiber.App) {
	// init session
	session.NewSessionStore()

	// init oauth providers
	oauth.Setup()

	// Apply UserContext middleware globally as first middleware
	app.Use(middleware.UserContextMiddleware)

	// Initialize admin controller with repositories
	controllers.InitializeAdminController()

	// Initialize admin news controller with repository
	controllers.InitializeAdminNewsController()

	// Initialize admin page controller with repository
	controllers.InitializeAdminPageController()

	// Initialize admin queue controller with repository
	controllers.InitializeAdminQueueController()

	// Initialize admin storage controller with repository
	controllers.InitializeAdminStorageController()

	// Initialize admin images controller with repository
	controllers.InitializeAdminImagesController()

	// API routes moved to ApiRouter (internal/pkg/router/api_router.go)
	app.Get("/docs/api", loggedInMiddleware, controllers.HandleDocsAPI)

	// NO AUTH - GENERAL
	//app.Get("/news", loggedInMiddleware, controllers.HandleNews)
	// Public News Routes
	app.Get("/news", loggedInMiddleware, controllers.HandleNewsIndex)
	app.Get("/news/:slug", loggedInMiddleware, controllers.HandleNewsShow)
	app.Get("/about", loggedInMiddleware, controllers.HandleAbout)
	app.Get("/contact", loggedInMiddleware, controllers.HandleContact)
	app.Get("/pricing", loggedInMiddleware, controllers.HandlePricing)
	app.Get("/jobs", loggedInMiddleware, controllers.HandleJobs)

	// image processing status endpoint (HTML + API status in ApiRouter)
	app.Get("/images/:uuid/status", loggedInMiddleware, controllers.HandleImageProcessingStatus)
	// image viewer
	app.Get("/image/:uuid", loggedInMiddleware, controllers.HandleImageViewer)

	// short url for sharing
	app.Get("/i/:sharelink", loggedInMiddleware, controllers.HandleShareLink)
	app.Get("/a/:sharelink", loggedInMiddleware, controllers.HandleAlbumShareLink)

	// public page display
	app.Get("/page/:slug", loggedInMiddleware, controllers.HandlePageDisplay)

	// flash helpers
	app.Get("/flash/upload-rate-limit", loggedInMiddleware, controllers.HandleFlashUploadRateLimit)
	app.Get("/flash/upload-duplicate", loggedInMiddleware, controllers.HandleFlashUploadDuplicate)
	app.Get("/flash/upload-error", loggedInMiddleware, controllers.HandleFlashUploadError)
	app.Get("/flash/upload-too-large", loggedInMiddleware, controllers.HandleFlashUploadTooLarge)
	app.Get("/flash/upload-unsupported-type", loggedInMiddleware, controllers.HandleFlashUploadUnsupportedType)

	// auth
	app.Post("/logout", middleware.RequireAuth, controllers.HandleAuthLogout)

	// social oauth routes (public)
	app.Get("/auth/:provider", gothfiber.BeginAuthHandler)
	app.Get("/auth/:provider/callback", controllers.HandleOAuthCallback)

	// Admin routes
	adminGroup := app.Group("/admin", middleware.RequireAdmin)
	adminGroup.Get("/", controllers.HandleAdminDashboard)
	adminGroup.Get("/users", controllers.HandleAdminUsers)
	adminGroup.Get("/users/edit/:id", controllers.HandleAdminUserEdit)
	adminGroup.Post("/users/update/:id", controllers.HandleAdminUserUpdate)
	adminGroup.Post("/users/update-plan/:id", controllers.HandleAdminUserUpdatePlan)
	adminGroup.Post("/users/delete/:id", controllers.HandleAdminUserDelete)
	// Resend activation email
	adminGroup.Post("/users/resend-activation/:id", controllers.HandleAdminResendActivation)
	// Admin Image Management Routes
	adminGroup.Get("/images", controllers.HandleAdminImages)
	adminGroup.Get("/images/edit/:uuid", controllers.HandleAdminImageEdit)
	adminGroup.Post("/images/update/:uuid", controllers.HandleAdminImageUpdate)
	adminGroup.Post("/images/delete/:uuid", controllers.HandleAdminImageDelete)
	adminGroup.Post("/images/backup/:uuid", controllers.HandleAdminImageStartBackup)
	adminGroup.Post("/images/backup-delete/:uuid", controllers.HandleAdminImageDeleteBackup)
	// Admin News Management Routes
	adminGroup.Get("/news", controllers.HandleAdminNews)
	adminGroup.Get("/news/create", controllers.HandleAdminNewsCreate)
	adminGroup.Post("/news/store", controllers.HandleAdminNewsStore)
	adminGroup.Get("/news/edit/:id", controllers.HandleAdminNewsEdit)
	adminGroup.Post("/news/update/:id", controllers.HandleAdminNewsUpdate)
	adminGroup.Post("/news/delete/:id", controllers.HandleAdminNewsDelete)
	// Admin Search Route
	adminGroup.Get("/search", controllers.HandleAdminSearch)
	// Admin Queue Monitor Route
	adminGroup.Get("/queues", controllers.HandleAdminQueues)
	adminGroup.Get("/queues/data", controllers.HandleAdminQueuesData)
	adminGroup.Delete("/queues/delete/:key", controllers.HandleAdminQueueDelete)
	// Admin Storage Management Routes
	adminGroup.Get("/storage", controllers.HandleAdminStorageManagement)
	adminGroup.Get("/storage/health-check/:id", controllers.HandleAdminStoragePoolHealthCheck)
	adminGroup.Post("/storage/recalculate-usage/:id", controllers.HandleAdminRecalculateStorageUsage)
	adminGroup.Post("/storage/delete/:id", controllers.HandleAdminDeleteStoragePool)
	adminGroup.Post("/storage/tiering/sweep", controllers.HandleAdminTieringSweep)
	// Admin Page Management Routes (moved to CSRF protected routes below)

	csrfConf := csrf.Config{
		KeyLookup:      "form:_csrf",
		ContextKey:     "csrf",
		CookieName:     "csrf_",
		CookieSameSite: "Lax",
		Expiration:     1 * time.Hour,
		CookieSecure:   !env.IsDev(),
		// Never enforce CSRF on API routes
		Next: func(c *fiber.Ctx) bool {
			p := c.Path()
			return strings.HasPrefix(p, "/api/")
		},
	}
	// setup group for csrf protected routes
	group := app.Group("", cors.New(), csrf.New(csrfConf))
	group.Get("/", loggedInMiddleware, controllers.HandleStart)
	group.Post("/upload", middleware.RequireAuth, controllers.HandleUpload)
	group.Get("/upload/batch/:id", middleware.RequireAuth, controllers.HandleUploadBatchView)
	group.Post("/upload/batch/:id/album", middleware.RequireAuth, controllers.HandleUploadBatchSaveAsAlbum)
	group.Get("/login", loggedInMiddleware, controllers.HandleAuthLogin)
	group.Post("/login", loggedInMiddleware, controllers.HandleAuthLogin)
	group.Get("/register", loggedInMiddleware, controllers.HandleAuthRegister)
	group.Post("/register", loggedInMiddleware, controllers.HandleAuthRegister)
	group.Get("/activate", loggedInMiddleware, controllers.HandleAuthActivate)
	group.Post("/activate", loggedInMiddleware, controllers.HandleAuthActivate)
	group.Get("/user/profile", middleware.RequireAuth, controllers.HandleUserProfile)
	group.Get("/user/profile/edit", middleware.RequireAuth, controllers.HandleUserProfileEdit)
	group.Post("/user/profile/edit", middleware.RequireAuth, controllers.HandleUserProfileEditPost)
	group.Get("/user/profile/verify-email-change", controllers.HandleEmailChangeVerification)
	group.Get("/user/profile/edit/cancel-email-change", middleware.RequireAuth, controllers.HandleCancelEmailChange)
	group.Get("/user/profile/edit/resend-email-change", middleware.RequireAuth, controllers.HandleResendEmailChange)
	group.Get("/user/settings", middleware.RequireAuth, controllers.HandleUserSettings)
	group.Post("/user/settings", middleware.RequireAuth, controllers.HandleUserSettingsPost)
	group.Post("/user/settings/api-key", middleware.RequireAuth, controllers.HandleUserAPIKeyGenerate)
	group.Post("/user/settings/api-key/revoke", middleware.RequireAuth, controllers.HandleUserAPIKeyRevoke)
	group.Get("/user/images", middleware.RequireAuth, controllers.HandleUserImages)
	group.Get("/user/images/load", middleware.RequireAuth, controllers.HandleLoadMoreImages)
	group.Get("/user/images/edit/:uuid", middleware.RequireAuth, controllers.HandleUserImageEdit)
	group.Post("/user/images/update/:uuid", middleware.RequireAuth, controllers.HandleUserImageUpdate)
	group.Post("/user/images/delete/:uuid", middleware.RequireAuth, controllers.HandleUserImageDelete)
	// User Album Routes (CSRF protected)
	group.Get("/user/albums", middleware.RequireAuth, controllers.HandleUserAlbums)
	group.Get("/user/albums/create", middleware.RequireAuth, controllers.HandleUserAlbumCreate)
	group.Post("/user/albums/create", middleware.RequireAuth, controllers.HandleUserAlbumCreate)
	group.Get("/user/albums/:id", middleware.RequireAuth, controllers.HandleUserAlbumView)
	group.Get("/user/albums/edit/:id", middleware.RequireAuth, controllers.HandleUserAlbumEdit)
	group.Post("/user/albums/edit/:id", middleware.RequireAuth, controllers.HandleUserAlbumEdit)
	group.Post("/user/albums/delete/:id", middleware.RequireAuth, controllers.HandleUserAlbumDelete)
	group.Post("/user/albums/:id/add-image", middleware.RequireAuth, controllers.HandleUserAlbumAddImage)
	group.Post("/user/albums/:id/set-cover", middleware.RequireAuth, controllers.HandleUserAlbumSetCover)
	group.Get("/user/albums/:id/remove-image/:image_id", middleware.RequireAuth, controllers.HandleUserAlbumRemoveImage)
	// Image Reports (CSRF protected, guests allowed)
	group.Get("/image/:uuid/report", loggedInMiddleware, controllers.HandleImageReportForm)
	group.Post("/image/:uuid/report", loggedInMiddleware, controllers.HandleImageReportSubmit)
	// Admin Page Management Routes (CSRF protected)
	group.Get("/admin/pages", middleware.RequireAdmin, controllers.HandleAdminPages)
	group.Get("/admin/pages/create", middleware.RequireAdmin, controllers.HandleAdminPageCreate)
	group.Post("/admin/pages/store", middleware.RequireAdmin, controllers.HandleAdminPageStore)
	group.Get("/admin/pages/edit/:id", middleware.RequireAdmin, controllers.HandleAdminPageEdit)
	group.Post("/admin/pages/update/:id", middleware.RequireAdmin, controllers.HandleAdminPageUpdate)
	group.Post("/admin/pages/delete/:id", middleware.RequireAdmin, controllers.HandleAdminPageDelete)
	// Admin Settings Routes (CSRF protected)
	group.Get("/admin/settings", middleware.RequireAdmin, controllers.HandleAdminSettings)
	group.Post("/admin/settings", middleware.RequireAdmin, controllers.HandleAdminSettingsUpdate)
	// Admin Storage Pool Management Routes (CSRF protected)
	group.Get("/admin/storage/create", middleware.RequireAdmin, controllers.HandleAdminCreateStoragePool)
	group.Post("/admin/storage/create", middleware.RequireAdmin, controllers.HandleAdminCreateStoragePoolPost)
	group.Get("/admin/storage/edit/:id", middleware.RequireAdmin, controllers.HandleAdminEditStoragePool)
	group.Post("/admin/storage/edit/:id", middleware.RequireAdmin, controllers.HandleAdminEditStoragePoolPost)
	group.Get("/admin/storage/move/:id", middleware.RequireAdmin, controllers.HandleAdminMoveStoragePool)
	group.Post("/admin/storage/move/:id", middleware.RequireAdmin, controllers.HandleAdminMoveStoragePoolPost)
	// Admin Reports (CSRF protected)
	group.Get("/admin/reports", middleware.RequireAdmin, controllers.HandleAdminReports)
	group.Get("/admin/reports/:id", middleware.RequireAdmin, controllers.HandleAdminReportShow)
	group.Post("/admin/reports/:id/resolve", middleware.RequireAdmin, controllers.HandleAdminReportResolve)
	group.Post("/admin/reports/:id/dismiss", middleware.RequireAdmin, controllers.HandleAdminReportDismiss)
}

func NewHttpRouter() *HttpRouter {
	return &HttpRouter{}
}

func loggedInMiddleware(c *fiber.Ctx) error {
	// UserContextMiddleware already set all user context
	// This middleware now just passes through - no additional logic needed
	// All user information is available via usercontext.GetUserContext(c)
	return c.Next()
}

// Auth middlewares moved to internal/pkg/middleware/auth.go
