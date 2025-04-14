package controllers

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/google/uuid"
	"github.com/sujit-baniya/flash"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/imageprocessor"
	"github.com/ManuelReschke/PixelFox/internal/pkg/shortener"
	"github.com/ManuelReschke/PixelFox/internal/pkg/statistics"
	"github.com/ManuelReschke/PixelFox/views"
)

// HandleUpload handles the image upload
func HandleUpload(c *fiber.Ctx) error {
	if !isLoggedIn(c) {
		return c.Status(fiber.StatusUnauthorized).SendString("Unauthorized")
	}

	// Use MultipartForm instead of FormFile for better control
	form, err := c.MultipartForm()
	if err != nil {
		log.Error(fmt.Sprintf("Error parsing multipart form: %v", err))

		fm := fiber.Map{
			"type":    "error",
			"message": fmt.Sprintf("Fehler beim Hochladen: %s", err),
		}

		if c.Get("HX-Request") == "true" {
			return c.Status(fiber.StatusBadRequest).SendString(fmt.Sprintf("Fehler beim Hochladen: %s", err))
		}
		return flash.WithError(c, fm).Redirect("/")
	}
	defer form.RemoveAll() // Important: Clean up temporary files

	files := form.File["file"]
	if len(files) == 0 {
		fm := fiber.Map{
			"type":    "error",
			"message": "Keine Datei hochgeladen",
		}

		if c.Get("HX-Request") == "true" {
			return c.Status(fiber.StatusBadRequest).SendString("Keine Datei hochgeladen")
		}
		return flash.WithError(c, fm).Redirect("/")
	}
	file := files[0]

	// Check if the file is an image
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
		fm := fiber.Map{
			"type":    "error",
			"message": "Nur Bildformate werden unterstu00fctzt (JPG, PNG, GIF, WEBP, AVIF, SVG, BMP)",
		}

		if c.Get("HX-Request") == "true" {
			return c.Status(fiber.StatusBadRequest).SendString("Nur Bildformate werden unterstu00fctzt (JPG, PNG, GIF, WEBP, AVIF, SVG, BMP)")
		}
		return flash.WithError(c, fm).Redirect("/")
	}

	// Open the file to get its content
	src, err := file.Open()
	if err != nil {
		log.Error(fmt.Sprintf("Error opening uploaded file: %v", err))

		fm := fiber.Map{
			"type":    "error",
			"message": fmt.Sprintf("Fehler beim u00d6ffnen der Datei: %s", err),
		}

		if c.Get("HX-Request") == "true" {
			return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Fehler beim u00d6ffnen der Datei: %s", err))
		}
		return flash.WithError(c, fm).Redirect("/")
	}
	defer src.Close()

	// Generate UUID for the image
	imageUUID := uuid.New().String()

	// Create the directory path according to the scheme /Year/Month/Day/UUID.fileextension
	now := time.Now()
	relativePath := fmt.Sprintf("%d/%02d/%02d", now.Year(), now.Month(), now.Day())
	fileName := fmt.Sprintf("%s%s", imageUUID, fileExt)

	// Create the full path for the original image in the new structure
	originalDirPath := filepath.Join("./"+imageprocessor.OriginalDir, relativePath)
	originalSavePath := filepath.Join(originalDirPath, fileName)

	// Make sure the directory exists
	if err := os.MkdirAll(originalDirPath, 0755); err != nil {
		log.Error(fmt.Sprintf("Error creating directory: %v", err))

		fm := fiber.Map{
			"type":    "error",
			"message": "Fehler beim Erstellen des Upload-Verzeichnisses",
		}

		if c.Get("HX-Request") == "true" {
			return c.Status(fiber.StatusInternalServerError).SendString("Fehler beim Erstellen des Upload-Verzeichnisses")
		}
		return flash.WithError(c, fm).Redirect("/")
	}

	log.Info(fmt.Sprintf("[Upload] file: %s -> %s", file.Filename, originalSavePath))

	// Create the destination file
	dst, err := os.Create(originalSavePath)
	if err != nil {
		log.Error(fmt.Sprintf("Error creating target file: %v", err))

		fm := fiber.Map{
			"type":    "error",
			"message": fmt.Sprintf("Fehler beim Erstellen der Zieldatei: %s", err),
		}

		if c.Get("HX-Request") == "true" {
			return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Fehler beim Erstellen der Zieldatei: %s", err))
		}
		return flash.WithError(c, fm).Redirect("/")
	}
	defer dst.Close()

	// Copy the file in blocks to reduce memory usage
	buffer := make([]byte, 1024*1024) // 1MB Buffer
	if _, err = io.CopyBuffer(dst, src, buffer); err != nil {
		log.Error(fmt.Sprintf("Error copying file: %v", err))

		fm := fiber.Map{
			"type":    "error",
			"message": fmt.Sprintf("Fehler beim Speichern der Datei: %s", err),
		}

		if c.Get("HX-Request") == "true" {
			return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Fehler beim Speichern der Datei: %s", err))
		}
		return flash.WithError(c, fm).Redirect("/")
	}

	image := models.Image{
		UUID:     imageUUID,
		UserID:   c.Locals("user_id").(uint),
		Filename: fileName,
		FilePath: originalDirPath, // Save the new path in the database
		Filesize: file.Size,
		FileType: fileExt,
		Title:    file.Filename,
	}

	db := database.GetDB()
	if err := db.Create(&image).Error; err != nil {
		log.Error(fmt.Sprintf("Error saving image to database: %v", err))

		// Clean up the file if database insertion fails
		os.Remove(originalSavePath)

		fm := fiber.Map{
			"type":    "error",
			"message": fmt.Sprintf("Datei konnte nicht gespeichert werden: %s", file.Filename),
		}

		if c.Get("HX-Request") == "true" {
			return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Fehler beim Speichern: %s", err))
		}

		return flash.WithError(c, fm).Redirect("/")
	}

	// Start image processing asynchronously with a semaphore to limit concurrent processing
	go func() {
		log.Info(fmt.Sprintf("[Upload] Starting asynchronous image processing for %s", image.UUID))
		if err := imageprocessor.ProcessImage(&image, originalSavePath); err != nil {
			log.Error(fmt.Sprintf("Error during image processing: %v", err))
		}
	}()

	// Update statistics after upload
	go statistics.UpdateStatisticsCache()

	// If the request came from HTMX, return a redirect
	if c.Get("HX-Request") == "true" {
		fm := fiber.Map{
			"type":    "success",
			"message": fmt.Sprintf("Datei erfolgreich hochgeladen: %s", file.Filename),
		}

		redirectPath := fmt.Sprintf("/image/%s", imageUUID)
		c.Set("HX-Redirect", redirectPath)
		return c.SendString(fmt.Sprintf("Datei erfolgreich hochgeladen: %s", file.Filename))
	}

	// Otherwise, redirect to the image view page
	redirectPath := fmt.Sprintf("/image/%s", imageUUID)

	fm := fiber.Map{
		"type":    "success",
		"message": fmt.Sprintf("Datei erfolgreich hochgeladen: %s", file.Filename),
	}
	return flash.WithSuccess(c, fm).Redirect(redirectPath)
}

