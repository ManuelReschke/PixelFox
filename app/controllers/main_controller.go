package controllers

import (
	"fmt"
	"path/filepath"

	"github.com/ManuelReschke/PixelFox/views"
	pages "github.com/ManuelReschke/PixelFox/views/pages"
	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/sujit-baniya/flash"
)

const (
	FROM_PROTECTED string = "from_protected"
)

func HandleStart(c *fiber.Ctx) error {
	fromProtected := c.Locals(FROM_PROTECTED).(bool)
	csrfToken := c.Locals("csrf").(string)

	hindex := views.HomeIndex(fromProtected, csrfToken)
	home := views.Home("", fromProtected, false, flash.Get(c), hindex)

	handler := adaptor.HTTPHandler(templ.Handler(home))

	return handler(c)
}

func HandleUpload(c *fiber.Ctx) error {
	//fromProtected := c.Locals(FROM_PROTECTED).(bool)

	file, err := c.FormFile("file")
	if err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": fmt.Sprintf("something went wrong: %s", err),
		}

		return flash.WithError(c, fm).Redirect("/")
		//return c.Status(fiber.StatusBadRequest).SendString("Fehler beim Hochladen der Datei.")
	}

	log.Infof("[Upload] file: %s", file.Filename)
	savePath := filepath.Join("./uploads", file.Filename)

	if err := c.SaveFile(file, savePath); err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": fmt.Sprintf("something went wrong: %s", err),
		}

		return flash.WithError(c, fm).Redirect("/")
		//return c.Status(fiber.StatusInternalServerError).SendString("Fehler beim Speichern der Datei.")
	}

	//fm := fiber.Map{
	//	"type":    "success",
	//	"message": fmt.Sprintf("Datei erfolgreich hochgeladen: %s", file.Filename),
	//}
	//
	//return flash.WithSuccess(c, fm).Redirect("/")

	return c.SendString(fmt.Sprintf("Datei erfolgreich hochgeladen: %s", file.Filename))
}

func HandleNews(c *fiber.Ctx) error {
	page := pages.NewsPage()
	home := views.Home("", false, false, flash.Get(c), page)

	handler := adaptor.HTTPHandler(templ.Handler(home))

	return handler(c)
}

func HandleAbout(c *fiber.Ctx) error {
	page := views.AboutPage()
	home := views.Home("", false, false, flash.Get(c), page)

	handler := adaptor.HTTPHandler(templ.Handler(home))

	return handler(c)
}

func HandleContact(c *fiber.Ctx) error {
	page := views.ContactPage()
	home := views.Home("", false, false, flash.Get(c), page)

	handler := adaptor.HTTPHandler(templ.Handler(home))

	return handler(c)
}

func HandleJobs(c *fiber.Ctx) error {
	page := pages.JobsPage()
	home := views.Home("", false, false, flash.Get(c), page)

	handler := adaptor.HTTPHandler(templ.Handler(home))

	return handler(c)
}

func HandleDocsAPI(c *fiber.Ctx) error {
	page := views.APIPage()
	home := views.Home("", false, false, flash.Get(c), page)

	handler := adaptor.HTTPHandler(templ.Handler(home))

	return handler(c)
}
