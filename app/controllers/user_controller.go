package controllers

import (
	"fmt"
	"strconv"

	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/sujit-baniya/flash"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/imageprocessor"
	"github.com/ManuelReschke/PixelFox/internal/pkg/jobqueue"
	"github.com/ManuelReschke/PixelFox/internal/pkg/s3backup"
	"github.com/ManuelReschke/PixelFox/internal/pkg/session"
	"github.com/ManuelReschke/PixelFox/views"
	user_views "github.com/ManuelReschke/PixelFox/views/user"
)

func HandleUserProfile(c *fiber.Ctx) error {
	sess, _ := session.GetSessionStore().Get(c)
	userID := sess.Get(USER_ID).(uint)
	username := sess.Get(USER_NAME).(string)
	isAdmin := sess.Get(USER_IS_ADMIN).(bool)

	// Get user data from database
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		flash.WithError(c, fiber.Map{"message": "User not found"})
		return c.Redirect("/")
	}

	// Get user statistics
	var imageCount int64
	database.DB.Model(&models.Image{}).Where("user_id = ?", userID).Count(&imageCount)

	var albumCount int64
	database.DB.Model(&models.Album{}).Where("user_id = ?", userID).Count(&albumCount)

	// Calculate storage usage
	var totalStorage int64
	database.DB.Model(&models.Image{}).Where("user_id = ?", userID).Select("SUM(file_size)").Row().Scan(&totalStorage)

	csrfToken := c.Locals("csrf").(string)

	profileIndex := user_views.ProfileIndex(username, csrfToken, user, int(imageCount), int(albumCount), int64(totalStorage))
	profile := user_views.Profile(
		" | Profil", isLoggedIn(c), false, flash.Get(c), username, profileIndex, isAdmin,
	)

	handler := adaptor.HTTPHandler(templ.Handler(profile))

	return handler(c)
}

func HandleUserSettings(c *fiber.Ctx) error {
	sess, _ := session.GetSessionStore().Get(c)
	_ = sess.Get(USER_ID) // Using _ to avoid unused variable warning
	username := sess.Get(USER_NAME).(string)
	isAdmin := sess.Get(USER_IS_ADMIN).(bool)

	csrfToken := c.Locals("csrf").(string)

	settingsIndex := user_views.SettingsIndex(username, csrfToken)
	settings := user_views.Settings(
		" | Einstellungen", isLoggedIn(c), false, flash.Get(c), username, settingsIndex, isAdmin,
	)

	handler := adaptor.HTTPHandler(templ.Handler(settings))

	return handler(c)
}

func HandleUserImages(c *fiber.Ctx) error {
	sess, _ := session.GetSessionStore().Get(c)
	userID := sess.Get(USER_ID).(uint)
	username := sess.Get(USER_NAME).(string)
	isAdmin := sess.Get(USER_IS_ADMIN).(bool)

	var images []models.Image
	result := database.DB.Preload("StoragePool").Where("user_id = ?", userID).Order("created_at DESC").Find(&images)
	if result.Error != nil {
		// Fehler beim Laden der Bilder
		flash.WithError(c, fiber.Map{"message": "Fehler beim Laden der Bilder: " + result.Error.Error()})
		return c.Redirect("/")
	}

	// Bereite die Bilderpfade für die Galerie vor
	var galleryImages []user_views.GalleryImage
	for _, img := range images {
		previewPath := ""
		// Get variant info for this image
		variantInfo, err := imageprocessor.GetImageVariantInfo(img.ID)
		if err != nil {
			variantInfo = &imageprocessor.VariantInfo{} // fallback to empty
		}

		// Try medium thumbnails first
		if variantInfo.HasThumbnailMedium {
			// Priority: AVIF -> WebP -> Original format
			if avifPath := imageprocessor.GetImageURL(&img, "avif", "medium"); avifPath != "" {
				previewPath = avifPath
			} else if webpPath := imageprocessor.GetImageURL(&img, "webp", "medium"); webpPath != "" {
				previewPath = webpPath
			} else if originalPath := imageprocessor.GetImageURL(&img, "original", "medium"); originalPath != "" {
				previewPath = originalPath
			}
		}

		// Fallback to small thumbnails if medium not available
		if previewPath == "" && variantInfo.HasThumbnailSmall {
			// Priority: AVIF -> WebP -> Original format
			if avifPath := imageprocessor.GetImageURL(&img, "avif", "small"); avifPath != "" {
				previewPath = avifPath
			} else if webpPath := imageprocessor.GetImageURL(&img, "webp", "small"); webpPath != "" {
				previewPath = webpPath
			} else if originalPath := imageprocessor.GetImageURL(&img, "original", "small"); originalPath != "" {
				previewPath = originalPath
			}
		}

		// Final fallback to original image
		if previewPath == "" {
			previewPath = imageprocessor.GetImageURL(&img, "original", "")
		}

		title := img.FileName
		if img.Title != "" {
			title = img.Title
		}

		originalPath := imageprocessor.GetImageURL(&img, "original", "")
		galleryImages = append(galleryImages, user_views.GalleryImage{
			ID:           img.ID,
			UUID:         img.UUID,
			Title:        title,
			ShareLink:    img.ShareLink,
			PreviewPath:  previewPath,
			OriginalPath: originalPath,
			CreatedAt:    img.CreatedAt.Format("02.01.2006 15:04"),
		})
	}

	imagesGallery := user_views.ImagesGallery(username, galleryImages)
	imagesPage := user_views.Images(
		" | Meine Bilder", isLoggedIn(c), false, flash.Get(c), username, imagesGallery, isAdmin,
	)

	handler := adaptor.HTTPHandler(templ.Handler(imagesPage))

	return handler(c)
}

