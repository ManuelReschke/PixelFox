package router

import (
	"github.com/ManuelReschke/PixelFox/app/controllers"
	"github.com/ManuelReschke/PixelFox/internal/pkg/session"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/csrf"
	"time"
)

type HttpRouter struct {
}

func (h HttpRouter) InstallRouter(app *fiber.App) {
	// init session
	session.NewSessionStore()

	// API
	app.Get("/docs/api", loggedInMiddleware, controllers.HandleDocsAPI)

	// NO AUTH - GENERAL
	app.Get("/news", loggedInMiddleware, controllers.HandleNews)
	app.Get("/about", loggedInMiddleware, controllers.HandleAbout)
	app.Get("/contact", loggedInMiddleware, controllers.HandleContact)
	app.Get("/jobs", loggedInMiddleware, controllers.HandleJobs)

	// image processing status endpoint
	app.Get("/image/status/:uuid", loggedInMiddleware, controllers.HandleImageProcessingStatus)
	// image viewer
	app.Get("/image/:uuid", loggedInMiddleware, controllers.HandleImageViewer)

	// short url for sharing
	app.Get("/i/:sharelink", loggedInMiddleware, controllers.HandleShareLink)

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
	// Admin Search Route
	adminGroup.Get("/search", controllers.HandleAdminSearch)
	// Admin Queue Monitor Route
	adminGroup.Get("/queues", controllers.HandleAdminQueues)
	adminGroup.Get("/queues/data", controllers.HandleAdminQueuesData)

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
	group.Get("/user/settings", requireAuthMiddleware, controllers.HandleUserSettings)
	group.Get("/user/images", requireAuthMiddleware, controllers.HandleUserImages)
	group.Get("/user/images/load", requireAuthMiddleware, controllers.HandleLoadMoreImages)
	group.Get("/user/images/edit/:uuid", requireAuthMiddleware, controllers.HandleUserImageEdit)
	group.Post("/user/images/update/:uuid", requireAuthMiddleware, controllers.HandleUserImageUpdate)
	group.Get("/user/images/delete/:uuid", requireAuthMiddleware, controllers.HandleUserImageDelete)
}

func NewHttpRouter() *HttpRouter {
	return &HttpRouter{}
}

func loggedInMiddleware(c *fiber.Ctx) error {
	// Get session with error handling
	sess, err := session.GetSessionStore().Get(c)
	if err != nil {
		// On error: treat user as not logged in
		c.Locals(controllers.FROM_PROTECTED, false)
		return c.Next()
	}

	// Get user ID from session
	userId := sess.Get(controllers.USER_ID)
	
	// If no user ID exists, user is not logged in
	if userId == nil {
		c.Locals(controllers.FROM_PROTECTED, false)
		return c.Next()
	}

	// Get username from session
	userName := sess.Get(controllers.USER_NAME)
	
	// If user ID exists but username is missing, something is inconsistent
	if userName == nil {
		// Still consider as logged in, but log a warning
		c.Locals(controllers.FROM_PROTECTED, true)
		c.Locals(controllers.USER_ID, userId.(uint))
		return c.Next()
	}

	// User is fully logged in
	c.Locals(controllers.FROM_PROTECTED, true)
	c.Locals(controllers.USER_NAME, userName)
	c.Locals(controllers.USER_ID, userId.(uint))
	
	// We can use the global map, but only when necessary
	// This is thread-safe thanks to our mutex
	session.SetKeyValue(controllers.USER_NAME, userName.(string))

	return c.Next()
}

func requireAuthMiddleware(c *fiber.Ctx) error {
	// Get session with error handling
	sess, err := session.GetSessionStore().Get(c)
	if err != nil {
		// On error: redirect to login page
		return c.Redirect("/login", fiber.StatusSeeOther)
	}

	// Get user ID from session
	userId := sess.Get(controllers.USER_ID)
	
	// If no user ID exists, user is not logged in - redirect to login page
	if userId == nil {
		return c.Redirect("/login", fiber.StatusSeeOther)
	}

	// Get username from session
	userName := sess.Get(controllers.USER_NAME)
	
	// If user ID exists but username is missing, something is inconsistent
	if userName == nil {
		// Still consider as logged in, but only set user ID
		c.Locals(controllers.FROM_PROTECTED, true)
		c.Locals(controllers.USER_ID, userId.(uint))
		return c.Next()
	}

	// User is fully logged in - set all locals
	c.Locals(controllers.FROM_PROTECTED, true)
	c.Locals(controllers.USER_NAME, userName)
	c.Locals(controllers.USER_ID, userId.(uint))
	
	// Store username in the global map (thread-safe)
	session.SetKeyValue(controllers.USER_NAME, userName.(string))

	return c.Next()
}

func RequireAdminMiddleware(c *fiber.Ctx) error {
	// First check if user is authenticated
	sess, _ := session.GetSessionStore().Get(c)
	userID := sess.Get(controllers.USER_ID)

	// User is not logged in
	if userID == nil {
		return c.Redirect("/login", fiber.StatusSeeOther)
	}

	// Check if user is admin based on session value
	isAdmin := sess.Get(controllers.USER_IS_ADMIN)
	if isAdmin == nil || isAdmin.(bool) != true {
		// User is not an admin
		return c.Redirect("/", fiber.StatusSeeOther)
	}

	// Set user info in context
	session.SetKeyValue(controllers.USER_NAME, sess.Get(controllers.USER_NAME).(string))
	c.Locals(controllers.FROM_PROTECTED, true)
	c.Locals(controllers.USER_NAME, sess.Get(controllers.USER_NAME))
	c.Locals(controllers.USER_ID, userID.(uint))
	c.Locals(controllers.USER_IS_ADMIN, true)

	return c.Next()
}
