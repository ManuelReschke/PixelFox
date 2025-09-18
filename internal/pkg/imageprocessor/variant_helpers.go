package imageprocessor

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/constants"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
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

	// Build web-accessible paths for each available variant (excluding original)
	for _, variant := range variantInfo.AvailableVariants {
		// Convert storage pool path to web path
		webPath := convertStoragePoolPathToWebPath(variant.FilePath, variant.FileName)

		switch variant.VariantType {
		case "webp":
			paths["webp_full"] = webPath
		case "avif":
			paths["avif_full"] = webPath
		case "thumbnail_small_webp":
			paths["thumbnail_small_webp"] = webPath
		case "thumbnail_small_avif":
			paths["thumbnail_small_avif"] = webPath
		case "thumbnail_small_original":
			paths["thumbnail_small_original"] = webPath
		case "thumbnail_medium_webp":
			paths["thumbnail_medium_webp"] = webPath
		case "thumbnail_medium_avif":
			paths["thumbnail_medium_avif"] = webPath
		case "thumbnail_medium_original":
			paths["thumbnail_medium_original"] = webPath
		}
	}

	return paths
}

// convertStoragePoolPathToWebPath converts a storage pool file path to a web-accessible path
func convertStoragePoolPathToWebPath(filePath, fileName string) string {
	// Construct full path first
	fullPath := filepath.Join(filePath, fileName)

	// Extract the relative path from the full path
	// Remove common storage pool base paths to get web-accessible paths
	webPath := fullPath

	// Find the position of "variants" or "original" in the path
	variantsIndex := strings.Index(webPath, "variants")
	originalIndex := strings.Index(webPath, "original")

	if variantsIndex >= 0 {
		// Extract from "variants" onwards and prepend uploads path
		relativePath := webPath[variantsIndex:]
		webPath = "/" + filepath.Join(constants.UploadsPath, relativePath)
	} else if originalIndex >= 0 {
		// Extract from "original" onwards and prepend uploads path
		relativePath := webPath[originalIndex:]
		webPath = "/" + filepath.Join(constants.UploadsPath, relativePath)
	} else {
		// If neither "variants" nor "original" found, try to remove common base paths
		cleanPath := webPath
		if strings.HasPrefix(cleanPath, "/app/uploads/") {
			cleanPath = strings.TrimPrefix(cleanPath, "/app/uploads/")
		} else if strings.HasPrefix(cleanPath, "/uploads/") {
			cleanPath = strings.TrimPrefix(cleanPath, "/uploads/")
		}
		webPath = "/" + filepath.Join(constants.UploadsPath, cleanPath)
	}

	// Convert to forward slashes for web URLs
	webPath = strings.ReplaceAll(webPath, "\\", "/")

	return webPath
}

// GetPublicBaseURLForImage returns the preferred public base URL for serving an image
// Priority: image.StoragePool.PublicBaseURL -> env PUBLIC_DOMAIN -> empty string
func GetPublicBaseURLForImage(imageModel *models.Image) string {
	if imageModel != nil && imageModel.StoragePool != nil {
		if base := strings.TrimSpace(imageModel.StoragePool.PublicBaseURL); base != "" {
			return strings.TrimRight(base, "/")
		}
	}
	// Fallback to global public domain
	return strings.TrimRight(env.GetEnv("PUBLIC_DOMAIN", ""), "/")
}

// MakeAbsoluteURL joins base URL and a web path ("/uploads/...") safely
func MakeAbsoluteURL(baseURL, webPath string) string {
	if webPath == "" {
		return webPath
	}
	lower := strings.ToLower(webPath)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		// already absolute
		return webPath
	}
	if baseURL == "" {
		return webPath
	}
	// Ensure single slash between base and path
	if !strings.HasPrefix(webPath, "/") {
		webPath = "/" + webPath
	}
	return strings.TrimRight(baseURL, "/") + webPath
}

// MakeAbsoluteForImage prefixes a relative web path with the image's public base URL
func MakeAbsoluteForImage(imageModel *models.Image, webPath string) string {
	base := GetPublicBaseURLForImage(imageModel)
	// If already absolute (http/https), return as-is
	lower := strings.ToLower(webPath)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return webPath
	}
	return MakeAbsoluteURL(base, webPath)
}

// GetImageAbsoluteURL returns an absolute URL for the requested variant based on the image's storage pool
func GetImageAbsoluteURL(imageModel *models.Image, format string, size string) string {
	rel := GetImageURL(imageModel, format, size)
	return MakeAbsoluteForImage(imageModel, rel)
}

// GetBestPreviewURL returns an absolute URL for a suitable preview image.
// Preference order: medium (AVIF -> WebP -> Original), then small (AVIF -> WebP -> Original),
// and finally falls back to the original image URL.
func GetBestPreviewURL(imageModel *models.Image) string {
	if imageModel == nil || imageModel.UUID == "" {
		return ""
	}

	// Try medium thumbnails first
	if p := GetImageURL(imageModel, "avif", "medium"); p != "" {
		return MakeAbsoluteForImage(imageModel, p)
	}
	if p := GetImageURL(imageModel, "webp", "medium"); p != "" {
		return MakeAbsoluteForImage(imageModel, p)
	}
	if p := GetImageURL(imageModel, "original", "medium"); p != "" {
		return MakeAbsoluteForImage(imageModel, p)
	}

	// Fallback to small thumbnails
	if p := GetImageURL(imageModel, "avif", "small"); p != "" {
		return MakeAbsoluteForImage(imageModel, p)
	}
	if p := GetImageURL(imageModel, "webp", "small"); p != "" {
		return MakeAbsoluteForImage(imageModel, p)
	}
	if p := GetImageURL(imageModel, "original", "small"); p != "" {
		return MakeAbsoluteForImage(imageModel, p)
	}

	// Final fallback to original
	return GetImageAbsoluteURL(imageModel, "original", "")
}