// ImageView zeigt die Bildansicht an
func ImageView(c *fiber.Ctx) error {
	// UUID aus der URL holen
	uuid := c.Params("uuid")
	if uuid == "" {
		return c.Redirect("/")
	}

	// Bild aus der Datenbank holen
	db := database.GetDB()
	var image models.Image
	result := db.Where("uuid = ?", uuid).First(&image)
	if result.Error != nil {
		return c.Redirect("/")
	}

	// Benutzer laden
	db.Model(&image).Association("User").Find(&image.User)

	// Varianten laden
	db.Model(&image).Association("Variants").Find(&image.Variants)

	// Bild-Informationen vorbereiten
	var imageInfo views.ImageInfoStruct

	imageInfo.UUID = image.UUID
	imageInfo.Filename = image.Filename
	imageInfo.OriginalPath = "/image/serve/" + image.UUID

	// Prüfe, ob WebP-Variante vorhanden ist
	if image.HasWebP() {
		imageInfo.WebPPath = imageprocessor.GetImagePathWithSize(&image, "webp", "")
		imageInfo.HasWebP = true
	}

	// Prüfe, ob AVIF-Variante vorhanden ist
	if image.HasAVIF() {
		imageInfo.AVIFPath = imageprocessor.GetImagePathWithSize(&image, "avif", "")
		imageInfo.HasAVIF = true
	}

	// Prüfe, ob kleine Thumbnail-Variante vorhanden ist
	if image.HasThumbnailSmall() {
		imageInfo.ThumbnailSmall = imageprocessor.GetImagePathWithSize(&image, "webp", "small")
		imageInfo.HasThumbnailS = true
	}

	// Prüfe, ob mittlere Thumbnail-Variante vorhanden ist
	if image.HasThumbnailMedium() {
		imageInfo.ThumbnailMedium = imageprocessor.GetImagePathWithSize(&image, "webp", "medium")
		imageInfo.HasThumbnailM = true
	}

	// Template rendern
	component := views.ImageView(image, imageInfo)
	return adaptor.HTTPHandler(templ.Handler(component))(c)
}

