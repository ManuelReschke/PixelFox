package controllers

import (
	"crypto/sha256"
	"encoding/hex"
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
	"github.com/ManuelReschke/PixelFox/app/repository"
	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
	"github.com/ManuelReschke/PixelFox/internal/pkg/imageprocessor"
	"github.com/ManuelReschke/PixelFox/internal/pkg/jobqueue"
	"github.com/ManuelReschke/PixelFox/internal/pkg/session"
	"github.com/ManuelReschke/PixelFox/internal/pkg/statistics"
	"github.com/ManuelReschke/PixelFox/internal/pkg/storage"
	"github.com/ManuelReschke/PixelFox/internal/pkg/viewmodel"
	"github.com/ManuelReschke/PixelFox/views"
)

// calculateFileHash calculates SHA-256 hash of file content
func calculateFileHash(file io.Reader) (string, error) {
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

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
	fileExt := strings.ToLower(filepath.Ext(file.Filename))
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

	// Check for unsupported but common image formats for better error messages
	unsupportedButCommon := map[string]string{
		".heic": "HEIC-Format wird derzeit nicht unterstützt. Bitte konvertiere das Bild zu JPG oder PNG.",
		".heif": "HEIF-Format wird derzeit nicht unterstützt. Bitte konvertiere das Bild zu JPG oder PNG.",
		".tiff": "TIFF-Format wird derzeit nicht unterstützt. Bitte konvertiere das Bild zu JPG oder PNG.",
		".tif":  "TIF-Format wird derzeit nicht unterstützt. Bitte konvertiere das Bild zu JPG oder PNG.",
		".raw":  "RAW-Format wird derzeit nicht unterstützt. Bitte konvertiere das Bild zu JPG oder PNG.",
		".cr2":  "Canon RAW-Format wird derzeit nicht unterstützt. Bitte konvertiere das Bild zu JPG oder PNG.",
		".nef":  "Nikon RAW-Format wird derzeit nicht unterstützt. Bitte konvertiere das Bild zu JPG oder PNG.",
		".arw":  "Sony RAW-Format wird derzeit nicht unterstützt. Bitte konvertiere das Bild zu JPG oder PNG.",
		".dng":  "DNG RAW-Format wird derzeit nicht unterstützt. Bitte konvertiere das Bild zu JPG oder PNG.",
	}

	if !validImageExtensions[fileExt] {
		var errorMessage string
		if specificMessage, exists := unsupportedButCommon[fileExt]; exists {
			errorMessage = specificMessage
		} else {
			errorMessage = "Nur folgende Bildformate werden unterstützt: JPG, JPEG, PNG, GIF, WEBP, AVIF, SVG, BMP"
		}

		fm := fiber.Map{
			"type":    "error",
			"message": errorMessage,
		}
		flash.WithError(c, fm)

		if c.Get("HX-Request") == "true" {
			return c.Status(fiber.StatusUnsupportedMediaType).SendString(errorMessage)
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

	// Calculate file hash for duplicate detection
	fileHash, err := calculateFileHash(src)
	if err != nil {
		fiberlog.Error(fmt.Sprintf("Error calculating file hash: %v", err))

		fm := fiber.Map{
			"type":    "error",
			"message": "Fehler beim Verarbeiten der Datei",
		}
		flash.WithError(c, fm)

		if c.Get("HX-Request") == "true" {
			return c.Status(fiber.StatusInternalServerError).SendString("Fehler beim Verarbeiten der Datei")
		}
		return c.Redirect("/")
	}

	// Reset file position after hash calculation
	src.Close()
	src, err = file.Open()
	if err != nil {
		fiberlog.Error(fmt.Sprintf("Error reopening file: %v", err))
		return c.Status(fiber.StatusInternalServerError).SendString("Fehler beim Verarbeiten der Datei")
	}
	defer src.Close()

	// Check for duplicate files by this user
	userID := c.Locals(USER_ID).(uint)
	imageRepo := repository.GetGlobalFactory().GetImageRepository()
	existingImage, err := imageRepo.GetByUserIDAndFileHash(userID, fileHash)
	if err == nil {
		// Duplicate found! Return user-friendly response
		fiberlog.Info(fmt.Sprintf("[Upload] Duplicate file detected for user %d, redirecting to existing image %s", userID, existingImage.UUID))

		if c.Get("HX-Request") == "true" {
			// Return user-friendly HTML response for HTMX
			duplicateTitle := existingImage.Title
			if duplicateTitle == "" {
				duplicateTitle = existingImage.FileName
			}

			htmlResponse := fmt.Sprintf(`
				<div class="alert alert-info shadow-lg mb-4">
					<div>
						<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="stroke-current flex-shrink-0 w-6 h-6">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
						</svg>
						<div>
							<h3 class="font-bold">Bild bereits vorhanden!</h3>
							<div class="text-xs">Du hast dieses Bild bereits hochgeladen: "%s"</div>
						</div>
					</div>
					<div class="flex-none">
						<a href="/image/%s" class="btn btn-sm btn-outline">
							📷 Bild ansehen
						</a>
					</div>
				</div>
			`, duplicateTitle, existingImage.UUID)

			return c.Status(fiber.StatusOK).Type("text/html").SendString(htmlResponse)
		} else {
			// For normal form submits, set flash message and redirect
			fm := fiber.Map{
				"type":           "info",
				"message":        "Du hast dieses Bild bereits hochgeladen!",
				"existing_image": existingImage.UUID,
				"existing_title": existingImage.Title,
			}
			flash.WithInfo(c, fm)
			return c.Redirect("/image/" + existingImage.UUID)
		}
	}

	// Generate UUID for the image
	imageUUID := uuid.New().String()

	// Select optimal storage pool for upload (hot-storage-first)
	storageManager := storage.NewStorageManager()
	selectedPool, err := storageManager.SelectPoolForUpload(file.Size)
	if err != nil {
		fiberlog.Error(fmt.Sprintf("Error selecting storage pool: %v", err))

		fm := fiber.Map{
			"type":    "error",
			"message": "Fehler bei der Speicherplatz-Auswahl",
		}
		flash.WithError(c, fm)

		if c.Get("HX-Request") == "true" {
			return c.Status(fiber.StatusInternalServerError).SendString("Fehler bei der Speicherplatz-Auswahl")
		}
		return c.Redirect("/")
	}

	fiberlog.Info(fmt.Sprintf("[Upload] Selected %s storage pool '%s' for upload", selectedPool.StorageTier, selectedPool.Name))

	// Create the directory path according to the scheme /Year/Month/Day/UUID.fileextension
	now := time.Now()
	relativePath := fmt.Sprintf("%d/%02d/%02d", now.Year(), now.Month(), now.Day())
	fileName := fmt.Sprintf("%s%s", imageUUID, fileExt)

	// Use the selected storage pool's base path
	poolBasePath := selectedPool.BasePath
	if !strings.HasSuffix(poolBasePath, "/") {
		poolBasePath += "/"
	}
	originalDirPath := filepath.Join(poolBasePath, "original", relativePath)
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
		UUID:          imageUUID,
		UserID:        c.Locals(USER_ID).(uint),
		StoragePoolID: selectedPool.ID,
		FileName:      fileName,
		FilePath:      filepath.Join("original", relativePath), // Store relative path within the pool
		FileSize:      file.Size,
		FileType:      fileExt,
		Title:         file.Filename,
		FileHash:      fileHash,
		IPv4:          ipv4,
		IPv6:          ipv6,
	}
	if err := imageRepo.Create(&image); err != nil {
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

	// Update storage pool usage
	if err := storageManager.UpdatePoolUsage(selectedPool.ID, file.Size); err != nil {
		fiberlog.Error(fmt.Sprintf("Error updating storage pool usage: %v", err))
		// Don't fail the upload for this, just log it
	}

	// Enqueue image processing in the unified queue (includes S3 backup if enabled)
	fiberlog.Info(fmt.Sprintf("[Upload] Enqueueing unified image processing for %s", image.UUID))
	if err := jobqueue.ProcessImageUnified(&image); err != nil {
		fiberlog.Error(fmt.Sprintf("Error enqueueing unified image processing for %s: %v", image.UUID, err))
	}

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

	imageRepo := repository.GetGlobalFactory().GetImageRepository()
	image, err := imageRepo.GetByShareLink(sharelink)
	if err != nil {
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
	imageRepo := repository.GetGlobalFactory().GetImageRepository()
	image, err := imageRepo.GetByUUID(identifier)
	if err != nil {
		fiberlog.Info(fmt.Sprintf("Image not found with UUID: %s, Error: %v", identifier, err))

		return c.Redirect("/")
	}

	// Get current user ID from session if logged in
	var currentUserID uint = 0
	if isLoggedIn(c) {
		sess, _ := session.GetSessionStore().Get(c)
		if userID := sess.Get(USER_ID); userID != nil {
			currentUserID = userID.(uint)
		}
	}

	// The FilePath contains the relative path within the uploads folder
	domain := env.GetEnv("PUBLIC_DOMAIN", "")

	// Increase the view counter
	imageRepo.UpdateViewCount(image.ID)

	// Korrekte URL-Konstruktion für das Original-Bild mit GetImageURL
	filePathComplete := imageprocessor.GetImageURL(image, "original", "")
	fiberlog.Debugf("[ImageController] Original-Pfad: %s", filePathComplete)
	filePathWithDomain := filepath.Join(domain, filePathComplete)

	// Generate the share link URL with the domain and the new /i/ path
	shareURL := filepath.Join(domain, "/i/", image.ShareLink)

	// Get variant information for this image
	variantInfo, err := imageprocessor.GetImageVariantInfo(image.ID)
	if err != nil {
		variantInfo = &imageprocessor.VariantInfo{} // fallback to empty
	}

	// Build all image paths using the new variant system
	imagePaths := imageprocessor.BuildImagePaths(image)

	// Extract paths with proper URL format for all available variants
	webpPath := ""
	avifPath := ""
	smallThumbWebpPath := ""
	smallThumbAvifPath := ""
	mediumThumbWebpPath := ""
	mediumThumbAvifPath := ""
	smallThumbOriginalPath := ""
	mediumThumbOriginalPath := ""

	if path, exists := imagePaths["webp_full"]; exists {
		webpPath = path
	}
	if path, exists := imagePaths["avif_full"]; exists {
		avifPath = path
	}

	// Set all available thumbnail paths regardless of admin settings
	if path, exists := imagePaths["thumbnail_small_webp"]; exists {
		smallThumbWebpPath = path
	}
	if path, exists := imagePaths["thumbnail_medium_webp"]; exists {
		mediumThumbWebpPath = path
	}

	if path, exists := imagePaths["thumbnail_small_avif"]; exists {
		smallThumbAvifPath = path
	}
	if path, exists := imagePaths["thumbnail_medium_avif"]; exists {
		mediumThumbAvifPath = path
	}

	if path, exists := imagePaths["thumbnail_small_original"]; exists {
		smallThumbOriginalPath = path
	}
	if path, exists := imagePaths["thumbnail_medium_original"]; exists {
		mediumThumbOriginalPath = path
	}

	// Use the medium thumbnail for the preview
	previewPath := filePathComplete
	previewWebpPath := ""
	previewAvifPath := ""

	if variantInfo.HasThumbnailMedium {
		// Set the paths for medium thumbnails, with priority: AVIF > WebP > Original Format
		if mediumThumbAvifPath != "" {
			previewPath = mediumThumbAvifPath
			previewAvifPath = mediumThumbAvifPath
		} else if mediumThumbWebpPath != "" {
			previewPath = mediumThumbWebpPath
			previewWebpPath = mediumThumbWebpPath
		} else if mediumThumbOriginalPath != "" {
			previewPath = mediumThumbOriginalPath
		}

		// Set the WebP path independently if available
		if mediumThumbWebpPath != "" {
			previewWebpPath = mediumThumbWebpPath
		}
	} else if variantInfo.HasThumbnailSmall {
		// Fallback to small thumbnail if medium is not available, with priority: AVIF > WebP > Original Format
		if smallThumbAvifPath != "" {
			previewPath = smallThumbAvifPath
			previewAvifPath = smallThumbAvifPath
		} else if smallThumbWebpPath != "" {
			previewPath = smallThumbWebpPath
			previewWebpPath = smallThumbWebpPath
		} else if smallThumbOriginalPath != "" {
			previewPath = smallThumbOriginalPath
		}

		// Set the WebP path independently if available
		if smallThumbWebpPath != "" {
			previewWebpPath = smallThumbWebpPath
		}
	}

	// Get paths for optimized versions
	optimizedWebpPath := webpPath
	optimizedAvifPath := avifPath

	// Prepare Open Graph meta tags
	ogImage := ""
	if variantInfo.HasThumbnailSmall {
		// Use small thumbnail for OG tags
		if smallThumbAvifPath != "" {
			ogImage = filepath.Join(domain, smallThumbAvifPath)
		} else if smallThumbWebpPath != "" {
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
		Domain:               domain,
		PreviewPath:          previewPath,
		FilePathWithDomain:   filePathWithDomain,
		DisplayName:          displayName,
		ShareURL:             shareURL,
		HasWebP:              variantInfo.HasWebP,
		HasAVIF:              variantInfo.HasAVIF,
		PreviewWebPPath:      previewWebpPath,
		PreviewAVIFPath:      previewAvifPath,
		PreviewOriginalPath:  mediumThumbOriginalPath,
		SmallWebPPath:        smallThumbWebpPath,
		SmallAVIFPath:        smallThumbAvifPath,
		SmallOriginalPath:    smallThumbOriginalPath,
		OptimizedWebPPath:    optimizedWebpPath,
		OptimizedAVIFPath:    optimizedAvifPath,
		OriginalPath:         filePathComplete,
		Width:                image.Width,
		Height:               image.Height,
		UUID:                 image.UUID,
		IsProcessing:         !imageprocessor.IsImageProcessingComplete(image.UUID),
		HasOptimizedVersions: variantInfo.HasWebP || variantInfo.HasAVIF || variantInfo.HasThumbnailSmall || variantInfo.HasThumbnailMedium,
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

	imageViewer := views.ImageViewerWithUser(imageModel, currentUserID, image.UserID)

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
	imageRepo := repository.GetGlobalFactory().GetImageRepository()
	image, err := imageRepo.GetByUUID(uuid)
	if err != nil {
		fiberlog.Error(err)
		return c.Status(fiber.StatusNotFound).SendString("Image not found")
	}

	// Get current user ID from session if logged in
	var currentUserID uint = 0
	if isLoggedIn(c) {
		sess, _ := session.GetSessionStore().Get(c)
		if userID := sess.Get(USER_ID); userID != nil {
			currentUserID = userID.(uint)
		}
	}

	// Check if any optimized versions are available (for Ajax response)
	variantInfoAjax, err := imageprocessor.GetImageVariantInfo(image.ID)
	if err != nil {
		variantInfoAjax = &imageprocessor.VariantInfo{} // fallback to empty
	}
	hasOptimizedVersions := variantInfoAjax.HasWebP || variantInfoAjax.HasAVIF || variantInfoAjax.HasThumbnailSmall || variantInfoAjax.HasThumbnailMedium

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
		return views.ImageViewerWithUser(imageModel, currentUserID, image.UserID).Render(c.Context(), c.Response().BodyWriter())
	}

	// If the image is not found or processing is not complete,
	// send an error for better error handling in the frontend
	if !isComplete || err != nil {
		return c.Status(fiber.StatusNotFound).SendString("Image not found or still processing")
	}

	// The image is complete and exists in the database
	// Build all image paths using the new variant system
	imagePaths := imageprocessor.BuildImagePaths(image)

	// Extract paths with proper URL format for all available variants
	webpPath := ""
	avifPath := ""
	smallThumbWebpPath := ""
	smallThumbAvifPath := ""
	mediumThumbWebpPath := ""
	mediumThumbAvifPath := ""
	smallThumbOriginalPath := ""
	mediumThumbOriginalPath := ""

	if path, exists := imagePaths["webp_full"]; exists {
		webpPath = path
	}
	if path, exists := imagePaths["avif_full"]; exists {
		avifPath = path
	}

	// Set all available thumbnail paths regardless of admin settings
	if path, exists := imagePaths["thumbnail_small_webp"]; exists {
		smallThumbWebpPath = path
	}
	if path, exists := imagePaths["thumbnail_medium_webp"]; exists {
		mediumThumbWebpPath = path
	}

	if path, exists := imagePaths["thumbnail_small_avif"]; exists {
		smallThumbAvifPath = path
	}
	if path, exists := imagePaths["thumbnail_medium_avif"]; exists {
		mediumThumbAvifPath = path
	}

	if path, exists := imagePaths["thumbnail_small_original"]; exists {
		smallThumbOriginalPath = path
	}
	if path, exists := imagePaths["thumbnail_medium_original"]; exists {
		mediumThumbOriginalPath = path
	}

	// Original path for download
	originalPath := imageprocessor.GetImageURL(image, "original", "")

	// Use the medium thumbnail for the preview
	previewPath := originalPath
	previewWebPPath := ""
	previewAVIFPath := ""

	if variantInfoAjax.HasThumbnailMedium {
		// Set the paths for medium thumbnails, with priority: AVIF > WebP > Original Format
		if mediumThumbAvifPath != "" {
			previewPath = mediumThumbAvifPath
			previewAVIFPath = mediumThumbAvifPath
		} else if mediumThumbWebpPath != "" {
			previewPath = mediumThumbWebpPath
			previewWebPPath = mediumThumbWebpPath
		} else if mediumThumbOriginalPath != "" {
			previewPath = mediumThumbOriginalPath
		}

		// Set the WebP path independently if available
		if mediumThumbWebpPath != "" {
			previewWebPPath = mediumThumbWebpPath
		}
	} else if variantInfoAjax.HasThumbnailSmall {
		// Fallback to small thumbnail if medium is not available, with priority: AVIF > WebP > Original Format
		if smallThumbAvifPath != "" {
			previewPath = smallThumbAvifPath
			previewAVIFPath = smallThumbAvifPath
		} else if smallThumbWebpPath != "" {
			previewPath = smallThumbWebpPath
			previewWebPPath = smallThumbWebpPath
		} else if smallThumbOriginalPath != "" {
			previewPath = smallThumbOriginalPath
		}

		// Set the WebP path independently if available
		if smallThumbWebpPath != "" {
			previewWebPPath = smallThumbWebpPath
		}
	}

	// Get paths for optimized versions
	optimizedWebpPath := webpPath
	optimizedAvifPath := avifPath

	// Create a simplified view model for image display only
	imageModel := viewmodel.Image{
		PreviewPath:          previewPath,
		PreviewWebPPath:      previewWebPPath,
		PreviewAVIFPath:      previewAVIFPath,
		PreviewOriginalPath:  mediumThumbOriginalPath,
		SmallWebPPath:        smallThumbWebpPath,
		SmallAVIFPath:        smallThumbAvifPath,
		SmallOriginalPath:    smallThumbOriginalPath,
		OptimizedWebPPath:    optimizedWebpPath,
		OptimizedAVIFPath:    optimizedAvifPath,
		OriginalPath:         originalPath,
		DisplayName:          displayName,
		HasWebP:              variantInfoAjax.HasWebP,
		HasAVIF:              variantInfoAjax.HasAVIF,
		Width:                image.Width,
		Height:               image.Height,
		IsProcessing:         false,
		UUID:                 image.UUID,
		ShareURL:             fmt.Sprintf("%s/i/%s", c.BaseURL(), image.ShareLink),
		Domain:               c.BaseURL(),
		HasOptimizedVersions: hasOptimizedVersions,
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

	// Render the entire card with the ImageViewer
	return views.ImageViewerWithUser(imageModel, currentUserID, image.UserID).Render(c.Context(), c.Response().BodyWriter())
}

// enqueueS3BackupIfEnabled is deprecated - replaced by unified queue system
// This function is kept for backwards compatibility but should not be used
// Use jobqueue.ProcessImageUnified() instead which handles both processing and backup
