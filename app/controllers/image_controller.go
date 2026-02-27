package controllers

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	fiberlog "github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/sujit-baniya/flash"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/app/repository"
	"github.com/ManuelReschke/PixelFox/internal/pkg/imageprocessor"
	metrics "github.com/ManuelReschke/PixelFox/internal/pkg/metrics/counter"
	"github.com/ManuelReschke/PixelFox/internal/pkg/usercontext"
	"github.com/ManuelReschke/PixelFox/internal/pkg/viewmodel"
	"github.com/ManuelReschke/PixelFox/views"
)

// formatBytes formats a byte size into a human-readable string like "900 KB" or "1.2 MB"
func formatBytes(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
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

	// Get current user ID from UserContext
	userCtx := usercontext.GetUserContext(c)
	var currentUserID uint = 0
	if userCtx.IsLoggedIn {
		currentUserID = userCtx.UserID
	}

	// Determine the preferred public base URL for this image's storage pool
	domain := imageprocessor.GetPublicBaseURLForImage(image)

	// Increase the view counter
	imageRepo.UpdateViewCount(image.ID)
	// Touch last viewed at (Redis -> periodic DB flush)
	_ = metrics.AddImageLastViewed(image.ID)

	// Korrekte URL-Konstruktion (absolut) für das Original-Bild
	filePathComplete := imageprocessor.GetImageURL(image, "original", "")
	fiberlog.Debugf("[ImageController] Original-Pfad: %s", filePathComplete)
	filePathWithDomain := imageprocessor.MakeAbsoluteURL(domain, filePathComplete)

	// Share-Seite bleibt auf App-Domain
	shareURL := fmt.Sprintf("%s/i/%s", c.BaseURL(), image.ShareLink)

	// Get variant information for this image
	variantInfo, err := imageprocessor.GetImageVariantInfo(image.ID)
	if err != nil {
		variantInfo = &imageprocessor.VariantInfo{} // fallback to empty
	}

	// Build maps of variant type -> size (human + bytes)
	sizeMap := make(map[string]string)
	bytesMap := make(map[string]int64)
	// Original size comes from images table
	sizeMap[models.VariantTypeOriginal] = formatBytes(image.FileSize)
	bytesMap[models.VariantTypeOriginal] = image.FileSize
	for _, v := range variantInfo.AvailableVariants {
		sizeMap[v.VariantType] = formatBytes(v.FileSize)
		bytesMap[v.VariantType] = v.FileSize
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

	// Keep variant paths relative for copy boxes and prefix in templates for display

	// Prepare Open Graph meta tags
	ogImage := ""
	if variantInfo.HasThumbnailSmall {
		// Use small thumbnail for OG tags
		if smallThumbAvifPath != "" {
			ogImage = imageprocessor.MakeAbsoluteURL(domain, smallThumbAvifPath)
		} else if smallThumbWebpPath != "" {
			ogImage = imageprocessor.MakeAbsoluteURL(domain, smallThumbWebpPath)
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

	isAdmin := userCtx.IsAdmin

	// Keep variant paths relative for copy boxes; templates prepend Domain where needed

	// Create the ImageViewModel
	imageModel := viewmodel.Image{
		// Domain liefert Base-URL für Kopier-Boxen
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
		// Sizes for tabs
		OptimizedOriginalSize: sizeMap[models.VariantTypeOriginal],
		OptimizedWebPSize:     sizeMap[models.VariantTypeWebP],
		OptimizedAVIFSize:     sizeMap[models.VariantTypeAVIF],
		MediumOriginalSize:    sizeMap[models.VariantTypeThumbnailMediumOrig],
		MediumWebPSize:        sizeMap[models.VariantTypeThumbnailMediumWebP],
		MediumAVIFSize:        sizeMap[models.VariantTypeThumbnailMediumAVIF],
		SmallOriginalSize:     sizeMap[models.VariantTypeThumbnailSmallOrig],
		SmallWebPSize:         sizeMap[models.VariantTypeThumbnailSmallWebP],
		SmallAVIFSize:         sizeMap[models.VariantTypeThumbnailSmallAVIF],
		// Byte sizes for calculations
		OptimizedOriginalBytes: bytesMap[models.VariantTypeOriginal],
		OptimizedWebPBytes:     bytesMap[models.VariantTypeWebP],
		OptimizedAVIFBytes:     bytesMap[models.VariantTypeAVIF],
		MediumOriginalBytes:    bytesMap[models.VariantTypeThumbnailMediumOrig],
		MediumWebPBytes:        bytesMap[models.VariantTypeThumbnailMediumWebP],
		MediumAVIFBytes:        bytesMap[models.VariantTypeThumbnailMediumAVIF],
		SmallOriginalBytes:     bytesMap[models.VariantTypeThumbnailSmallOrig],
		SmallWebPBytes:         bytesMap[models.VariantTypeThumbnailSmallWebP],
		SmallAVIFBytes:         bytesMap[models.VariantTypeThumbnailSmallAVIF],
	}

	imageViewer := views.ImageViewerWithUser(imageModel, currentUserID, image.UserID)

	ogViewModel := &viewmodel.OpenGraph{
		URL:         shareURL,
		Image:       ogImage,
		ImageAlt:    ogTitle,
		Title:       ogTitle,
		Description: ogDescription,
	}

	home := views.HomeCtx(c, fmt.Sprintf("| Bild %s ansehen", imageModel.DisplayName), userCtx.IsLoggedIn, false, flash.Get(c), imageViewer, isAdmin, ogViewModel)

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

	// Get current user ID from UserContext
	userCtx := usercontext.GetUserContext(c)
	var currentUserID uint = 0
	if userCtx.IsLoggedIn {
		currentUserID = userCtx.UserID
	}

	// Check if any optimized versions are available (for Ajax response)
	variantInfoAjax, err := imageprocessor.GetImageVariantInfo(image.ID)
	if err != nil {
		variantInfoAjax = &imageprocessor.VariantInfo{} // fallback to empty
	}
	hasOptimizedVersions := variantInfoAjax.HasWebP || variantInfoAjax.HasAVIF || variantInfoAjax.HasThumbnailSmall || variantInfoAjax.HasThumbnailMedium

	// Build maps of variant type -> size (human + bytes)
	sizeMap := make(map[string]string)
	bytesMap := make(map[string]int64)
	// Original size comes from images table
	sizeMap[models.VariantTypeOriginal] = formatBytes(image.FileSize)
	bytesMap[models.VariantTypeOriginal] = image.FileSize
	for _, v := range variantInfoAjax.AvailableVariants {
		sizeMap[v.VariantType] = formatBytes(v.FileSize)
		bytesMap[v.VariantType] = v.FileSize
	}

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

	// Base URL for storage domain and relative original path
	base := imageprocessor.GetPublicBaseURLForImage(image)
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

	// Create a simplified view model for image display only (use relative paths; templates prefix with Domain)
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
		Domain:               base,
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
		// Sizes for tabs
		OptimizedOriginalSize: sizeMap[models.VariantTypeOriginal],
		OptimizedWebPSize:     sizeMap[models.VariantTypeWebP],
		OptimizedAVIFSize:     sizeMap[models.VariantTypeAVIF],
		MediumOriginalSize:    sizeMap[models.VariantTypeThumbnailMediumOrig],
		MediumWebPSize:        sizeMap[models.VariantTypeThumbnailMediumWebP],
		MediumAVIFSize:        sizeMap[models.VariantTypeThumbnailMediumAVIF],
		SmallOriginalSize:     sizeMap[models.VariantTypeThumbnailSmallOrig],
		SmallWebPSize:         sizeMap[models.VariantTypeThumbnailSmallWebP],
		SmallAVIFSize:         sizeMap[models.VariantTypeThumbnailSmallAVIF],
		// Byte sizes for calculations
		OptimizedOriginalBytes: bytesMap[models.VariantTypeOriginal],
		OptimizedWebPBytes:     bytesMap[models.VariantTypeWebP],
		OptimizedAVIFBytes:     bytesMap[models.VariantTypeAVIF],
		MediumOriginalBytes:    bytesMap[models.VariantTypeThumbnailMediumOrig],
		MediumWebPBytes:        bytesMap[models.VariantTypeThumbnailMediumWebP],
		MediumAVIFBytes:        bytesMap[models.VariantTypeThumbnailMediumAVIF],
		SmallOriginalBytes:     bytesMap[models.VariantTypeThumbnailSmallOrig],
		SmallWebPBytes:         bytesMap[models.VariantTypeThumbnailSmallWebP],
		SmallAVIFBytes:         bytesMap[models.VariantTypeThumbnailSmallAVIF],
	}

	// Render the entire card with the ImageViewer
	return views.ImageViewerWithUser(imageModel, currentUserID, image.UserID).Render(c.Context(), c.Response().BodyWriter())
}

// Image processing is handled via jobqueue.ProcessImageUnified().
