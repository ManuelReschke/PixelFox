package controllers

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/sujit-baniya/flash"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
	"github.com/ManuelReschke/PixelFox/internal/pkg/hcaptcha"
	"github.com/ManuelReschke/PixelFox/internal/pkg/mail"
	"github.com/ManuelReschke/PixelFox/internal/pkg/session"
	"github.com/ManuelReschke/PixelFox/internal/pkg/statistics"
	auth_views "github.com/ManuelReschke/PixelFox/views/auth"
	email_views "github.com/ManuelReschke/PixelFox/views/email_views"
)

const (
	AUTH_KEY       string = "authenticated"
	USER_ID        string = "user_id"
	USER_NAME      string = "username"
	USER_IS_ADMIN  string = "isAdmin"
	FROM_PROTECTED string = "from_protected"
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

		// Prevent login if account is not activated
		if !user.IsActive() {
			fm["message"] = "Bitte aktiviere dein Konto per E-Mail"
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

		// Cache user plan in session for navbar/entitlements
		if us, err := models.GetOrCreateUserSettings(database.GetDB(), user.ID); err == nil && us != nil {
			if us.Plan == "" {
				session.SetSessionValue(c, "user_plan", "free")
			} else {
				session.SetSessionValue(c, "user_plan", us.Plan)
			}
		}

		err = sess.Save()
		if err != nil {
			fm["message"] = fmt.Sprintf("something went wrong: %s", err)

			return flash.WithError(c, fm).Redirect("/login")
		}

		database.GetDB().Model(&user).UpdateColumn("last_login_at", time.Now())

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

		// Validate matching password confirmation
		password := c.FormValue("password")
		passwordConfirm := c.FormValue("password_confirm")
		if password != passwordConfirm {
			fm := fiber.Map{
				"type":    "error",
				"message": "Die Passwörter stimmen nicht überein.",
			}
			return flash.WithError(c, fm).Redirect("/register")
		}

		// Create user after successful captcha validation
		user, err := models.CreateUser(c.FormValue("username"), c.FormValue("email"), password)
		if err != nil {
			fm := fiber.Map{
				"type":    "error",
				"message": fmt.Sprintf("something went wrong: %s", err),
			}

			return flash.WithError(c, fm).Redirect("/register")
		}

		ipv4, ipv6 := GetClientIP(c)
		user.IPv4 = ipv4
		user.IPv6 = ipv6

		// Generate activation token
		if err := user.GenerateActivationToken(); err != nil {
			return flash.WithError(c, fiber.Map{"type": "error", "message": "Fehler beim Generieren des Aktivierungstokens"}).Redirect("/register")
		}
		// Save user with token
		err = database.GetDB().Create(&user).Error
		if err != nil {
			fm := fiber.Map{
				"type":    "error",
				"message": fmt.Sprintf("something went wrong: %s", err),
			}

			return flash.WithError(c, fm).Redirect("/register")
		}

		// Update statistics
		go statistics.UpdateStatisticsCache()
		// Send activation email
		domain := env.GetEnv("PUBLIC_DOMAIN", "")
		activationURL := fmt.Sprintf("%s/activate?token=%s", domain, user.ActivationToken)
		rec := httptest.NewRecorder()
		templ.Handler(email_views.ActivationEmail(user.Email, templ.SafeURL(activationURL), user.ActivationToken)).ServeHTTP(rec, &http.Request{})
		body := rec.Body.String()
		if err := mail.SendMail(user.Email, "Aktivierungslink PIXELFOX.cc", body); err != nil {
			log.Printf("Activation email error: %v", err)
		}
		// Flash success and redirect
		fm := fiber.Map{"type": "success", "message": "Registrierung erfolgreich! Bitte prüfe dein Postfach für den Aktivierungslink."}
		return flash.WithSuccess(c, fm).Redirect("/activate")
	}

	return handler(c)
}

// HandleAuthActivate handles activation form display and token submission
func HandleAuthActivate(c *fiber.Ctx) error {
	fromProtected := c.Locals(FROM_PROTECTED).(bool)
	csrfToken := c.Locals("csrf").(string)
	// determine token from form or query
	var token string
	if c.Method() == fiber.MethodPost {
		token = c.FormValue("token")
	} else {
		token = c.Query("token", "")
	}
	if token == "" {
		// render activation form
		aindex := auth_views.ActivateIndex(fromProtected, csrfToken)
		activate := auth_views.Activate(" | Aktivieren", fromProtected, false, flash.Get(c), "", aindex, false)
		return adaptor.HTTPHandler(templ.Handler(activate))(c)
	}
	// activation logic
	var user models.User
	db := database.GetDB()
	if err := db.Where("activation_token = ?", token).First(&user).Error; err != nil {
		return flash.WithError(c, fiber.Map{"type": "error", "message": "Ungültiger Aktivierungslink."}).Redirect("/activate")
	}
	user.Status = models.STATUS_ACTIVE
	user.ActivationToken = ""
	user.ActivationSentAt = nil
	if err := db.Save(&user).Error; err != nil {
		return flash.WithError(c, fiber.Map{"type": "error", "message": "Aktivierung fehlgeschlagen."}).Redirect("/activate")
	}
	fm := fiber.Map{"type": "success", "message": "Konto aktiviert! Du kannst dich jetzt anmelden."}
	return flash.WithSuccess(c, fm).Redirect("/login")
}
