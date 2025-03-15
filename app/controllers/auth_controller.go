package controllers

import (
	"fmt"
	"time"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/session"
	"github.com/ManuelReschke/PixelFox/internal/pkg/statistics"
	auth "github.com/ManuelReschke/PixelFox/views/auth"
	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/sujit-baniya/flash"
)

const (
	AUTH_KEY  string = "authenticated"
	USER_ID   string = "user_id"
	USER_NAME string = "username"
)

func HandleAuthLogin(c *fiber.Ctx) error {
	fromProtected := c.Locals(FROM_PROTECTED).(bool)
	csrfToken := c.Locals("csrf").(string)

	lindex := auth.LoginIndex(fromProtected, csrfToken)
	login := auth.Login(
		" | Einloggen", fromProtected, false, flash.Get(c), lindex,
	)

	handler := adaptor.HTTPHandler(templ.Handler(login))

	if c.Method() == fiber.MethodPost {
		var (
			user models.User
			err  error
		)
		fm := fiber.Map{
			"type": "error",
		}

		// notice: in production you should not inform the user
		// with detailed messages about login failures
		result := database.GetDB().Where("email = ?", c.FormValue("email")).First(&user)
		if result.Error != nil {
			fm["message"] = "There is a problem with the login process"

			return flash.WithError(c, fm).Redirect("/login")
		}

		if models.CheckPasswordHash(c.FormValue("password"), user.Password) == false {
			fm["message"] = "There is a problem with the login process"

			return flash.WithError(c, fm).Redirect("/login")
		}

		sess, err := session.GetSessionStore().Get(c)
		if err != nil {
			fm["message"] = fmt.Sprintf("something went wrong: %s", err)

			return flash.WithError(c, fm).Redirect("/login")
		}

		sess.Set(AUTH_KEY, true)
		sess.Set(USER_ID, user.ID)
		sess.Set(USER_NAME, user.Name)

		err = sess.Save()
		if err != nil {
			fm["message"] = fmt.Sprintf("something went wrong: %s", err)

			return flash.WithError(c, fm).Redirect("/login")
		}

		database.GetDB().Model(&user).Update("last_login_at", time.Now())

		fm = fiber.Map{
			"type":    "success",
			"message": "You have successfully logged in!",
		}

		return flash.WithSuccess(c, fm).Redirect("/")
	}

	return handler(c)
}

func HandleAuthLogout(c *fiber.Ctx) error {
	fm := fiber.Map{
		"type": "error",
	}

	sess, err := session.GetSessionStore().Get(c)
	if err != nil {
		fm["message"] = "logged out (no sess)"

		return flash.WithError(c, fm).Redirect("/login")
	}

	err = sess.Destroy()
	if err != nil {
		fm["message"] = fmt.Sprintf("something went wrong: %s", err)

		return flash.WithError(c, fm).Redirect("/login")
	}

	fm = fiber.Map{
		"type":    "success",
		"message": "You have successfully logged out!!",
	}

	c.Locals(FROM_PROTECTED, false)
	// fromProtected = false

	return flash.WithSuccess(c, fm).Redirect("/login")
}

func HandleAuthRegister(c *fiber.Ctx) error {
	fromProtected := c.Locals(FROM_PROTECTED).(bool)
	csrfToken := c.Locals("csrf").(string)

	rindex := auth.RegisterIndex(fromProtected, csrfToken)
	register := auth.Register(
		" | Registrieren", fromProtected, false, flash.Get(c), rindex,
	)

	handler := adaptor.HTTPHandler(templ.Handler(register))

	if c.Method() == fiber.MethodPost {
		user, err := models.CreateUser(c.FormValue("username"), c.FormValue("email"), c.FormValue("password"))
		if err != nil {
			fm := fiber.Map{
				"type":    "error",
				"message": fmt.Sprintf("something went wrong: %s", err),
			}

			return flash.WithError(c, fm).Redirect("/register")
		}

		err = database.GetDB().Create(&user).Error
		if err != nil {
			fm := fiber.Map{
				"type":    "error",
				"message": fmt.Sprintf("something went wrong: %s", err),
			}

			return flash.WithError(c, fm).Redirect("/register")
		}

		// Aktualisiere die Statistiken nach der Registrierung
		go statistics.UpdateStatisticsCache()

		fm := fiber.Map{
			"type":    "success",
			"message": "You have successfully registered!",
		}

		return flash.WithSuccess(c, fm).Redirect("/login")
	}

	return handler(c)
}
