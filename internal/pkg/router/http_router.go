package router

import (
	"github.com/ManuelReschke/PixelFox/app/controllers"
	"github.com/ManuelReschke/PixelFox/internal/pkg/middleware"
	"github.com/ManuelReschke/PixelFox/internal/pkg/session"

	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/csrf"
)

type HttpRouter struct {
}

func (h HttpRouter) InstallRouter(app *fiber.App) {
	// init session
	session.NewSessionStore()

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

	// API
	app.Get("/docs/api", loggedInMiddleware, controllers.HandleDocsAPI)
	// Phase 2: Direct-to-Storage Upload Session API (requires auth)
	app.Post("/api/v1/upload/sessions", requireAuthMiddleware, controllers.HandleCreateUploadSession)
	// Storage-side upload endpoint (token protected) with CORS for cross-origin uploads
	app.Post("/api/internal/upload", cors.New(cors.Config{
		AllowOrigins:     "*",
		AllowHeaders:     "Authorization, Content-Type",
		AllowMethods:     "POST, OPTIONS",
		AllowCredentials: false,
	}), controllers.HandleStorageDirectUpload)
	// Lightweight reachability: HEAD returns 204 (used by health monitor)
	app.Head("/api/internal/upload", controllers.HandleStorageUploadHead)
	// Replication endpoint for server-to-server file copy (HTTP Push)
	app.Put("/api/internal/replicate", controllers.HandleStorageReplicate)

	// NO AUTH - GENERAL
	//app.Get("/news", loggedInMiddleware, controllers.HandleNews)
	// Public News Routes
	app.Get("/news", loggedInMiddleware, controllers.HandleNewsIndex)
	app.Get("/news/:slug", loggedInMiddleware, controllers.HandleNewsShow)
	app.Get("/about", loggedInMiddleware, controllers.HandleAbout)
	app.Get("/contact", loggedInMiddleware, controllers.HandleContact)
	app.Get("/pricing", loggedInMiddleware, controllers.HandlePricing)
	app.Get("/jobs", loggedInMiddleware, controllers.HandleJobs)

	// image processing status endpoint
	app.Get("/image/status/:uuid", loggedInMiddleware, controllers.HandleImageProcessingStatus)
	app.Get("/api/v1/image/status/:uuid", loggedInMiddleware, controllers.HandleImageStatusJSON)
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
	app.Post("/logout", requireAuthMiddleware, controllers.HandleAuthLogout)

	// Admin routes
	adminGroup := app.Group("/admin", RequireAdminMiddleware)
	adminGroup.Get("/", controllers.HandleAdminDashboard)
	adminGroup.Get("/users", controllers.HandleAdminUsers)
	adminGroup.Get("/users/edit/:id", controllers.HandleAdminUserEdit)
	adminGroup.Post("/users/update/:id", controllers.HandleAdminUserUpdate)
	adminGroup.Get("/users/delete/:id", controllers.HandleAdminUserDelete)
	// Resend activation email
	adminGroup.Post("/users/resend-activation/:id", controllers.HandleAdminResendActivation)
	// Admin Image Management Routes
	adminGroup.Get("/images", controllers.HandleAdminImages)
	adminGroup.Get("/images/edit/:uuid", controllers.HandleAdminImageEdit)
	adminGroup.Post("/images/update/:uuid", controllers.HandleAdminImageUpdate)
	adminGroup.Get("/images/delete/:uuid", controllers.HandleAdminImageDelete)
	// Admin News Management Routes
	adminGroup.Get("/news", controllers.HandleAdminNews)
	adminGroup.Get("/news/create", controllers.HandleAdminNewsCreate)
	adminGroup.Post("/news/store", controllers.HandleAdminNewsStore)
	adminGroup.Get("/news/edit/:id", controllers.HandleAdminNewsEdit)
	adminGroup.Post("/news/update/:id", controllers.HandleAdminNewsUpdate)
	adminGroup.Get("/news/delete/:id", controllers.HandleAdminNewsDelete)
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
	adminGroup.Get("/storage/delete/:id", controllers.HandleAdminDeleteStoragePool)
	// Admin Page Management Routes (moved to CSRF protected routes below)

	csrfConf := csrf.Config{
		KeyLookup:      "form:_csrf",
		ContextKey:     "csrf",
		CookieName:     "csrf_",
		CookieSameSite: "Lax",
		Expiration:     1 * time.Hour,
		CookieSecure:   false, // Im Entwicklungsmodus auf false setzen
	}
	// setup group for csrf protected routes
	group := app.Group("", cors.New(), csrf.New(csrfConf))
	group.Get("/", loggedInMiddleware, controllers.HandleStart)
	group.Post("/upload", requireAuthMiddleware, controllers.HandleUpload)
	group.Get("/login", loggedInMiddleware, controllers.HandleAuthLogin)
	group.Post("/login", loggedInMiddleware, controllers.HandleAuthLogin)
	group.Get("/register", loggedInMiddleware, controllers.HandleAuthRegister)
	group.Post("/register", loggedInMiddleware, controllers.HandleAuthRegister)
	group.Get("/activate", loggedInMiddleware, controllers.HandleAuthActivate)
	group.Post("/activate", loggedInMiddleware, controllers.HandleAuthActivate)
	group.Get("/user/profile", requireAuthMiddleware, controllers.HandleUserProfile)
	group.Get("/user/profile/edit", requireAuthMiddleware, controllers.HandleUserProfileEdit)
	group.Post("/user/profile/edit", requireAuthMiddleware, controllers.HandleUserProfileEditPost)
	group.Get("/user/profile/verify-email-change", controllers.HandleEmailChangeVerification)
	group.Get("/user/profile/edit/cancel-email-change", requireAuthMiddleware, controllers.HandleCancelEmailChange)
	group.Get("/user/profile/edit/resend-email-change", requireAuthMiddleware, controllers.HandleResendEmailChange)
	group.Get("/user/settings", requireAuthMiddleware, controllers.HandleUserSettings)
	group.Get("/user/images", requireAuthMiddleware, controllers.HandleUserImages)
	group.Get("/user/images/load", requireAuthMiddleware, controllers.HandleLoadMoreImages)
	group.Get("/user/images/edit/:uuid", requireAuthMiddleware, controllers.HandleUserImageEdit)
	group.Post("/user/images/update/:uuid", requireAuthMiddleware, controllers.HandleUserImageUpdate)
	group.Get("/user/images/delete/:uuid", requireAuthMiddleware, controllers.HandleUserImageDelete)
	// User Album Routes (CSRF protected)
	group.Get("/user/albums", requireAuthMiddleware, controllers.HandleUserAlbums)
	group.Get("/user/albums/create", requireAuthMiddleware, controllers.HandleUserAlbumCreate)
	group.Post("/user/albums/create", requireAuthMiddleware, controllers.HandleUserAlbumCreate)
	group.Get("/user/albums/:id", requireAuthMiddleware, controllers.HandleUserAlbumView)
	group.Get("/user/albums/edit/:id", requireAuthMiddleware, controllers.HandleUserAlbumEdit)
	group.Post("/user/albums/edit/:id", requireAuthMiddleware, controllers.HandleUserAlbumEdit)
	group.Get("/user/albums/delete/:id", requireAuthMiddleware, controllers.HandleUserAlbumDelete)
	group.Post("/user/albums/:id/add-image", requireAuthMiddleware, controllers.HandleUserAlbumAddImage)
	group.Post("/user/albums/:id/set-cover", requireAuthMiddleware, controllers.HandleUserAlbumSetCover)
	group.Get("/user/albums/:id/remove-image/:image_id", requireAuthMiddleware, controllers.HandleUserAlbumRemoveImage)
	// Admin Page Management Routes (CSRF protected)
	group.Get("/admin/pages", RequireAdminMiddleware, controllers.HandleAdminPages)
	group.Get("/admin/pages/create", RequireAdminMiddleware, controllers.HandleAdminPageCreate)
	group.Post("/admin/pages/store", RequireAdminMiddleware, controllers.HandleAdminPageStore)
	group.Get("/admin/pages/edit/:id", RequireAdminMiddleware, controllers.HandleAdminPageEdit)
	group.Post("/admin/pages/update/:id", RequireAdminMiddleware, controllers.HandleAdminPageUpdate)
	group.Get("/admin/pages/delete/:id", RequireAdminMiddleware, controllers.HandleAdminPageDelete)
	// Admin Settings Routes (CSRF protected)
	group.Get("/admin/settings", RequireAdminMiddleware, controllers.HandleAdminSettings)
	group.Post("/admin/settings", RequireAdminMiddleware, controllers.HandleAdminSettingsUpdate)
	// Admin Storage Pool Management Routes (CSRF protected)
	group.Get("/admin/storage/create", RequireAdminMiddleware, controllers.HandleAdminCreateStoragePool)
	group.Post("/admin/storage/create", RequireAdminMiddleware, controllers.HandleAdminCreateStoragePoolPost)
	group.Get("/admin/storage/edit/:id", RequireAdminMiddleware, controllers.HandleAdminEditStoragePool)
	group.Post("/admin/storage/edit/:id", RequireAdminMiddleware, controllers.HandleAdminEditStoragePoolPost)
	group.Get("/admin/storage/move/:id", RequireAdminMiddleware, controllers.HandleAdminMoveStoragePool)
	group.Post("/admin/storage/move/:id", RequireAdminMiddleware, controllers.HandleAdminMoveStoragePoolPost)
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

func requireAuthMiddleware(c *fiber.Ctx) error {
	// UserContextMiddleware already parsed session data
	// Just check if user is logged in, redirect if not
	if !c.Locals(controllers.FROM_PROTECTED).(bool) {
		return c.Redirect("/login", fiber.StatusSeeOther)
	}

	return c.Next()
}

func RequireAdminMiddleware(c *fiber.Ctx) error {
	// UserContextMiddleware already parsed session data
	// Check if user is logged in
	if !c.Locals(controllers.FROM_PROTECTED).(bool) {
		return c.Redirect("/login", fiber.StatusSeeOther)
	}

	// Check if user is admin
	if !c.Locals(controllers.USER_IS_ADMIN).(bool) {
		return c.Redirect("/", fiber.StatusSeeOther)
	}

	return c.Next()
}
