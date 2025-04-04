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
}

func NewHttpRouter() *HttpRouter {
	return &HttpRouter{}
}

func loggedInMiddleware(c *fiber.Ctx) error {
	sess, _ := session.GetSessionStore().Get(c)
	userId := sess.Get(controllers.USER_ID)
	if userId == nil {
		// Benutzer ist nicht eingeloggt
		c.Locals(controllers.FROM_PROTECTED, false)
		return c.Next()
	}

	// Benutzer ist eingeloggt
	userName := sess.Get(controllers.USER_NAME)
	if userName != nil {
		// set session values
		session.SetKeyValue(controllers.USER_NAME, userName.(string))

		// set locals fiber context values
		c.Locals(controllers.FROM_PROTECTED, true)
		c.Locals(controllers.USER_NAME, userName)
		c.Locals(controllers.USER_ID, userId.(uint))
	}

	return c.Next()
}

func requireAuthMiddleware(c *fiber.Ctx) error {
	sess, _ := session.GetSessionStore().Get(c)
	userId := sess.Get(controllers.USER_ID)
	if userId == nil {
		// Benutzer ist nicht eingeloggt, leite zum Login-Formular weiter
		return c.Redirect("/login", fiber.StatusSeeOther)
	}

	// set session values
	session.SetKeyValue(controllers.USER_NAME, sess.Get(controllers.USER_NAME).(string))

	// set locals fiber context values
	c.Locals(controllers.FROM_PROTECTED, true)
	c.Locals(controllers.USER_NAME, sess.Get(controllers.USER_NAME))
	c.Locals(controllers.USER_ID, userId.(uint))

	return c.Next()
}
