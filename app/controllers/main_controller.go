package controllers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ManuelReschke/PixelFox/internal/pkg/env"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/statistics"
	"github.com/ManuelReschke/PixelFox/views"
	pages "github.com/ManuelReschke/PixelFox/views/pages"
	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	fiberlog "github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/google/uuid"
	"github.com/sujit-baniya/flash"
)

const (
	FROM_PROTECTED     string = "from_protected"
	DEFAULT_UPLOAD_DIR string = "uploads"
)

func HandleStart(c *fiber.Ctx) error {
	fromProtected := getFromProtected(c)
	csrfToken := c.Locals("csrf").(string)

	stats := statistics.GetStatisticsData()

	hindex := views.HomeIndex(fromProtected, csrfToken, stats)
	home := views.Home("", fromProtected, false, flash.Get(c), hindex)

	handler := adaptor.HTTPHandler(templ.Handler(home))

	return handler(c)
}

func HandleUpload(c *fiber.Ctx) error {
	if !getFromProtected(c) {
		return c.Status(fiber.StatusUnauthorized).SendString("Unauthorized")
	}

	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(fmt.Sprintf("Irgendwas ist schief gelaufen: %s", err))
	}

	// Prüfe, ob die Datei ein Bild ist
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
		return c.Status(fiber.StatusBadRequest).SendString("Nur Bildformate werden unterstützt (JPG, PNG, GIF, WEBP, SVG, BMP)")
	}

	// Generiere UUID für das Bild
	imageUUID := uuid.New().String()

	// Erstelle den Verzeichnispfad nach dem Schema /Jahr/Monat/Tag/UUID.fileextension
	now := time.Now()
	relativePath := fmt.Sprintf("%d/%02d/%02d", now.Year(), now.Month(), now.Day())
	fileName := fmt.Sprintf("%s%s", imageUUID, fileExt)

	// Erstelle den vollständigen Pfad
	dirPath := filepath.Join("./"+DEFAULT_UPLOAD_DIR, relativePath)
	savePath := filepath.Join(dirPath, fileName)

	// Stelle sicher, dass das Verzeichnis existiert
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		fiberlog.Error(fmt.Sprintf("Fehler beim Erstellen des Verzeichnisses: %v", err))
		return c.Status(fiber.StatusInternalServerError).SendString("Fehler beim Erstellen des Upload-Verzeichnisses")
	}

	fiberlog.Info(fmt.Sprintf("[Upload] file: %s -> %s", file.Filename, savePath))
	if err := c.SaveFile(file, savePath); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("something went wrong: %s", err))
	}

	// ShareLink wird jetzt automatisch im Image-Modell generiert
	image := models.Image{
		UUID:     imageUUID,
		UserID:   c.Locals(USER_ID).(uint),
		FileName: fileName,
		FilePath: dirPath,
		FileSize: file.Size,
		FileType: fileExt,
		Title:    file.Filename, // Verwende den ursprünglichen Dateinamen als Titel
	}

	db := database.GetDB()
	if err := db.Create(&image).Error; err != nil {
		fiberlog.Error(fmt.Sprintf("Fehler beim Speichern des Bildes in der Datenbank: %v", err))

		fm := fiber.Map{
			"type":    "error",
			"message": fmt.Sprintf("Datei konnte nicht gespeichert werden: %s", file.Filename),
		}
		flash.WithError(c, fm)
		redirectPath := fmt.Sprintf("/image/%s", fileName)

		return c.Redirect(redirectPath)
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

		// Verwende die UUID für die Weiterleitung
		redirectPath := fmt.Sprintf("/image/%s", imageUUID)
		c.Set("HX-Redirect", redirectPath)
		return c.SendString(fmt.Sprintf("Datei erfolgreich hochgeladen: %s", file.Filename))
	}

	// Ansonsten leiten wir zur Bildanzeige-Seite weiter
	redirectPath := fmt.Sprintf("/image/%s", imageUUID)
	return c.Redirect(redirectPath)
}

func HandleImageViewer(c *fiber.Ctx) error {
	identifier := c.Params("uuid")
	if identifier == "" {
		return c.Redirect("/")
	}

	// Prüfe, ob der Identifier eine UUID ist (36 Zeichen mit Bindestrichen)
	isUUID := len(identifier) == 36 && strings.Count(identifier, "-") == 4
	// Wenn es eine UUID ist, versuche das Bild in der Datenbank zu finden
	if isUUID {
		// Versuche zuerst, das Bild anhand der UUID zu finden
		db := database.GetDB()
		image, err := models.FindImageByUUID(db, identifier)
		if err != nil {
			fiberlog.Info(fmt.Sprintf("Bild nicht gefunden mit UUID: %s, Fehler: %v", identifier, err))

			return c.Redirect("/")
		}

		// Der FilePath enthält den relativen Pfad innerhalb des uploads-Ordners
		imagePath := fmt.Sprintf("/%s", image.FilePath)

		// Erhöhe den View-Counter
		image.IncrementViewCount(db)

		// Verwende den ursprünglichen Dateinamen (Titel) für die Anzeige, falls vorhanden
		displayName := image.FileName
		if image.Title != "" {
			displayName = image.Title
		}

		// Baue den vollständigen Pfad zum Bild zusammen /uploads/Jahr/Monat/Tag/UUID.ext
		filePathComplete := filepath.Join(imagePath, identifier) + image.FileType
		filePathWithDomain := filepath.Join(env.GetEnv("PUBLIC_DOMAIN", ""), filePathComplete)

		imageViewer := views.ImageViewer(filePathComplete, filePathWithDomain, displayName)
		home := views.Home("", getFromProtected(c), false, flash.Get(c), imageViewer)

		handler := adaptor.HTTPHandler(templ.Handler(home))
		return handler(c)
	}

	//imageViewer := views.ImageViewer("-", "", "No Image Found")
	//home := views.Home("", getFromProtected(c), false, flash.Get(c), imageViewer)
	//
	//handler := adaptor.HTTPHandler(templ.Handler(home))
	return c.SendStatus(404)
}

func HandleNews(c *fiber.Ctx) error {
	page := pages.NewsPage()
	home := views.Home("", getFromProtected(c), false, flash.Get(c), page)

	handler := adaptor.HTTPHandler(templ.Handler(home))

	return handler(c)
}

func HandleAbout(c *fiber.Ctx) error {
	page := views.AboutPage()
	home := views.Home("", getFromProtected(c), false, flash.Get(c), page)

	handler := adaptor.HTTPHandler(templ.Handler(home))

	return handler(c)
}

func HandleContact(c *fiber.Ctx) error {
	page := views.ContactPage()
	home := views.Home("", getFromProtected(c), false, flash.Get(c), page)

	handler := adaptor.HTTPHandler(templ.Handler(home))

	return handler(c)
}

func HandleJobs(c *fiber.Ctx) error {
	page := pages.JobsPage()
	home := views.Home("", getFromProtected(c), false, flash.Get(c), page)

	handler := adaptor.HTTPHandler(templ.Handler(home))

	return handler(c)
}

func HandleDocsAPI(c *fiber.Ctx) error {
	page := views.APIPage()
	home := views.Home("", getFromProtected(c), false, flash.Get(c), page)

	handler := adaptor.HTTPHandler(templ.Handler(home))

	return handler(c)
}
