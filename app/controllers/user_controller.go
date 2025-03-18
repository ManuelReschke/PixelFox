package controllers

import (
	"github.com/ManuelReschke/PixelFox/internal/pkg/session"
	userviews "github.com/ManuelReschke/PixelFox/views/user"
	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/sujit-baniya/flash"
)

func HandleUserProfile(c *fiber.Ctx) error {
	// Get user information from session
	sess, _ := session.GetSessionStore().Get(c)
	_ = sess.Get(USER_ID) // Using _ to avoid unused variable warning
	username := sess.Get(USER_NAME).(string)

	// Get CSRF token for forms
	csrfToken := c.Locals("csrf").(string)

	// Render the profile page
	profileIndex := userviews.ProfileIndex(username, csrfToken)
	profile := userviews.Profile(
		" | Profil", getFromProtected(c), false, flash.Get(c), username, profileIndex,
	)

	handler := adaptor.HTTPHandler(templ.Handler(profile))

	return handler(c)
}

func HandleUserSettings(c *fiber.Ctx) error {
	// Get user information from session
	sess, _ := session.GetSessionStore().Get(c)
	_ = sess.Get(USER_ID) // Using _ to avoid unused variable warning
	username := sess.Get(USER_NAME).(string)

	// Get CSRF token for forms
	csrfToken := c.Locals("csrf").(string)

	// Render the settings page
	settingsIndex := userviews.SettingsIndex(username, csrfToken)
	settings := userviews.Settings(
		" | Einstellungen", getFromProtected(c), false, flash.Get(c), username, settingsIndex,
	)

	handler := adaptor.HTTPHandler(templ.Handler(settings))

	return handler(c)
}
