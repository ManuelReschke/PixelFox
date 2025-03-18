package controllers

import (
	"fmt"
	"path/filepath"

	"github.com/ManuelReschke/PixelFox/internal/pkg/statistics"
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

	stats := statistics.GetStatisticsData()

	hindex := views.HomeIndex(fromProtected, csrfToken, stats)
	home := views.Home("", fromProtected, false, flash.Get(c), hindex)

	handler := adaptor.HTTPHandler(templ.Handler(home))

	return handler(c)
}

func HandleUpload(c *fiber.Ctx) error {
	if !c.Locals(FROM_PROTECTED).(bool) {
		return c.Status(fiber.StatusUnauthorized).SendString("Unauthorized")
	}

	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(fmt.Sprintf("Irgendwas ist schief gelaufen: %s", err))
	}

	// Pru00fcfe, ob die Datei ein Bild ist
	fileExt := filepath.Ext(file.Filename)
	validImageExtensions := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
		".webp": true,
		".svg":  true,
		".bmp":  true,
	}

	if !validImageExtensions[fileExt] {
		return c.Status(fiber.StatusBadRequest).SendString("Nur Bildformate werden unterstu00fctzt (JPG, PNG, GIF, WEBP, SVG, BMP)")
	}

	log.Infof("[Upload] file: %s", file.Filename)
	savePath := filepath.Join("./uploads", file.Filename)

	if err := c.SaveFile(file, savePath); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("something went wrong: %s", err))
	}

	// Aktualisiere die Statistiken nach dem Upload
	go statistics.UpdateStatisticsCache()

	// Wenn der Request über HTMX kam, geben wir eine Umleitung zurück
	if c.Get("HX-Request") == "true" {
		fm := fiber.Map{
			"type":    "success",
			"message": fmt.Sprintf("Datei erfolgreich hochgeladen: %s", file.Filename),
		}
		flash.WithSuccess(c, fm)
		c.Set("HX-Redirect", fmt.Sprintf("/image/%s", file.Filename))
		return c.SendString(fmt.Sprintf("Datei erfolgreich hochgeladen: %s", file.Filename))
	}

	// Ansonsten leiten wir zur Bildanzeige-Seite weiter
	return c.Redirect(fmt.Sprintf("/image/%s", file.Filename))
}

func HandleImageViewer(c *fiber.Ctx) error {
	imageFilename := c.Params("filename")
	if imageFilename == "" {
		return c.Redirect("/")
	}

	var fromProtected bool
	if protectedValue := c.Locals(FROM_PROTECTED); protectedValue != nil {
		fromProtected = protectedValue.(bool)
	}

	imagePath := fmt.Sprintf("/uploads/%s", imageFilename)

	stats := statistics.GetStatisticsData()

	imageViewer := views.ImageViewer(imagePath, imageFilename, stats)
	home := views.Home("", fromProtected, false, flash.Get(c), imageViewer)

	handler := adaptor.HTTPHandler(templ.Handler(home))

	return handler(c)
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
