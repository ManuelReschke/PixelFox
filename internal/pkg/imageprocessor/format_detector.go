package imageprocessor

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"

	"github.com/ManuelReschke/PixelFox/app/models"
)

// GetOptimalImageFormat bestimmt das optimale Bildformat basierend auf dem Accept-Header des Browsers
func GetOptimalImageFormat(c *fiber.Ctx, image *models.Image) string {
	// Prüfe den Accept-Header
	acceptHeader := c.Get("Accept")

	// Prüfe, ob der Browser AVIF unterstützt
	if strings.Contains(acceptHeader, "image/avif") && HasAVIF(image) {
		return "avif"
	}

	// Prüfe, ob der Browser WebP unterstützt
	if strings.Contains(acceptHeader, "image/webp") && HasWebP(image) {
		return "webp"
	}

	// Fallback auf das Originalformat
	return "original"
}

// GetOptimizedFormat returns the most optimized format for the image based on browser support
func GetOptimizedFormat(c *fiber.Ctx, image *models.Image) (string, string, error) {
	// Check if browser supports AVIF
	if strings.Contains(c.Get("Accept"), "image/avif") && HasAVIF(image) {
		log.Info(fmt.Sprintf("[FormatDetector] Browser supports AVIF, serving AVIF version for %s", image.UUID))
		path, err := GetAVIFPath(image)
		return path, "image/avif", err
	}

	// Check if browser supports WebP
	if strings.Contains(c.Get("Accept"), "image/webp") && HasWebP(image) {
		log.Info(fmt.Sprintf("[FormatDetector] Browser supports WebP, serving WebP version for %s", image.UUID))
		path, err := GetWebPPath(image)
		return path, "image/webp", err
	}

	// Fallback to original
	log.Info(fmt.Sprintf("[FormatDetector] Serving original version for %s", image.UUID))
	path, err := GetOriginalPath(image)
	if err != nil {
		return "", "", err
	}

	// Determine content type based on file extension
	ext := filepath.Ext(path)
	contentType := getContentTypeFromExtension(ext)

	return path, contentType, nil
}

// GetThumbnailFormat returns the most optimized thumbnail format
func GetThumbnailFormat(c *fiber.Ctx, image *models.Image, size string) (string, string, error) {
	if size != "small" && size != "medium" {
		return "", "", fmt.Errorf("invalid thumbnail size: %s", size)
	}

	// Check if thumbnail exists
	var hasThumb bool
	if size == "small" {
		hasThumb = HasThumbnailSmall(image)
	} else {
		hasThumb = HasThumbnailMedium(image)
	}

	if !hasThumb {
		return "", "", fmt.Errorf("thumbnail %s does not exist for image %s", size, image.UUID)
	}

	// Check if browser supports AVIF
	if strings.Contains(c.Get("Accept"), "image/avif") && HasAVIF(image) {
		log.Info(fmt.Sprintf("[FormatDetector] Browser supports AVIF, serving AVIF thumbnail for %s", image.UUID))
		path := filepath.Join(ThumbnailsDir, size, "avif", image.UUID, image.Filename)
		return path, "image/avif", nil
	}

	// Default to WebP thumbnail
	path := filepath.Join(ThumbnailsDir, size, "webp", image.UUID, image.Filename)
	return path, "image/webp", nil
}

// GetImagePathForBrowser gibt den optimalen Bildpfad basierend auf den Browser-Fähigkeiten zurück
func GetImagePathForBrowser(c *fiber.Ctx, image *models.Image, size string) string {
	// Bestimme das optimale Format
	format := GetOptimalImageFormat(c, image)

	// Wenn Thumbnails angefordert wurden, aber keine vorhanden sind, verwende das Original
	if (size == "small" || size == "medium") && !HasThumbnailSmall(image) {
		size = ""
	}

	// Wenn das Format nicht unterstützt wird, verwende das Original
	if format == "original" || (!HasWebP(image) && !HasAVIF(image)) {
		// Verwende die Original-Variante
		path, err := GetOriginalPath(image)
		if err != nil {
			log.Error(fmt.Sprintf("Error getting original path: %v", err))
			return "/"
		}
		return "/" + path
	}

	// Sonst gib den Pfad zum optimierten Bild zurück
	var path string
	var err error

	switch {
	case size == "small" && format == "avif":
		path = filepath.Join(ThumbnailsDir, "small", "avif", image.UUID, filepath.Base(image.Filename))
	case size == "small":
		path = filepath.Join(ThumbnailsDir, "small", "webp", image.UUID, filepath.Base(image.Filename))
	case size == "medium" && format == "avif":
		path = filepath.Join(ThumbnailsDir, "medium", "avif", image.UUID, filepath.Base(image.Filename))
	case size == "medium":
		path = filepath.Join(ThumbnailsDir, "medium", "webp", image.UUID, filepath.Base(image.Filename))
	case format == "avif":
		path, err = GetAVIFPath(image)
	case format == "webp":
		path, err = GetWebPPath(image)
	default:
		path, err = GetOriginalPath(image)
	}

	if err != nil {
		log.Error(fmt.Sprintf("Error getting image path: %v", err))
		return "/"
	}

	return "/" + path
}

// getContentTypeFromExtension returns the content type based on file extension
func getContentTypeFromExtension(ext string) string {
	ext = strings.ToLower(ext)
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".avif":
		return "image/avif"
	case ".svg":
		return "image/svg+xml"
	default:
		return "application/octet-stream"
	}
}