func HandleLoadMoreImages(c *fiber.Ctx) error {
	sess, _ := session.GetSessionStore().Get(c)
	userID := sess.Get(USER_ID).(uint)

	page, err := strconv.Atoi(c.Query("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}

	const imagesPerPage = 25
	offset := (page - 1) * imagesPerPage

	var images []models.Image
	result := database.DB.Preload("StoragePool").Where("user_id = ?", userID).Order("created_at DESC").Offset(offset).Limit(imagesPerPage).Find(&images)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Fehler beim Laden der Bilder")
	}

	var galleryImages []user_views.GalleryImage
	for _, img := range images {
		previewPath := ""
		// Get variant info for this image
		variantInfo, err := imageprocessor.GetImageVariantInfo(img.ID)
		if err != nil {
			variantInfo = &imageprocessor.VariantInfo{} // fallback to empty
		}

		// Try medium thumbnails first
		if variantInfo.HasThumbnailMedium {
			// Priority: AVIF -> WebP -> Original format
			if avifPath := imageprocessor.GetImageURL(&img, "avif", "medium"); avifPath != "" {
				previewPath = avifPath
			} else if webpPath := imageprocessor.GetImageURL(&img, "webp", "medium"); webpPath != "" {
				previewPath = webpPath
			} else if originalPath := imageprocessor.GetImageURL(&img, "original", "medium"); originalPath != "" {
				previewPath = originalPath
			}
		}

		// Fallback to small thumbnails if medium not available
		if previewPath == "" && variantInfo.HasThumbnailSmall {
			// Priority: AVIF -> WebP -> Original format
			if avifPath := imageprocessor.GetImageURL(&img, "avif", "small"); avifPath != "" {
				previewPath = avifPath
			} else if webpPath := imageprocessor.GetImageURL(&img, "webp", "small"); webpPath != "" {
				previewPath = webpPath
			} else if originalPath := imageprocessor.GetImageURL(&img, "original", "small"); originalPath != "" {
				previewPath = originalPath
			}
		}

		// Final fallback to original image
		if previewPath == "" {
			previewPath = imageprocessor.GetImageURL(&img, "original", "")
		}

		title := img.FileName
		if img.Title != "" {
			title = img.Title
		}

		originalPath := imageprocessor.GetImageURL(&img, "original", "")
		galleryImages = append(galleryImages, user_views.GalleryImage{
			ID:           img.ID,
			UUID:         img.UUID,
			Title:        title,
			ShareLink:    img.ShareLink,
			PreviewPath:  previewPath,
			OriginalPath: originalPath,
			CreatedAt:    img.CreatedAt.Format("02.01.2006 15:04"),
		})
	}

	return user_views.GalleryItems(galleryImages, page).Render(c.Context(), c.Response().BodyWriter())
}

// HandleUserImageEdit allows users to edit their own images
func HandleUserImageEdit(c *fiber.Ctx) error {
	sess, _ := session.GetSessionStore().Get(c)
	userID := sess.Get(USER_ID).(uint)
	uuid := c.Params("uuid")
	if uuid == "" {
		return c.Redirect("/user/images")
	}
	db := database.GetDB()
	image, err := models.FindImageByUUID(db, uuid)
	if err != nil || image.UserID != userID {
		flash.WithError(c, fiber.Map{"type": "error", "message": "Bild nicht gefunden"})
		return c.Redirect("/user/images")
	}
	csrfToken := c.Locals("csrf").(string)
	userEdit := user_views.UserImageEdit(*image, csrfToken)
	page := views.Home(fmt.Sprintf("| Bild %s bearbeiten", image.Title), isLoggedIn(c), false, flash.Get(c), userEdit, sess.Get(USER_IS_ADMIN).(bool), nil)
	handler := adaptor.HTTPHandler(templ.Handler(page))
	return handler(c)
}

// HandleUserImageUpdate processes the edit form
func HandleUserImageUpdate(c *fiber.Ctx) error {
	sess, _ := session.GetSessionStore().Get(c)
	userID := sess.Get(USER_ID).(uint)
	uuid := c.Params("uuid")
	if uuid == "" {
		return c.Redirect("/user/images")
	}
	db := database.GetDB()
	image, err := models.FindImageByUUID(db, uuid)
	if err != nil || image.UserID != userID {
		flash.WithError(c, fiber.Map{"type": "error", "message": "Bild nicht gefunden"})
		return c.Redirect("/user/images")
	}
	title := c.FormValue("title")
	description := c.FormValue("description")
	isPublic := c.FormValue("is_public") == "on"

	image.Title = title
	image.Description = description
	image.IsPublic = isPublic

	db.Save(image)
	flash.WithSuccess(c, fiber.Map{"type": "success", "message": "Bild aktualisiert"})
	return c.Redirect("/user/images")
}

// HandleUserImageDelete removes user's image and all variants
func HandleUserImageDelete(c *fiber.Ctx) error {
	sess, _ := session.GetSessionStore().Get(c)
	userID := sess.Get(USER_ID).(uint)
	uuid := c.Params("uuid")
	if uuid == "" {
		return c.Redirect("/user/images")
	}
	db := database.GetDB()
	image, err := models.FindImageByUUID(db, uuid)
	if err != nil || image.UserID != userID {
		flash.WithError(c, fiber.Map{"type": "error", "message": "Bild nicht gefunden"})
		return c.Redirect("/user/images")
	}

	// Enqueue S3 delete jobs for completed backups before deleting the image
	go func() {
		enqueueUserS3DeleteJobsIfEnabled(image)
	}()

	// Use the new function to delete image and all variants (files and database records)
	if err := imageprocessor.DeleteImageAndVariants(image); err != nil {
		flash.WithError(c, fiber.Map{"type": "error", "message": "Fehler beim Löschen des Bildes"})
		return c.Redirect("/user/images")
	}

	flash.WithSuccess(c, fiber.Map{"type": "success", "message": "Bild und alle Varianten gelöscht"})
	return c.Redirect("/user/images")
}

// enqueueUserS3DeleteJobsIfEnabled creates S3 delete jobs for completed backups if S3 backup is enabled
func enqueueUserS3DeleteJobsIfEnabled(image *models.Image) {
	// Check if S3 backup is enabled
	config, err := s3backup.LoadConfig()
	if err != nil {
		fmt.Printf("[S3Delete] Failed to load S3 config: %v\n", err)
		return
	}

	if !config.IsEnabled() {
		fmt.Printf("[S3Delete] S3 backup disabled, skipping delete for image %s\n", image.UUID)
		return
	}

	db := database.GetDB()
	if db == nil {
		fmt.Printf("[S3Delete] Database connection is nil\n")
		return
	}

	// Find all completed backups for this image
	backups, err := models.FindCompletedBackupsByImageID(db, image.ID)
	if err != nil {
		fmt.Printf("[S3Delete] Failed to find backups for image %d: %v\n", image.ID, err)
		return
	}

	if len(backups) == 0 {
		fmt.Printf("[S3Delete] No completed backups found for image %s\n", image.UUID)
		return
	}

	// Get job queue from manager
	queue := jobqueue.GetManager().GetQueue()

	// Enqueue delete jobs for each completed backup
	for _, backup := range backups {
		job, err := queue.EnqueueS3DeleteJob(
			image.ID,
			image.UUID,
			backup.ObjectKey,
			backup.BucketName,
			backup.ID,
		)
		if err != nil {
			fmt.Printf("[S3Delete] Failed to enqueue delete job for backup %d: %v\n", backup.ID, err)
			continue
		}

		fmt.Printf("[S3Delete] Successfully enqueued delete job %s for image %s\n", job.ID, image.UUID)
	}
}
