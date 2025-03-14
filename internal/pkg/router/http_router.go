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
	app.Get("/docs/api", controllers.HandleDocsAPI)

	// NO AUTH - GENERAL
	app.Get("/news", controllers.HandleNews)
	app.Get("/about", controllers.HandleAbout)
	app.Get("/contact", controllers.HandleContact)
	app.Get("/jobs", controllers.HandleJobs)
	app.Post("/logout", loggedInMiddleware, controllers.HandleAuthLogout)

	// AUTH CORS AND CSRF
	group := app.Group("", cors.New(), csrf.New(csrfConf))
	group.Get("/", loggedInMiddleware, controllers.HandleStart)
	group.Post("/upload", controllers.HandleUpload)

	// AUTH
	group.Get("/login", loggedInMiddleware, controllers.HandleAuthLogin)
	group.Post("/login", loggedInMiddleware, controllers.HandleAuthLogin)
	group.Get("/register", loggedInMiddleware, controllers.HandleAuthRegister)
	group.Post("/register", loggedInMiddleware, controllers.HandleAuthRegister)

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

	c.Locals(controllers.FROM_PROTECTED, true)
	c.Locals(controllers.USER_NAME, sess.Get(controllers.USER_NAME))
	session.SetKeyValue(controllers.USER_NAME, sess.Get(controllers.USER_NAME).(string))

	return c.Next()
}
