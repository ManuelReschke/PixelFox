package imageprocessor

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gofiber/fiber/v2/log"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
)

// Variant types
const (
	VariantTypeOriginal        = "original"
	VariantTypeWebP            = "webp"
	VariantTypeAVIF            = "avif"
	VariantTypeThumbnailSmall  = "thumb_small"
	VariantTypeThumbnailMedium = "thumb_medium"
)

// SaveVariant creates and saves a new image variant record
func SaveVariant(image *models.Image, variantType string, path string, width, height int, fileSize int64, quality int) error {
	db := database.GetDB()

	// Check if variant already exists
	var existingVariant models.ImageVariant
	result := db.Where("image_id = ? AND variant_type = ?", image.ID, variantType).First(&existingVariant)

	if result.Error == nil {
		// Update existing variant
		existingVariant.Path = path
		existingVariant.Width = &width
		existingVariant.Height = &height
		existingVariant.FileSize = &fileSize
		existingVariant.Quality = &quality
		return db.Save(&existingVariant).Error
	}

	// Create new variant
	variant := models.ImageVariant{
		ImageID:     image.ID,
		VariantType: variantType,
		Path:        path,
		Width:       &width,
		Height:      &height,
		FileSize:    &fileSize,
		Quality:     &quality,
	}

	return db.Create(&variant).Error
}

// GetVariant retrieves a specific variant for an image
func GetVariant(image *models.Image, variantType string) (*models.ImageVariant, error) {
	db := database.GetDB()
	var variant models.ImageVariant

	result := db.Where("image_id = ? AND variant_type = ?", image.ID, variantType).First(&variant)
	if result.Error != nil {
		return nil, result.Error
	}

	return &variant, nil
}

// HasVariant checks if an image has a specific variant
func HasVariant(image *models.Image, variantType string) bool {
	_, err := GetVariant(image, variantType)
	return err == nil
}

// GetVariantPath returns the path for a specific variant
func GetVariantPath(image *models.Image, variantType string) (string, error) {
	variant, err := GetVariant(image, variantType)
	if err != nil {
		return "", err
	}

	return variant.Path, nil
}

// DeleteVariant removes a specific variant
func DeleteVariant(image *models.Image, variantType string) error {
	db := database.GetDB()
	variant, err := GetVariant(image, variantType)
	if err != nil {
		return nil // Variant doesn't exist, nothing to delete
	}

	// Delete the file
	err = os.Remove(variant.Path)
	if err != nil && !os.IsNotExist(err) {
		log.Warn(fmt.Sprintf("Failed to delete variant file %s: %v", variant.Path, err))
	}

	// Delete the database record
	return db.Delete(&variant).Error
}

// GetOriginalPath gibt den Pfad zur Original-Variante zurück
func GetOriginalPath(image *models.Image) (string, error) {
	for _, variant := range image.Variants {
		if variant.VariantType == "original" {
			return variant.Path, nil
		}
	}
	return "", fmt.Errorf("original variant not found for image %s", image.UUID)
}

// GetWebPPath gibt den Pfad zur WebP-Variante zurück
func GetWebPPath(image *models.Image) (string, error) {
	for _, variant := range image.Variants {
		if variant.VariantType == "webp" {
			return variant.Path, nil
		}
	}
	return "", fmt.Errorf("webp variant not found for image %s", image.UUID)
}

// GetAVIFPath gibt den Pfad zur AVIF-Variante zurück
func GetAVIFPath(image *models.Image) (string, error) {
	for _, variant := range image.Variants {
		if variant.VariantType == "avif" {
			return variant.Path, nil
		}
	}
	return "", fmt.Errorf("avif variant not found for image %s", image.UUID)
}

// GetThumbnailSmallPath gibt den Pfad zum kleinen Thumbnail zurück
func GetThumbnailSmallPath(image *models.Image) (string, error) {
	for _, variant := range image.Variants {
		if variant.VariantType == "thumbnail_small" {
			return variant.Path, nil
		}
	}
	return "", fmt.Errorf("small thumbnail variant not found for image %s", image.UUID)
}

// GetThumbnailMediumPath gibt den Pfad zum mittleren Thumbnail zurück
func GetThumbnailMediumPath(image *models.Image) (string, error) {
	for _, variant := range image.Variants {
		if variant.VariantType == "thumbnail_medium" {
			return variant.Path, nil
		}
	}
	return "", fmt.Errorf("medium thumbnail variant not found for image %s", image.UUID)
}

// HasProcessedVariants prüft, ob ein Bild bereits verarbeitete Varianten hat
func HasProcessedVariants(image *models.Image) bool {
	return len(image.Variants) > 0
}

// HasAVIF prüft, ob ein Bild eine AVIF-Variante hat
func HasAVIF(image *models.Image) bool {
	for _, variant := range image.Variants {
		if variant.VariantType == "avif" {
			return true
		}
	}
	return false
}

// HasWebP prüft, ob ein Bild eine WebP-Variante hat
func HasWebP(image *models.Image) bool {
	for _, variant := range image.Variants {
		if variant.VariantType == "webp" {
			return true
		}
	}
	return false
}

// HasThumbnailSmall prüft, ob ein Bild eine kleine Thumbnail-Variante hat
func HasThumbnailSmall(image *models.Image) bool {
	for _, variant := range image.Variants {
		if variant.VariantType == "thumbnail_small" {
			return true
		}
	}
	return false
}

// HasThumbnailMedium prüft, ob ein Bild eine mittlere Thumbnail-Variante hat
func HasThumbnailMedium(image *models.Image) bool {
	for _, variant := range image.Variants {
		if variant.VariantType == "thumbnail_medium" {
			return true
		}
	}
	return false
}

// GetImagePathForVariant gibt den Pfad zu einer bestimmten Variante zurück
func GetImagePathForVariant(image *models.Image, variantType string) string {
	// Für Original-Variante
	if variantType == "original" {
		for _, variant := range image.Variants {
			if variant.VariantType == "original" {
				return "/image/serve/" + image.UUID
			}
		}
	}

	// Für WebP-Variante
	if variantType == "webp" {
		for _, variant := range image.Variants {
			if variant.VariantType == "webp" {
				return "/image/serve/" + image.UUID
			}
		}
	}

	// Für AVIF-Variante
	if variantType == "avif" {
		for _, variant := range image.Variants {
			if variant.VariantType == "avif" {
				return "/image/serve/" + image.UUID
			}
		}
	}

	// Fallback zur Original-Variante
	return "/image/serve/" + image.UUID
}

// GetImagePathByFilename gibt den Pfad zu einer Datei basierend auf UUID und Dateiname zurück
func GetImagePathByFilename(uuid, filename string) string {
	return filepath.Join(OriginalDir, uuid, filename)
}

// GetImagePathWithSize ist eine Kompatibilitätsfunktion für ältere Aufrufe
func GetImagePathWithSize(image *models.Image, format string, size string) string {
	// Diese Funktion bleibt für Kompatibilität mit älterem Code erhalten
	if size == "small" {
		return "/image/thumbnail/" + image.UUID + "/small"
	} else if size == "medium" {
		return "/image/thumbnail/" + image.UUID + "/medium"
	}

	// Fallback zur Original-Variante
	return "/image/serve/" + image.UUID
}

// SaveOriginalVariant saves the original image as a variant
func SaveOriginalVariant(image *models.Image, path string, width, height int, fileSize int64) error {
	return SaveVariant(image, VariantTypeOriginal, path, width, height, fileSize, 100)
}
