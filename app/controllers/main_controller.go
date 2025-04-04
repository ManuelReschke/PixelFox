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
	"github.com/ManuelReschke/PixelFox/internal/pkg/imageprocessor"
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

	// Erstelle den vollständigen Pfad für das Original-Bild in der neuen Struktur
	originalDirPath := filepath.Join("./"+imageprocessor.OriginalDir, relativePath)
	originalSavePath := filepath.Join(originalDirPath, fileName)

	// Stelle sicher, dass das Verzeichnis existiert
	if err := os.MkdirAll(originalDirPath, 0755); err != nil {
		fiberlog.Error(fmt.Sprintf("Fehler beim Erstellen des Verzeichnisses: %v", err))
		return c.Status(fiber.StatusInternalServerError).SendString("Fehler beim Erstellen des Upload-Verzeichnisses")
	}

	fiberlog.Info(fmt.Sprintf("[Upload] file: %s -> %s", file.Filename, originalSavePath))
	if err := c.SaveFile(file, originalSavePath); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("something went wrong: %s", err))
	}

	image := models.Image{
		UUID:     imageUUID,
		UserID:   c.Locals(USER_ID).(uint),
		FileName: fileName,
		FilePath: originalDirPath, // Speichere den neuen Pfad in der Datenbank
		FileSize: file.Size,
		FileType: fileExt,
		Title:    file.Filename,
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

	// Starte die Bildverarbeitung asynchron
	go func() {
		fiberlog.Info(fmt.Sprintf("[Upload] Starte asynchrone Bildverarbeitung für %s", image.UUID))
		if err := imageprocessor.ProcessImage(&image, originalSavePath); err != nil {
			fiberlog.Error(fmt.Sprintf("Fehler bei der Bildverarbeitung: %v", err))
		}
	}()

	// Aktualisiere die Statistiken nach dem Upload
	go statistics.UpdateStatisticsCache()

	// Wenn der Request über HTMX kam, geben wir eine Umleitung zurück
	if c.Get("HX-Request") == "true" {
		fm := fiber.Map{
			"type":    "success",
			"message": fmt.Sprintf("Datei erfolgreich hochgeladen: %s", file.Filename),
		}
		flash.WithSuccess(c, fm)

		redirectPath := fmt.Sprintf("/image/%s", imageUUID)
		c.Set("HX-Redirect", redirectPath)
		return c.SendString(fmt.Sprintf("Datei erfolgreich hochgeladen: %s", file.Filename))
	}

	// Ansonsten leiten wir zur Bildanzeige-Seite weiter
	redirectPath := fmt.Sprintf("/image/%s", imageUUID)
	return c.Redirect(redirectPath)
}

func HandleShareLink(c *fiber.Ctx) error {
	sharelink := c.Params("sharelink")
	if sharelink == "" {
		return c.Redirect("/")
	}

	db := database.GetDB()
	var image models.Image
	if err := db.Where("share_link = ?", sharelink).First(&image).Error; err != nil {
		fiberlog.Info(fmt.Sprintf("Bild nicht gefunden mit ShareLink: %s, Fehler: %v", sharelink, err))
		return c.Redirect("/")
	}

	redirectPath := fmt.Sprintf("/image/%s", image.UUID)
	return c.Redirect(redirectPath)
}

