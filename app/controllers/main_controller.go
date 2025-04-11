package controllers

import (
	"github.com/ManuelReschke/PixelFox/internal/pkg/session"
	"github.com/ManuelReschke/PixelFox/internal/pkg/statistics"
	"github.com/ManuelReschke/PixelFox/views"
	pages "github.com/ManuelReschke/PixelFox/views/pages"
	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/sujit-baniya/flash"
)

const (
	FROM_PROTECTED string = "from_protected"
)

func HandleStart(c *fiber.Ctx) error {
	fromProtected := isLoggedIn(c)
	csrfToken := c.Locals("csrf").(string)

	// Überprüfe, ob der Benutzer ein Admin ist
	isAdmin := false
	if fromProtected {
		sess, _ := session.GetSessionStore().Get(c)
		isAdmin = sess.Get(USER_IS_ADMIN).(bool)
	}

	stats := statistics.GetStatisticsData()

	page := views.HomeIndex(fromProtected, csrfToken, stats)
	home := views.Home("", fromProtected, false, flash.Get(c), page, isAdmin, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))

	return handler(c)
}

func HandleNews(c *fiber.Ctx) error {
	// Überprüfe, ob der Benutzer ein Admin ist
	isAdmin := false
	if isLoggedIn(c) {
		sess, _ := session.GetSessionStore().Get(c)
		isAdmin = sess.Get(USER_IS_ADMIN).(bool)
	}

	page := pages.NewsPage()
	home := views.Home("", isLoggedIn(c), false, flash.Get(c), page, isAdmin, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))

	return handler(c)
}

func HandleAbout(c *fiber.Ctx) error {
	// Überprüfe, ob der Benutzer ein Admin ist
	isAdmin := false
	if isLoggedIn(c) {
		sess, _ := session.GetSessionStore().Get(c)
		isAdmin = sess.Get(USER_IS_ADMIN).(bool)
	}

	page := views.AboutPage()
	home := views.Home("", isLoggedIn(c), false, flash.Get(c), page, isAdmin, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))

	return handler(c)
}

func HandleContact(c *fiber.Ctx) error {
	// Überprüfe, ob der Benutzer ein Admin ist
	isAdmin := false
	if isLoggedIn(c) {
		sess, _ := session.GetSessionStore().Get(c)
		isAdmin = sess.Get(USER_IS_ADMIN).(bool)
	}

	page := views.ContactPage()
	home := views.Home("", isLoggedIn(c), false, flash.Get(c), page, isAdmin, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))

	return handler(c)
}

func HandleJobs(c *fiber.Ctx) error {
	// Überprüfe, ob der Benutzer ein Admin ist
	isAdmin := false
	if isLoggedIn(c) {
		sess, _ := session.GetSessionStore().Get(c)
		isAdmin = sess.Get(USER_IS_ADMIN).(bool)
	}

	page := pages.JobsPage()
	home := views.Home("", isLoggedIn(c), false, flash.Get(c), page, isAdmin, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))

	return handler(c)
}

func HandleDocsAPI(c *fiber.Ctx) error {
	// Überprüfe, ob der Benutzer ein Admin ist
	isAdmin := false
	if isLoggedIn(c) {
		sess, _ := session.GetSessionStore().Get(c)
		isAdmin = sess.Get(USER_IS_ADMIN).(bool)
	}

	page := views.APIPage()
	home := views.Home("", isLoggedIn(c), false, flash.Get(c), page, isAdmin, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))

	return handler(c)
}
