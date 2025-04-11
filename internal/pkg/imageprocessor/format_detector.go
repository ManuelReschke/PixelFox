package imageprocessor

import (
	"path/filepath"
	"strings"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/gofiber/fiber/v2"
)

// GetOptimalImageFormat bestimmt das optimale Bildformat basierend auf dem Accept-Header des Browsers
func GetOptimalImageFormat(c *fiber.Ctx, image *models.Image) string {
	// Prüfe den Accept-Header
	acceptHeader := c.Get("Accept")

	// Prüfe, ob der Browser AVIF unterstützt
	if strings.Contains(acceptHeader, "image/avif") && image.HasAVIF {
		return "avif"
	}

	// Prüfe, ob der Browser WebP unterstützt
	if strings.Contains(acceptHeader, "image/webp") && image.HasWebp {
		return "webp"
	}

	// Fallback auf das Originalformat
	return "original"
}

// GetImagePathForBrowser gibt den optimalen Bildpfad basierend auf den Browser-Fähigkeiten zurück
func GetImagePathForBrowser(c *fiber.Ctx, image *models.Image, size string) string {
	// Bestimme das optimale Format
	format := GetOptimalImageFormat(c, image)

	// Wenn Thumbnails angefordert wurden, aber keine vorhanden sind, verwende das Original
	if (size == "small" || size == "medium") && !image.HasThumbnailSmall {
		size = ""
	}

	// Wenn das Format nicht unterstützt wird, verwende das Original
	if format == "original" || (!image.HasWebp && !image.HasAVIF) {
		// Extrahiere Dateiinformationen
		relativePath := strings.TrimPrefix(image.FilePath, "./")
		return "/" + filepath.Join(relativePath, image.FileName)
	}

	// Sonst gib den Pfad zum optimierten Bild zurück
	return "/" + GetImagePath(image, format, size)
}