func HandleImageViewer(c *fiber.Ctx) error {
	identifier := c.Params("uuid")
	if identifier == "" {
		return c.Redirect("/")
	}

	// Prüfe, ob der Identifier eine UUID ist (36 Zeichen mit Bindestrichen)
	isUUID := len(identifier) == 36 && strings.Count(identifier, "-") == 4
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

		// Generiere die ShareLink-URL mit der Domain und dem neuen /i/-Pfad
		shareURL := filepath.Join(env.GetEnv("PUBLIC_DOMAIN", ""), "/i/", image.ShareLink)

		// Pfade für optimierte Bildformate
		webpPath := ""
		avifPath := ""
		smallThumbPath := ""
		smallThumbWebpPath := ""
		smallThumbAvifPath := ""

		// Wenn optimierte Formate verfügbar sind, generiere die Pfade
		if image.HasWebp {
			webpRelativePath := imageprocessor.GetImagePath(image, "webp", "")
			webpPath = "/" + webpRelativePath
		}

		if image.HasAVIF {
			avifRelativePath := imageprocessor.GetImagePath(image, "avif", "")
			avifPath = "/" + avifRelativePath
		}

		// Wenn Thumbnails verfügbar sind, generiere die Pfade
		if image.HasThumbnails {
			// Pfad zum kleinen Thumbnail (Original-Format)
			smallThumbRelativePath := imageprocessor.GetImagePath(image, "", "small")
			smallThumbPath = "/" + smallThumbRelativePath

			// Pfade zu optimierten Thumbnails
			if image.HasWebp {
				smallThumbWebpRelativePath := imageprocessor.GetImagePath(image, "webp", "small")
				smallThumbWebpPath = "/" + smallThumbWebpRelativePath
			}

			if image.HasAVIF {
				smallThumbAvifRelativePath := imageprocessor.GetImagePath(image, "avif", "small")
				smallThumbAvifPath = "/" + smallThumbAvifRelativePath
			}
		}

		// Verwende das kleine Thumbnail für die Vorschau, falls verfügbar
		previewPath := filePathComplete
		previewWebpPath := webpPath
		previewAvifPath := avifPath

		if image.HasThumbnails {
			// Verwende das kleine Thumbnail für die Vorschau im Viewer
			previewPath = smallThumbPath
			if image.HasWebp {
				previewWebpPath = smallThumbWebpPath
			}
			if image.HasAVIF {
				previewAvifPath = smallThumbAvifPath
			}
		}

		// Open Graph Meta-Tags vorbereiten
		ogImage := ""
		if image.HasThumbnails && image.HasAVIF {
			// Prefer medium AVIF thumbnail for Open Graph
			mediumAvifPath := "/" + imageprocessor.GetImagePath(image, "avif", "medium")
			ogImage = filepath.Join(env.GetEnv("PUBLIC_DOMAIN", ""), mediumAvifPath)
		} else if image.HasThumbnails && image.HasWebp {
			// Fallback to medium WebP thumbnail
			mediumWebpPath := "/" + imageprocessor.GetImagePath(image, "webp", "medium")
			ogImage = filepath.Join(env.GetEnv("PUBLIC_DOMAIN", ""), mediumWebpPath)
		} else {
			// If no thumbnails available, use original
			ogImage = filePathWithDomain
		}

		// Titel und Beschreibung für Open Graph
		ogTitle := fmt.Sprintf("%s - %s", displayName, "PIXELFOX.cc")
		ogDescription := "Bild hochgeladen auf PIXELFOX.cc - Kostenloser Bilderhoster"

		imageViewer := views.ImageViewer(
			previewPath,         // Pfad für die Vorschau (Thumbnail oder Original)
			filePathWithDomain,  // Vollständiger Pfad zum Original für Download
			displayName,         // Anzeigename
			shareURL,            // ShareLink URL
			image.HasWebp,       // Hat WebP?
			image.HasAVIF,       // Hat AVIF?
			previewWebpPath,     // WebP-Pfad für die Vorschau
			previewAvifPath,     // AVIF-Pfad für die Vorschau
			filePathComplete,    // Original-Pfad (für Download)
			image.HasThumbnails, // Hat Thumbnails?
		)

		// Open Graph Meta-Tags an das Home-Template übergeben
		home := views.Home("", getFromProtected(c), false, flash.Get(c), imageViewer, ogImage, ogTitle, ogDescription)

		handler := adaptor.HTTPHandler(templ.Handler(home))

		return handler(c)
	}

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
