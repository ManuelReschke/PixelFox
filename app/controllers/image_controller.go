package controllers

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	fiberlog "github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/google/uuid"
	"github.com/sujit-baniya/flash"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
	"github.com/ManuelReschke/PixelFox/internal/pkg/imageprocessor"
	"github.com/ManuelReschke/PixelFox/internal/pkg/session"
	"github.com/ManuelReschke/PixelFox/internal/pkg/statistics"
	"github.com/ManuelReschke/PixelFox/internal/pkg/viewmodel"
	"github.com/ManuelReschke/PixelFox/views"
)

func HandleUpload(c *fiber.Ctx) error {
	if !isLoggedIn(c) {
		return c.Status(fiber.StatusUnauthorized).SendString("Unauthorized")
	}

	// Check if image upload is globally enabled
	if !models.GetAppSettings().IsImageUploadEnabled() {
		fm := fiber.Map{
			"type":    "error",
			"message": "Der Bild-Upload ist derzeit deaktiviert",
		}
		flash.WithError(c, fm)

		if c.Get("HX-Request") == "true" {
			return c.Status(fiber.StatusForbidden).SendString("Der Bild-Upload ist derzeit deaktiviert")
		}
		return c.Redirect("/")
	}

	// Use MultipartForm instead of FormFile for better control
	form, err := c.MultipartForm()
	if err != nil {
		fiberlog.Error(fmt.Sprintf("Error parsing multipart form: %v", err))

		fm := fiber.Map{
			"type":    "error",
			"message": fmt.Sprintf("Fehler beim Hochladen: %s", err),
		}
		flash.WithError(c, fm)

		if c.Get("HX-Request") == "true" {
			return c.Status(fiber.StatusBadRequest).SendString(fmt.Sprintf("Fehler beim Hochladen: %s", err))
		}
		return c.Redirect("/")
	}
	defer form.RemoveAll() // Important: Clean up temporary files

	files := form.File["file"]
	if len(files) == 0 {
		fm := fiber.Map{
			"type":    "error",
			"message": "Keine Datei hochgeladen",
		}
		flash.WithError(c, fm)

		if c.Get("HX-Request") == "true" {
			return c.Status(fiber.StatusBadRequest).SendString("Keine Datei hochgeladen")
		}
		return c.Redirect("/")
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
			"message": "Nur Bildformate werden unterstützt (JPG, PNG, GIF, WEBP, AVIF, SVG, BMP)",
		}
		flash.WithError(c, fm)

		if c.Get("HX-Request") == "true" {
			return c.Status(fiber.StatusBadRequest).SendString("Nur Bildformate werden unterstützt (JPG, PNG, GIF, WEBP, AVIF, SVG, BMP)")
		}
		return c.Redirect("/")
	}

	// Open the file to get its content
	src, err := file.Open()
	if err != nil {
		fiberlog.Error(fmt.Sprintf("Error opening uploaded file: %v", err))

		fm := fiber.Map{
			"type":    "error",
			"message": fmt.Sprintf("Fehler beim Öffnen der Datei: %s", err),
		}
		flash.WithError(c, fm)

		if c.Get("HX-Request") == "true" {
			return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Fehler beim Öffnen der Datei: %s", err))
		}
		return c.Redirect("/")
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
		fiberlog.Error(fmt.Sprintf("Error creating directory: %v", err))

		fm := fiber.Map{
			"type":    "error",
			"message": "Fehler beim Erstellen des Upload-Verzeichnisses",
		}
		flash.WithError(c, fm)

		if c.Get("HX-Request") == "true" {
			return c.Status(fiber.StatusInternalServerError).SendString("Fehler beim Erstellen des Upload-Verzeichnisses")
		}
		return c.Redirect("/")
	}

	fiberlog.Info(fmt.Sprintf("[Upload] file: %s -> %s", file.Filename, originalSavePath))

	// Create the destination file
	dst, err := os.Create(originalSavePath)
	if err != nil {
		fiberlog.Error(fmt.Sprintf("Error creating target file: %v", err))

		fm := fiber.Map{
			"type":    "error",
			"message": fmt.Sprintf("Fehler beim Erstellen der Zieldatei: %s", err),
		}
		flash.WithError(c, fm)

		if c.Get("HX-Request") == "true" {
			return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Fehler beim Erstellen der Zieldatei: %s", err))
		}
		return c.Redirect("/")
	}
	defer dst.Close()

	// Copy the file in blocks to reduce memory usage
	buffer := make([]byte, 1024*1024) // 1MB Buffer
	if _, err = io.CopyBuffer(dst, src, buffer); err != nil {
		fiberlog.Error(fmt.Sprintf("Error copying file: %v", err))

		fm := fiber.Map{
			"type":    "error",
			"message": fmt.Sprintf("Fehler beim Speichern der Datei: %s", err),
		}
		flash.WithError(c, fm)

		if c.Get("HX-Request") == "true" {
			return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Fehler beim Speichern der Datei: %s", err))
		}
		return c.Redirect("/")
	}

	// Erfassen der IP-Adresse des Nutzers mit der gemeinsamen Hilfsfunktion
	ipv4, ipv6 := GetClientIP(c)

	image := models.Image{
		UUID:     imageUUID,
		UserID:   c.Locals(USER_ID).(uint),
		FileName: fileName,
		FilePath: originalDirPath, // Save the new path in the database
		FileSize: file.Size,
		FileType: fileExt,
		Title:    file.Filename,
		IPv4:     ipv4,
		IPv6:     ipv6,
	}

	db := database.GetDB()
	if err := db.Create(&image).Error; err != nil {
		fiberlog.Error(fmt.Sprintf("Error saving image to database: %v", err))

		// Clean up the file if database insertion fails
		os.Remove(originalSavePath)

		fm := fiber.Map{
			"type":    "error",
			"message": fmt.Sprintf("Datei konnte nicht gespeichert werden: %s", file.Filename),
		}
		flash.WithError(c, fm)

		if c.Get("HX-Request") == "true" {
			return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Fehler beim Speichern: %s", err))
		}

		return c.Redirect("/")
	}

	// Start image processing asynchronously with a semaphore to limit concurrent processing
	go func() {
		fiberlog.Info(fmt.Sprintf("[Upload] Starting asynchronous image processing for %s", image.UUID))
		if err := imageprocessor.ProcessImage(&image); err != nil {
			fiberlog.Error(fmt.Sprintf("Error during image processing: %v", err))
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
		flash.WithSuccess(c, fm)

		redirectPath := fmt.Sprintf("/image/%s", imageUUID)
		c.Set("HX-Redirect", redirectPath)
		return c.SendString(fmt.Sprintf("Datei erfolgreich hochgeladen: %s", file.Filename))
	}

	// Otherwise, redirect to the image view page
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
		fiberlog.Info(fmt.Sprintf("Image not found with ShareLink: %s, Error: %v", sharelink, err))
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

	// Check if the identifier is a UUID (36 characters with hyphens)
	isUUID := len(identifier) == 36 && strings.Count(identifier, "-") == 4
	if isUUID == false {
		return c.SendStatus(404)
	}

	// Try to find the image by UUID first
	db := database.GetDB()
	image, err := models.FindImageByUUID(db, identifier)
	if err != nil {
		fiberlog.Info(fmt.Sprintf("Image not found with UUID: %s, Error: %v", identifier, err))

		return c.Redirect("/")
	}

	// The FilePath contains the relative path within the uploads folder
	domain := env.GetEnv("PUBLIC_DOMAIN", "")

	// Increase the view counter
	image.IncrementViewCount(db)

	// Korrekte Pfadkonstruktion für das Original-Bild mit GetImagePath
	filePathComplete := "/" + imageprocessor.GetImagePath(image, "original", "")
	fiberlog.Debugf("[ImageController] Original-Pfad: %s", filePathComplete)
	filePathWithDomain := filepath.Join(domain, filePathComplete)

	// Generate the share link URL with the domain and the new /i/ path
	shareURL := filepath.Join(domain, "/i/", image.ShareLink)

	// Paths for optimized image formats
	webpPath := ""
	avifPath := ""
	// Nur WebP und AVIF Small-Thumbnails existieren
	smallThumbWebpPath := ""
	smallThumbAvifPath := ""

	// Check if any optimized versions are available
	hasOptimizedVersions := image.HasWebp || image.HasAVIF || image.HasThumbnailSmall || image.HasThumbnailMedium

	// If optimized formats are available, generate the paths
	if image.HasWebp {
		webpRelativePath := imageprocessor.GetImagePath(image, "webp", "")
		webpPath = "/" + webpRelativePath
	}

	if image.HasAVIF {
		avifRelativePath := imageprocessor.GetImagePath(image, "avif", "")
		avifPath = "/" + avifRelativePath
	}

	// If thumbnails are available, generate the paths
	if image.HasThumbnailSmall {
		if image.HasWebp {
			smallThumbWebpRelativePath := imageprocessor.GetImagePath(image, "webp", "small")
			smallThumbWebpPath = "/" + smallThumbWebpRelativePath
		}

		if image.HasAVIF {
			smallThumbAvifRelativePath := imageprocessor.GetImagePath(image, "avif", "small")
			smallThumbAvifPath = "/" + smallThumbAvifRelativePath
		}
	}

	// Paths for medium thumbnails
	mediumThumbWebpPath := ""
	mediumThumbAvifPath := ""

	// If medium thumbnails are available, generate the paths
	if image.HasThumbnailMedium {
		// Path to the medium thumbnail (WebP)
		if image.HasWebp {
			mediumThumbWebpRelativePath := imageprocessor.GetImagePath(image, "webp", "medium")
			mediumThumbWebpPath = "/" + mediumThumbWebpRelativePath
		}

		// Path to the medium thumbnail (AVIF)
		if image.HasAVIF {
			mediumThumbAvifRelativePath := imageprocessor.GetImagePath(image, "avif", "medium")
			mediumThumbAvifPath = "/" + mediumThumbAvifRelativePath
		}
	}

	// Use the medium thumbnail for the preview
	previewPath := filePathComplete
	previewWebpPath := webpPath
	previewAvifPath := avifPath

	if image.HasThumbnailMedium {
		// Set the paths for medium thumbnails, if available
		if image.HasAVIF {
			previewPath = mediumThumbAvifPath
			previewAvifPath = mediumThumbAvifPath
		}
		// Set the WebP path independently of AVIF
		if image.HasWebp {
			if !image.HasAVIF { // Only change the main path if no AVIF is available
				previewPath = mediumThumbWebpPath
			}
			previewWebpPath = mediumThumbWebpPath
		}
	} else if image.HasThumbnailSmall {
		// Fallback to small thumbnail if medium is not available
		if image.HasAVIF {
			previewPath = smallThumbAvifPath
			previewAvifPath = smallThumbAvifPath
		}
		// Set the WebP path independently of AVIF
		if image.HasWebp {
			if !image.HasAVIF { // Only change the main path if no AVIF is available
				previewPath = smallThumbWebpPath
			}
			previewWebpPath = smallThumbWebpPath
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

	// Optimized version for direct links and embeddings (AVIF or WebP)
	// Note: This variable is no longer needed, as optimization is done directly in the template
	// through the use of hasAVIF, hasWebP, avifPath, and webpPath

	// Prepare Open Graph meta tags
	ogImage := ""
	if image.HasThumbnailSmall {
		// Use small thumbnail for OG tags
		if image.HasAVIF {
			ogImage = filepath.Join(domain, smallThumbAvifPath)
		} else if image.HasWebp {
			ogImage = filepath.Join(domain, smallThumbWebpPath)
		}
	} else {
		// If no thumbnails are available, use the original
		ogImage = filePathWithDomain
	}

	// Use the original file name (title) for display, if available
	displayName := image.FileName
	if image.Title != "" {
		displayName = image.Title
	}

	ogTitle := fmt.Sprintf("%s - %s", displayName, "PIXELFOX.cc")
	ogDescription := "Image uploaded on PIXELFOX.cc - Free image hosting"

	isAdmin := false
	if isLoggedIn(c) {
		sess, _ := session.GetSessionStore().Get(c)
		isAdmin = sess.Get(USER_IS_ADMIN).(bool)
	}

	// Create the ImageViewModel
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
		SmallWebPPath:      smallThumbWebpPath,
		SmallAVIFPath:      smallThumbAvifPath,
		OptimizedWebPPath:  optimizedWebpPath,
		OptimizedAVIFPath:  optimizedAvifPath,
		OriginalPath:       filePathComplete,
		Width:              image.Width,
		Height:             image.Height,
		UUID:               image.UUID,
		IsProcessing:       true, // Wird später geprüft
		CameraModel: func() string {
			if image.Metadata != nil && image.Metadata.CameraModel != nil {
				return *image.Metadata.CameraModel
			}
			return ""
		}(),
		TakenAt: func() string {
			if image.Metadata != nil && image.Metadata.TakenAt != nil {
				return image.Metadata.TakenAt.Format("02.01.2006 15:04")
			}
			return ""
		}(),
		Latitude: func() string {
			if image.Metadata != nil && image.Metadata.Latitude != nil {
				return fmt.Sprintf("%f", *image.Metadata.Latitude)
			}
			return ""
		}(),
		Longitude: func() string {
			if image.Metadata != nil && image.Metadata.Longitude != nil {
				return fmt.Sprintf("%f", *image.Metadata.Longitude)
			}
			return ""
		}(),
		ExposureTime: func() string {
			if image.Metadata != nil && image.Metadata.ExposureTime != nil {
				return *image.Metadata.ExposureTime
			}
			return ""
		}(),
		Aperture: func() string {
			if image.Metadata != nil && image.Metadata.Aperture != nil {
				return *image.Metadata.Aperture
			}
			return ""
		}(),
		ISO: func() string {
			if image.Metadata != nil && image.Metadata.ISO != nil {
				return strconv.Itoa(*image.Metadata.ISO)
			}
			return ""
		}(),
		FocalLength: func() string {
			if image.Metadata != nil && image.Metadata.FocalLength != nil {
				return *image.Metadata.FocalLength
			}
			return ""
		}(),
		HasOptimizedVersions: hasOptimizedVersions,
	}

	imageViewer := views.ImageViewer(imageModel)

	ogViewModel := &viewmodel.OpenGraph{
		URL:         shareURL,
		Image:       ogImage,
		ImageAlt:    ogTitle,
		Title:       ogTitle,
		Description: ogDescription,
	}

	home := views.Home(fmt.Sprintf("| Bild %s ansehen", imageModel.DisplayName), isLoggedIn(c), false, flash.Get(c), imageViewer, isAdmin, ogViewModel)

	handler := adaptor.HTTPHandler(templ.Handler(home))

	return handler(c)
}

func HandleImageProcessingStatus(c *fiber.Ctx) error {
	uuid := c.Params("uuid")
	if uuid == "" {
		return c.Status(fiber.StatusBadRequest).SendString("UUID missing")
	}

	// Check if the image is complete
	isComplete := imageprocessor.IsImageProcessingComplete(uuid)

	// Get the image from the database
	db := database.GetDB()
	image, err := models.FindImageByUUID(db, uuid)
	if err != nil {
		fiberlog.Error(err)
		return c.Status(fiber.StatusNotFound).SendString("Image not found")
	}

	// Check if any optimized versions are available
	hasOptimizedVersions := image.HasWebp || image.HasAVIF || image.HasThumbnailSmall || image.HasThumbnailMedium

	// Use the original file name (title) for display, if available
	displayName := image.FileName
	if image.Title != "" {
		displayName = image.Title
	} // If the image is still processing but exists in the database,
	// send a partial model with IsProcessing=true
	if !isComplete && err == nil {
		// Create a view model with preliminary data
		imageModel := viewmodel.Image{
			UUID:         uuid,
			DisplayName:  displayName,
			ShareURL:     fmt.Sprintf("%s/i/%s", c.BaseURL(), image.ShareLink),
			Domain:       c.BaseURL(),
			OriginalPath: "/" + filepath.Join(image.FilePath, image.FileName),
			IsProcessing: true,
			CameraModel: func() string {
				if image.Metadata != nil && image.Metadata.CameraModel != nil {
					return *image.Metadata.CameraModel
				}
				return ""
			}(),
			TakenAt: func() string {
				if image.Metadata != nil && image.Metadata.TakenAt != nil {
					return image.Metadata.TakenAt.Format("02.01.2006 15:04")
				}
				return ""
			}(),
			Latitude: func() string {
				if image.Metadata != nil && image.Metadata.Latitude != nil {
					return fmt.Sprintf("%f", *image.Metadata.Latitude)
				}
				return ""
			}(),
			Longitude: func() string {
				if image.Metadata != nil && image.Metadata.Longitude != nil {
					return fmt.Sprintf("%f", *image.Metadata.Longitude)
				}
				return ""
			}(),
			ExposureTime: func() string {
				if image.Metadata != nil && image.Metadata.ExposureTime != nil {
					return *image.Metadata.ExposureTime
				}
				return ""
			}(),
			Aperture: func() string {
				if image.Metadata != nil && image.Metadata.Aperture != nil {
					return *image.Metadata.Aperture
				}
				return ""
			}(),
			ISO: func() string {
				if image.Metadata != nil && image.Metadata.ISO != nil {
					return strconv.Itoa(*image.Metadata.ISO)
				}
				return ""
			}(),
			FocalLength: func() string {
				if image.Metadata != nil && image.Metadata.FocalLength != nil {
					return *image.Metadata.FocalLength
				}
				return ""
			}(),
		}

		// Render the entire card with IsProcessing = true
		return views.ImageViewer(imageModel).Render(c.Context(), c.Response().BodyWriter())
	}

	// If the image is not found or processing is not complete,
	// send an error for better error handling in the frontend
	if !isComplete || err != nil {
		return c.Status(fiber.StatusNotFound).SendString("Image not found or still processing")
	}

	// The image is complete and exists in the database
	// Construct the image paths
	previewPath := ""
	previewWebpPath := ""
	previewAvifPath := ""
	// Paths for optimized versions
	optimizedWebpPath := ""
	optimizedAvifPath := ""

	// Use the medium thumbnail for the preview
	if image.HasThumbnailMedium {
		previewPath = "/" + imageprocessor.GetImagePath(image, "", "medium")
		if image.HasWebp {
			previewWebpPath = "/" + imageprocessor.GetImagePath(image, "webp", "medium")
		}
		if image.HasAVIF {
			previewAvifPath = "/" + imageprocessor.GetImagePath(image, "avif", "medium")
		}
	} else if image.HasThumbnailSmall {
		previewPath = "/" + imageprocessor.GetImagePath(image, "", "small")
		if image.HasWebp {
			previewWebpPath = "/" + imageprocessor.GetImagePath(image, "webp", "small")
		}
		if image.HasAVIF {
			previewAvifPath = "/" + imageprocessor.GetImagePath(image, "avif", "small")
		}
	} else {
		// Use the original if no thumbnails are available
		previewPath = "/" + imageprocessor.GetImagePath(image, "original", "")
	}

	// Set the paths for the optimized versions (for the lightbox)
	if image.HasWebp {
		optimizedWebpPath = "/" + imageprocessor.GetImagePath(image, "webp", "")
	}
	if image.HasAVIF {
		optimizedAvifPath = "/" + imageprocessor.GetImagePath(image, "avif", "")
	}

	// Original path for download
	// Create the full path to the original (FilePath only contains the directory, so we need to add the file name)
	originalPath := "/" + filepath.Join(image.FilePath, image.FileName)

	// Create a simplified view model for image display only
	imageModel := viewmodel.Image{
		PreviewPath:       previewPath,
		PreviewWebPPath:   previewWebpPath,
		PreviewAVIFPath:   previewAvifPath,
		SmallWebPPath:     "/" + imageprocessor.GetImagePath(image, "webp", "small"),
		SmallAVIFPath:     "/" + imageprocessor.GetImagePath(image, "avif", "small"),
		OptimizedWebPPath: optimizedWebpPath,
		OptimizedAVIFPath: optimizedAvifPath,
		OriginalPath:      originalPath,
		DisplayName:       displayName,
		HasWebP:           image.HasWebp,
		HasAVIF:           image.HasAVIF,
		Width:             image.Width,
		Height:            image.Height,
		IsProcessing:      false,
		UUID:              image.UUID,
		ShareURL:          fmt.Sprintf("%s/i/%s", c.BaseURL(), image.ShareLink),
		Domain:            c.BaseURL(),
		CameraModel: func() string {
			if image.Metadata != nil && image.Metadata.CameraModel != nil {
				return *image.Metadata.CameraModel
			}
			return ""
		}(),
		TakenAt: func() string {
			if image.Metadata != nil && image.Metadata.TakenAt != nil {
				return image.Metadata.TakenAt.Format("02.01.2006 15:04")
			}
			return ""
		}(),
		Latitude: func() string {
			if image.Metadata != nil && image.Metadata.Latitude != nil {
				return fmt.Sprintf("%f", *image.Metadata.Latitude)
			}
			return ""
		}(),
		Longitude: func() string {
			if image.Metadata != nil && image.Metadata.Longitude != nil {
				return fmt.Sprintf("%f", *image.Metadata.Longitude)
			}
			return ""
		}(),
		ExposureTime: func() string {
			if image.Metadata != nil && image.Metadata.ExposureTime != nil {
				return *image.Metadata.ExposureTime
			}
			return ""
		}(),
		Aperture: func() string {
			if image.Metadata != nil && image.Metadata.Aperture != nil {
				return *image.Metadata.Aperture
			}
			return ""
		}(),
		ISO: func() string {
			if image.Metadata != nil && image.Metadata.ISO != nil {
				return strconv.Itoa(*image.Metadata.ISO)
			}
			return ""
		}(),
		FocalLength: func() string {
			if image.Metadata != nil && image.Metadata.FocalLength != nil {
				return *image.Metadata.FocalLength
			}
			return ""
		}(),
		HasOptimizedVersions: hasOptimizedVersions,
	}

	// Render the entire card with the ImageViewer
	return views.ImageViewer(imageModel).Render(c.Context(), c.Response().BodyWriter())
}