// HandleImageViewer ist der Handler für die Bildansicht
func HandleImageViewer(c *fiber.Ctx) error {
	return ImageView(c)
}

// HandleImageView handles the image view page
func HandleImageView(c *fiber.Ctx) error {
	uuid := c.Params("uuid")
	if uuid == "" {
		return c.Redirect("/")
	}

	// Bild aus der Datenbank holen
	db := database.GetDB()
	var image models.Image
	result := db.Where("uuid = ?", uuid).First(&image)
	if result.Error != nil {
		log.Error("Bild nicht gefunden: " + result.Error.Error())
		return c.Redirect("/")
	}

	// Benutzer laden
	db.Model(&image).Association("User").Find(&image.User)

	// Varianten laden
	db.Model(&image).Association("Variants").Find(&image.Variants)

	// Bild-Informationen vorbereiten
	var imageInfo views.ImageInfoStruct

	imageInfo.UUID = image.UUID
	imageInfo.Filename = image.Filename
	imageInfo.OriginalPath = "/image/serve/" + image.UUID

	// Prüfe, ob WebP-Variante vorhanden ist
	if image.HasWebP() {
		imageInfo.WebPPath = imageprocessor.GetImagePathWithSize(&image, "webp", "")
		imageInfo.HasWebP = true
	}

	// Prüfe, ob AVIF-Variante vorhanden ist
	if image.HasAVIF() {
		imageInfo.AVIFPath = imageprocessor.GetImagePathWithSize(&image, "avif", "")
		imageInfo.HasAVIF = true
	}

	// Prüfe, ob kleine Thumbnail-Variante vorhanden ist
	if image.HasThumbnailSmall() {
		imageInfo.ThumbnailSmall = imageprocessor.GetImagePathWithSize(&image, "webp", "small")
		imageInfo.HasThumbnailS = true
	}

	// Prüfe, ob mittlere Thumbnail-Variante vorhanden ist
	if image.HasThumbnailMedium() {
		imageInfo.ThumbnailMedium = imageprocessor.GetImagePathWithSize(&image, "webp", "medium")
		imageInfo.HasThumbnailM = true
	}

	// Aufrufzähler erhöhen
	image.IncrementViewCount(db)

	// Template rendern
	component := views.ImageView(image, imageInfo)
	return adaptor.HTTPHandler(templ.Handler(component))(c)
}

// HandleShareLink handles the short URL for images
func HandleShareLink(c *fiber.Ctx) error {
	shareLink := c.Params("sharelink")
	if shareLink == "" {
		return c.Redirect("/")
	}

	// Get image ID from share link
	imageID := shortener.DecodeID(shareLink)

	// Get image from database
	db := database.GetDB()
	var image models.Image
	result := db.Where("id = ?", imageID).First(&image)
	if result.Error != nil {
		log.Error(fmt.Sprintf("Error getting image: %v", result.Error))
		return c.Redirect("/")
	}

	// Redirect to full image URL
	return c.Redirect(fmt.Sprintf("/image/%s", image.UUID))
}

// HandleImageDownload handles the image download
func HandleImageDownload(c *fiber.Ctx) error {
	uuid := c.Params("uuid")
	if uuid == "" {
		return c.Redirect("/")
	}

	// Get image from database
	db := database.GetDB()
	var image models.Image
	result := db.Preload("Variants").Where("uuid = ?", uuid).First(&image)
	if result.Error != nil {
		log.Error(fmt.Sprintf("Error getting image: %v", result.Error))
		return c.Redirect("/")
	}

	// Increment download count
	image.Downloads++
	db.Save(&image)

	// Get original image path
	originalPath, err := imageprocessor.GetOriginalPath(&image)
	if err != nil {
		log.Error(fmt.Sprintf("Error getting original path: %v", err))
		return c.Redirect("/")
	}

	// Return the file
	return c.Download(originalPath, image.Filename)
}

