package controllers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ManuelReschke/PixelFox/internal/pkg/viewmodel"

	"github.com/ManuelReschke/PixelFox/internal/pkg/env"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/imageprocessor"
	"github.com/ManuelReschke/PixelFox/internal/pkg/session"
	"github.com/ManuelReschke/PixelFox/internal/pkg/statistics"
	"github.com/ManuelReschke/PixelFox/views"
	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	fiberlog "github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/google/uuid"
	"github.com/sujit-baniya/flash"
)

func HandleUpload(c *fiber.Ctx) error {
	if !isLoggedIn(c) {
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
		".avif": true,
		".svg":  true,
		".bmp":  true,
	}

	if !validImageExtensions[fileExt] {
		return c.Status(fiber.StatusBadRequest).SendString("Nur Bildformate werden unterstützt (JPG, PNG, GIF, WEBP, AVIF, SVG, BMP)")
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

		domain := env.GetEnv("PUBLIC_DOMAIN", "")

		// Baue den vollständigen Pfad zum Bild zusammen /uploads/Jahr/Monat/Tag/UUID.ext
		filePathComplete := filepath.Join(imagePath, identifier) + image.FileType
		filePathWithDomain := filepath.Join(domain, filePathComplete)

		// Generiere die ShareLink-URL mit der Domain und dem neuen /i/-Pfad
		shareURL := filepath.Join(domain, "/i/", image.ShareLink)

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
		if image.HasThumbnailSmall {
			// Pfad zum kleinen Thumbnail (Original-Format)
			smallThumbRelativePath := imageprocessor.GetImagePath(image, "", "small")
			smallThumbPath = "/" + smallThumbRelativePath

			if image.HasWebp {
				smallThumbWebpRelativePath := imageprocessor.GetImagePath(image, "webp", "small")
				smallThumbWebpPath = "/" + smallThumbWebpRelativePath
			}

			if image.HasAVIF {
				smallThumbAvifRelativePath := imageprocessor.GetImagePath(image, "avif", "small")
				smallThumbAvifPath = "/" + smallThumbAvifRelativePath
			}
		}

		// Pfade für Medium-Thumbnails
		mediumThumbWebpPath := ""
		mediumThumbAvifPath := ""

		// Wenn Medium-Thumbnails verfügbar sind, generiere die Pfade
		if image.HasThumbnailMedium {
			// Pfad zum Medium-Thumbnail (WebP)
			if image.HasWebp {
				mediumThumbWebpRelativePath := imageprocessor.GetImagePath(image, "webp", "medium")
				mediumThumbWebpPath = "/" + mediumThumbWebpRelativePath
			}

			// Pfad zum Medium-Thumbnail (AVIF)
			if image.HasAVIF {
				mediumThumbAvifRelativePath := imageprocessor.GetImagePath(image, "avif", "medium")
				mediumThumbAvifPath = "/" + mediumThumbAvifRelativePath
			}
		}

		// Verwende das Medium-Thumbnail für die Vorschau im Viewer
		previewPath := filePathComplete
		previewWebpPath := webpPath
		previewAvifPath := avifPath

		if image.HasThumbnailMedium {
			// Setze die Pfade für Medium-Thumbnails, wenn verfügbar
			if image.HasAVIF {
				previewPath = mediumThumbAvifPath
				previewAvifPath = mediumThumbAvifPath
			}
			// WebP-Pfad unabhängig von AVIF setzen
			if image.HasWebp {
				if !image.HasAVIF { // Nur wenn kein AVIF verfügbar ist, den Hauptpfad ändern
					previewPath = mediumThumbWebpPath
				}
				previewWebpPath = mediumThumbWebpPath
			}
		} else if image.HasThumbnailSmall {
			// Fallback auf Small-Thumbnail wenn Medium nicht verfügbar
			if image.HasAVIF {
				previewPath = smallThumbAvifPath
				previewAvifPath = smallThumbAvifPath
			}
			// WebP-Pfad unabhängig von AVIF setzen
			if image.HasWebp {
				if !image.HasAVIF { // Nur wenn kein AVIF verfügbar ist, den Hauptpfad ändern
					previewPath = smallThumbWebpPath
				}
				previewWebpPath = smallThumbWebpPath
			} else if !image.HasAVIF && !image.HasWebp {
				// Nur wenn weder AVIF noch WebP verfügbar sind
				previewPath = smallThumbPath
			}
		}

		// Get paths for optimized versions
		optimizedWebpPath := ""
		optimizedAvifPath := ""
		if image.HasWebp {
			optimizedWebpPath = "/" + imageprocessor.GetImagePath(image, "webp", "")
		}
		if image.HasAVIF {
			optimizedAvifPath = "/" + imageprocessor.GetImagePath(image, "avif", "")
		}

		// Optimierte Version für Direktlinks und Einbettungen (AVIF oder WebP)
		// Hinweis: Diese Variable wird nicht mehr benötigt, da die Optimierung direkt im Template erfolgt
		// durch die Verwendung von hasAVIF, hasWebP, avifPath und webpPath

		// Open Graph Meta-Tags vorbereiten
		ogImage := ""
		if image.HasThumbnailSmall {
			// Verwende Small-Thumbnail für OG-Tags
			if image.HasAVIF {
				ogImage = filepath.Join(domain, smallThumbAvifPath)
			} else if image.HasWebp {
				ogImage = filepath.Join(domain, smallThumbWebpPath)
			}
		} else {
			// If no thumbnails available, use original
			ogImage = filePathWithDomain
		}

		ogTitle := fmt.Sprintf("%s - %s", displayName, "PIXELFOX.cc")
		ogDescription := "Bild hochgeladen auf PIXELFOX.cc - Kostenloser Bilderhoster"

		isAdmin := false
		if isLoggedIn(c) {
			sess, _ := session.GetSessionStore().Get(c)
			isAdmin = sess.Get(USER_IS_ADMIN).(bool)
		}

		// Erstelle das ImageViewModel
		imageModel := viewmodel.Image{
			Domain:             domain,
			PreviewPath:        previewPath,
			FilePathWithDomain: filePathWithDomain,
			DisplayName:        displayName,
			ShareURL:           shareURL,
			HasWebP:            image.HasWebp,
			HasAVIF:            image.HasAVIF,
			PreviewWebPPath:    previewWebpPath,
			PreviewAVIFPath:    previewAvifPath,
			OptimizedWebPPath:  optimizedWebpPath,
			OptimizedAVIFPath:  optimizedAvifPath,
			OriginalPath:       filePathComplete,
			Width:              image.Width,
			Height:             image.Height,
		}

		imageViewer := views.ImageViewer(imageModel)

		ogViewModel := &viewmodel.OpenGraph{
			URL:         shareURL,
			Image:       ogImage,
			ImageAlt:    ogTitle,
			Title:       ogTitle,
			Description: ogDescription,
		}

		home := views.Home("", isLoggedIn(c), false, flash.Get(c), imageViewer, isAdmin, ogViewModel)

		handler := adaptor.HTTPHandler(templ.Handler(home))

		return handler(c)
	}

	return c.SendStatus(404)
}
