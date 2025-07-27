package imageprocessor

import (
	"fmt"
	"path/filepath"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/gofiber/fiber/v2/log"
)

// VariantInfo holds information about available variants for an image
type VariantInfo struct {
	HasWebP            bool
	HasAVIF            bool
	HasThumbnailSmall  bool
	HasThumbnailMedium bool
	AvailableVariants  []models.ImageVariant
}

// GetImageVariantInfo returns information about available variants for an image
func GetImageVariantInfo(imageID uint) (*VariantInfo, error) {
	db := database.GetDB()
	if db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	variants, err := models.FindVariantsByImageID(db, imageID)
	if err != nil {
		return nil, fmt.Errorf("failed to find variants: %w", err)
	}

	info := &VariantInfo{
		AvailableVariants: variants,
	}

	// Check which variants are available
	for _, variant := range variants {
		switch variant.VariantType {
		case "webp":
			info.HasWebP = true
		case "avif":
			info.HasAVIF = true
		case "thumbnail_small_webp", "thumbnail_small_avif", "thumbnail_small_original":
			info.HasThumbnailSmall = true
		case "thumbnail_medium_webp", "thumbnail_medium_avif", "thumbnail_medium_original":
			info.HasThumbnailMedium = true
		}
	}

	return info, nil
}

// HasVariantType checks if a specific variant type exists for an image
func HasVariantType(imageID uint, variantType string) bool {
	db := database.GetDB()
	if db == nil {
		log.Error("[HasVariantType] Database connection is nil")
		return false
	}

	return models.HasVariant(db, imageID, variantType)
}

// GetVariantPath returns the full path for a specific variant type
func GetVariantPath(imageID uint, variantType string) string {
	db := database.GetDB()
	if db == nil {
		log.Error("[GetVariantPath] Database connection is nil")
		return ""
	}

	variant, err := models.FindVariantByImageIDAndType(db, imageID, variantType)
	if err != nil {
		log.Debugf("[GetVariantPath] Variant '%s' not found for image ID %d: %v", variantType, imageID, err)
		return ""
	}

	return filepath.Join(variant.FilePath, variant.FileName)
}

// GetOptimalImagePath returns the best available image path based on preferences
func GetOptimalImagePath(imageModel *models.Image, preferredFormats []string, preferredSize string) string {
	if imageModel == nil {
		return ""
	}

	// Try each preferred format in order
	for _, format := range preferredFormats {
		path := GetImagePath(imageModel, format, preferredSize)
		if path != "" {
			return path
		}
	}

	// Fallback to original
	return GetImagePath(imageModel, "original", "")
}

// BuildImagePaths builds all possible image paths for an image
func BuildImagePaths(imageModel *models.Image) map[string]string {
	paths := make(map[string]string)

	if imageModel == nil {
		return paths
	}

	// Get all variants for this image
	variantInfo, err := GetImageVariantInfo(imageModel.ID)
	if err != nil {
		log.Errorf("[BuildImagePaths] Failed to get variant info for image %s: %v", imageModel.UUID, err)
		return paths
	}

	// Add original path from images table (not from variants anymore)
	if imageModel.FilePath != "" && imageModel.FileName != "" {
		paths["original"] = filepath.Join(imageModel.FilePath, imageModel.FileName)
	}

	// Build paths for each available variant (excluding original)
	for _, variant := range variantInfo.AvailableVariants {
		switch variant.VariantType {
		case "webp":
			paths["webp_full"] = filepath.Join(variant.FilePath, variant.FileName)
		case "avif":
			paths["avif_full"] = filepath.Join(variant.FilePath, variant.FileName)
		case "thumbnail_small_webp":
			paths["thumbnail_small_webp"] = filepath.Join(variant.FilePath, variant.FileName)
		case "thumbnail_small_avif":
			paths["thumbnail_small_avif"] = filepath.Join(variant.FilePath, variant.FileName)
		case "thumbnail_small_original":
			paths["thumbnail_small_original"] = filepath.Join(variant.FilePath, variant.FileName)
		case "thumbnail_medium_webp":
			paths["thumbnail_medium_webp"] = filepath.Join(variant.FilePath, variant.FileName)
		case "thumbnail_medium_avif":
			paths["thumbnail_medium_avif"] = filepath.Join(variant.FilePath, variant.FileName)
		case "thumbnail_medium_original":
			paths["thumbnail_medium_original"] = filepath.Join(variant.FilePath, variant.FileName)
		}
	}

	return paths
}