// HandleImageServe serves the image file
func HandleImageServe(c *fiber.Ctx) error {
	uuid := c.Params("uuid")
	if uuid == "" {
		return c.SendStatus(fiber.StatusNotFound)
	}

	// Get image from database
	db := database.GetDB()
	var image models.Image
	result := db.Preload("Variants").Where("uuid = ?", uuid).First(&image)
	if result.Error != nil {
		log.Error(fmt.Sprintf("Error getting image: %v", result.Error))
		return c.SendStatus(fiber.StatusNotFound)
	}

	// Check if client supports WebP
	acceptHeader := c.Get("Accept")
	supportsWebP := strings.Contains(acceptHeader, "image/webp")

	// Check if client supports AVIF
	supportsAVIF := strings.Contains(acceptHeader, "image/avif")

	// Determine which format to serve
	var imagePath string
	var err error

	// Try to serve the best format based on client support
	if supportsAVIF && imageprocessor.HasAVIF(&image) {
		imagePath, err = imageprocessor.GetAVIFPath(&image)
	} else if supportsWebP && imageprocessor.HasWebP(&image) {
		imagePath, err = imageprocessor.GetWebPPath(&image)
	} else {
		// Fallback to original
		imagePath, err = imageprocessor.GetOriginalPath(&image)
	}

	if err != nil {
		log.Error(fmt.Sprintf("Error getting image path: %v", err))
		return c.SendStatus(fiber.StatusNotFound)
	}

	// Set cache headers
	c.Set("Cache-Control", "public, max-age=31536000")
	c.Set("Expires", "31536000")

	// Serve the file
	return c.SendFile(imagePath)
}

// HandleThumbnailServe serves the thumbnail
func HandleThumbnailServe(c *fiber.Ctx) error {
	uuid := c.Params("uuid")
	size := c.Params("size")

	if uuid == "" || (size != "small" && size != "medium") {
		return c.SendStatus(fiber.StatusNotFound)
	}

	// Get image from database
	db := database.GetDB()
	var image models.Image
	result := db.Preload("Variants").Where("uuid = ?", uuid).First(&image)
	if result.Error != nil {
		log.Error(fmt.Sprintf("Error getting image: %v", result.Error))
		return c.SendStatus(fiber.StatusNotFound)
	}

	// Check if client supports WebP
	acceptHeader := c.Get("Accept")
	supportsWebP := strings.Contains(acceptHeader, "image/webp")

	// Check if client supports AVIF
	supportsAVIF := strings.Contains(acceptHeader, "image/avif")

	// Determine which format to serve
	var imagePath string
	var err error

	if size == "small" {
		if supportsAVIF && imageprocessor.HasAVIF(&image) && imageprocessor.HasThumbnailSmall(&image) {
			imagePath, err = imageprocessor.GetThumbnailSmallPath(&image)
		} else if supportsWebP && imageprocessor.HasWebP(&image) && imageprocessor.HasThumbnailSmall(&image) {
			imagePath, err = imageprocessor.GetThumbnailSmallPath(&image)
		} else if imageprocessor.HasThumbnailSmall(&image) {
			imagePath, err = imageprocessor.GetThumbnailSmallPath(&image)
		} else {
			// Fallback to original
			imagePath, err = imageprocessor.GetOriginalPath(&image)
		}
	} else { // medium
		if supportsAVIF && imageprocessor.HasAVIF(&image) && imageprocessor.HasThumbnailMedium(&image) {
			imagePath, err = imageprocessor.GetThumbnailMediumPath(&image)
		} else if supportsWebP && imageprocessor.HasWebP(&image) && imageprocessor.HasThumbnailMedium(&image) {
			imagePath, err = imageprocessor.GetThumbnailMediumPath(&image)
		} else if imageprocessor.HasThumbnailMedium(&image) {
			imagePath, err = imageprocessor.GetThumbnailMediumPath(&image)
		} else {
			// Fallback to original
			imagePath, err = imageprocessor.GetOriginalPath(&image)
		}
	}

	if err != nil {
		log.Error(fmt.Sprintf("Error getting thumbnail path: %v", err))
		return c.SendStatus(fiber.StatusNotFound)
	}

	// Set cache headers
	c.Set("Cache-Control", "public, max-age=31536000")
	c.Set("Expires", "31536000")

	// Serve the file
	return c.SendFile(imagePath)
}

// HandleImageProcessingStatus gibt den Status der Bildverarbeitung zurück
func HandleImageProcessingStatus(c *fiber.Ctx) error {
	// UUID aus der URL holen
	uuid := c.Params("uuid")
	if uuid == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "UUID fehlt",
		})
	}

	// Bild aus der Datenbank holen
	db := database.GetDB()
	var image models.Image
	result := db.Where("uuid = ?", uuid).First(&image)
	if result.Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Bild nicht gefunden",
		})
	}

	// Varianten laden
	db.Model(&image).Association("Variants").Find(&image.Variants)

	// Status der Bildverarbeitung ermitteln
	status := imageprocessor.GetImageProcessingStatus(&image)

	return c.JSON(fiber.Map{
		"status": status,
		"image": fiber.Map{
			"uuid":               image.UUID,
			"filename":           image.Filename,
			"hasWebP":            image.HasWebP(),
			"hasAVIF":            image.HasAVIF(),
			"hasThumbnailSmall":  image.HasThumbnailSmall(),
			"hasThumbnailMedium": image.HasThumbnailMedium(),
		},
	})
}
