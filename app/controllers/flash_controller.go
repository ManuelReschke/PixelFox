package controllers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/sujit-baniya/flash"
)

// HandleFlashUploadRateLimit sets a flash error and redirects to home
func HandleFlashUploadRateLimit(c *fiber.Ctx) error {
	fm := fiber.Map{
		"type":    "error",
		"message": "Upload-Limit erreicht. Bitte warte kurz und versuche es erneut.",
	}
	flash.WithError(c, fm)
	return c.Redirect("/", fiber.StatusSeeOther)
}

// HandleFlashUploadDuplicate sets an info flash and redirects to the given view URL
// Query: ?view=/i/<share>
func HandleFlashUploadDuplicate(c *fiber.Ctx) error {
	view := c.Query("view", "/")
	fm := fiber.Map{
		"type":    "info",
		"message": "Du hast dieses Bild bereits hochgeladen.",
	}
	flash.WithInfo(c, fm)
	return c.Redirect(view, fiber.StatusSeeOther)
}

// HandleFlashUploadError shows a generic upload error from query string
// Query: ?msg=...
func HandleFlashUploadError(c *fiber.Ctx) error {
	msg := c.Query("msg", "Fehler beim Hochladen. Bitte versuche es erneut.")
	if len(msg) > 300 {
		msg = msg[:300]
	}
	fm := fiber.Map{
		"type":    "error",
		"message": msg,
	}
	flash.WithError(c, fm)
	return c.Redirect("/", fiber.StatusSeeOther)
}

// HandleFlashUploadTooLarge shows a size error and redirects home
func HandleFlashUploadTooLarge(c *fiber.Ctx) error {
	fm := fiber.Map{
		"type":    "error",
		"message": "Die Datei ist zu groß. Bitte wähle ein kleineres Bild.",
	}
	flash.WithError(c, fm)
	return c.Redirect("/", fiber.StatusSeeOther)
}

// HandleFlashUploadUnsupportedType shows an unsupported type error and redirects home
func HandleFlashUploadUnsupportedType(c *fiber.Ctx) error {
	fm := fiber.Map{
		"type":    "error",
		"message": "Dateityp nicht unterstützt. Erlaubt: JPG, JPEG, PNG, GIF, WEBP, AVIF, BMP",
	}
	flash.WithError(c, fm)
	return c.Redirect("/", fiber.StatusSeeOther)
}
