package router

import (
	"github.com/ManuelReschke/PixelFox/app/controllers"
	"github.com/ManuelReschke/PixelFox/internal/pkg/session"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/csrf"
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

	csrfConf := csrf.Config{
		KeyLookup:  "form:_csrf",
		ContextKey: "csrf",
	}
	// setup group for csrf protected routes
	group := app.Group("", cors.New(), csrf.New(csrfConf))
	group.Get("/", loggedInMiddleware, controllers.HandleStart)
	group.Post("/upload", requireAuthMiddleware, controllers.HandleUpload)
	group.Get("/login", loggedInMiddleware, controllers.HandleAuthLogin)
	group.Post("/login", loggedInMiddleware, controllers.HandleAuthLogin)
	group.Get("/register", loggedInMiddleware, controllers.HandleAuthRegister)
	group.Post("/register", loggedInMiddleware, controllers.HandleAuthRegister)
	group.Get("/user/profile", requireAuthMiddleware, controllers.HandleUserProfile)
	group.Get("/user/settings", requireAuthMiddleware, controllers.HandleUserSettings)
	group.Get("/user/images", requireAuthMiddleware, controllers.HandleUserImages)
	group.Get("/user/images/load", requireAuthMiddleware, controllers.HandleLoadMoreImages)
}

func NewHttpRouter() *HttpRouter {
	return &HttpRouter{}
}

func loggedInMiddleware(c *fiber.Ctx) error {
	sess, _ := session.GetSessionStore().Get(c)
	userId := sess.Get(controllers.USER_ID)
	// user is not logged in
	if userId == nil {
		c.Locals(controllers.FROM_PROTECTED, false)
		return c.Next()
	}

	// user is logged in
	userName := sess.Get(controllers.USER_NAME)
	if userName != nil {
		session.SetKeyValue(controllers.USER_NAME, userName.(string))

		c.Locals(controllers.FROM_PROTECTED, true)
		c.Locals(controllers.USER_NAME, userName)
		c.Locals(controllers.USER_ID, userId.(uint))
	}

	return c.Next()
}

func requireAuthMiddleware(c *fiber.Ctx) error {
	sess, _ := session.GetSessionStore().Get(c)
	userId := sess.Get(controllers.USER_ID)
	// user is not logged in
	if userId == nil {
		return c.Redirect("/login", fiber.StatusSeeOther)
	}

	session.SetKeyValue(controllers.USER_NAME, sess.Get(controllers.USER_NAME).(string))

	c.Locals(controllers.FROM_PROTECTED, true)
	c.Locals(controllers.USER_NAME, sess.Get(controllers.USER_NAME))
	c.Locals(controllers.USER_ID, userId.(uint))

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
