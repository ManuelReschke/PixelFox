package controllers

import (
	"fmt"
	"time"

	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/sujit-baniya/flash"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
	"github.com/ManuelReschke/PixelFox/internal/pkg/hcaptcha"
	"github.com/ManuelReschke/PixelFox/internal/pkg/session"
	"github.com/ManuelReschke/PixelFox/internal/pkg/statistics"
	auth_views "github.com/ManuelReschke/PixelFox/views/auth"
)

const (
	AUTH_KEY      string = "authenticated"
	USER_ID       string = "user_id"
	USER_NAME     string = "username"
	USER_IS_ADMIN string = "isAdmin"
)

func HandleAuthLogin(c *fiber.Ctx) error {
	fromProtected := c.Locals(FROM_PROTECTED).(bool)
	csrfToken := c.Locals("csrf").(string)

	lindex := auth_views.LoginIndex(fromProtected, csrfToken)
	login := auth_views.Login(
		" | Einloggen", fromProtected, false, flash.Get(c), "", lindex, false,
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
		sess.Set(USER_IS_ADMIN, user.Role == "admin")

		err = sess.Save()
		if err != nil {
			fm["message"] = fmt.Sprintf("something went wrong: %s", err)

			return flash.WithError(c, fm).Redirect("/login")
		}

		database.GetDB().Model(&user).Update("last_login_at", time.Now())

		fm = fiber.Map{
			"type":    "success",
			"message": "Glückwunsch du bist drin! Viel Spaß!",
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
		"message": "Bye bye! Auf wiedersehen.",
	}

	c.Locals(FROM_PROTECTED, false)
	// fromProtected = false

	return flash.WithSuccess(c, fm).Redirect("/login")
}

func HandleAuthRegister(c *fiber.Ctx) error {
	fromProtected := c.Locals(FROM_PROTECTED).(bool)
	csrfToken := c.Locals("csrf").(string)

	// Get hCaptcha site key from environment
	hcaptchaSitekey := env.GetEnv("HCAPTCHA_SITEKEY", "")

	rindex := auth_views.RegisterIndex(fromProtected, csrfToken, hcaptchaSitekey)
	register := auth_views.Register(
		" | Registrieren", fromProtected, false, flash.Get(c), "", rindex, false,
	)

	handler := adaptor.HTTPHandler(templ.Handler(register))

	if c.Method() == fiber.MethodPost {
		// Verify hCaptcha token
		hcaptchaToken := c.FormValue("h-captcha-response")
		valid, err := hcaptcha.Verify(hcaptchaToken)
		if err != nil || !valid {
			// Detailliertere Fehlermeldung für Debugging
			errorMsg := "Captcha validation failed. Please try again."
			if err != nil {
				// Im Entwicklungsmodus den genauen Fehler anzeigen
				if env.IsDev() {
					errorMsg = fmt.Sprintf("Captcha validation failed: %v", err)
				}
				// Fehler loggen
				fmt.Printf("hCaptcha validation error: %v\n", err)
			}
			
			fm := fiber.Map{
				"type":    "error",
				"message": errorMsg,
			}
			return flash.WithError(c, fm).Redirect("/register")
		}

		// Create user after successful captcha validation
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

		// Update statistics after registration
		go statistics.UpdateStatisticsCache()

		fm := fiber.Map{
			"type":    "success",
			"message": "Mega! Du hast dich erfolgreich registriert!",
		}

		return flash.WithSuccess(c, fm).Redirect("/login")
	}

	return handler(c)
}
