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

	csrfConf := csrf.Config{
		KeyLookup:  "form:_csrf",
		ContextKey: "csrf",
	}

	// API
	app.Get("/docs/api", loggedInMiddleware, controllers.HandleDocsAPI)

	// NO AUTH - GENERAL
	app.Get("/news", loggedInMiddleware, controllers.HandleNews)
	app.Get("/about", loggedInMiddleware, controllers.HandleAbout)
	app.Get("/contact", loggedInMiddleware, controllers.HandleContact)
	app.Get("/jobs", loggedInMiddleware, controllers.HandleJobs)
	// Image Viewer
	app.Get("/image/:uuid", loggedInMiddleware, controllers.HandleImageViewer)
	// ShareLink Shortener Route
	app.Get("/i/:sharelink", loggedInMiddleware, controllers.HandleShareLink)

	// AUTH
	app.Post("/logout", loggedInMiddleware, controllers.HandleAuthLogout)

	// AUTH + ADD CORS AND CSRF
	group := app.Group("", cors.New(), csrf.New(csrfConf))
	group.Get("/", loggedInMiddleware, controllers.HandleStart)
	group.Post("/upload", loggedInMiddleware, controllers.HandleUpload)
	group.Get("/login", loggedInMiddleware, controllers.HandleAuthLogin)
	group.Post("/login", loggedInMiddleware, controllers.HandleAuthLogin)
	group.Get("/register", loggedInMiddleware, controllers.HandleAuthRegister)
	group.Post("/register", loggedInMiddleware, controllers.HandleAuthRegister)
	group.Get("/user/profile", loggedInMiddleware, controllers.HandleUserProfile)
	group.Get("/user/settings", loggedInMiddleware, controllers.HandleUserSettings)

}

func NewHttpRouter() *HttpRouter {
	return &HttpRouter{}
}

func loggedInMiddleware(c *fiber.Ctx) error {
	sess, _ := session.GetSessionStore().Get(c)
	userId := sess.Get(controllers.USER_ID)
	if userId == nil {
		c.Locals(controllers.FROM_PROTECTED, false)

		return c.Next()
	}

	// set session values
	session.SetKeyValue(controllers.USER_NAME, sess.Get(controllers.USER_NAME).(string))

	// set locals fiber context values
	c.Locals(controllers.FROM_PROTECTED, true)
	c.Locals(controllers.USER_NAME, sess.Get(controllers.USER_NAME))
	c.Locals(controllers.USER_ID, userId.(uint))

	return c.Next()
}
