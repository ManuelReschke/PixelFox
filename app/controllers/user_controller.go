package controllers

import (
	"path/filepath"
	"strconv"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/imageprocessor"
	"github.com/ManuelReschke/PixelFox/internal/pkg/session"
	user_views "github.com/ManuelReschke/PixelFox/views/user"
	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/sujit-baniya/flash"
)

func HandleUserProfile(c *fiber.Ctx) error {
	sess, _ := session.GetSessionStore().Get(c)
	_ = sess.Get(USER_ID) // Using _ to avoid unused variable warning
	username := sess.Get(USER_NAME).(string)
	isAdmin := sess.Get(USER_IS_ADMIN).(bool)

	csrfToken := c.Locals("csrf").(string)

	profileIndex := user_views.ProfileIndex(username, csrfToken)
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
	result := database.DB.Where("user_id = ?", userID).Order("created_at DESC").Find(&images)
	if result.Error != nil {
		// Fehler beim Laden der Bilder
		flash.WithError(c, fiber.Map{"message": "Fehler beim Laden der Bilder: " + result.Error.Error()})
		return c.Redirect("/")
	}

	// Bereite die Bilderpfade für die Galerie vor
	var galleryImages []user_views.GalleryImage
	for _, img := range images {
		previewPath := ""
		if img.HasThumbnailSmall {
			// Verwende WebP wenn verfügbar, sonst AVIF, sonst Original
			if img.HasWebp {
				previewPath = "/" + imageprocessor.GetImagePath(&img, "webp", "medium")
			} else if img.HasAVIF {
				previewPath = "/" + imageprocessor.GetImagePath(&img, "avif", "medium")
			} else {
				// Fallback zum Original
				previewPath = filepath.Join("/", img.FilePath, img.FileName)
			}
		} else {
			// Wenn keine Thumbnails verfügbar sind, verwende das Original
			previewPath = filepath.Join("/", img.FilePath, img.FileName)
		}

		// Titel bestimmen
		title := img.FileName
		if img.Title != "" {
			title = img.Title
		}

		galleryImages = append(galleryImages, user_views.GalleryImage{
			ID:          img.ID,
			UUID:        img.UUID,
			Title:       title,
			ShareLink:   img.ShareLink,
			PreviewPath: previewPath,
			CreatedAt:   img.CreatedAt.Format("02.01.2006 15:04"),
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

	const imagesPerPage = 20
	offset := (page - 1) * imagesPerPage

	var images []models.Image
	result := database.DB.Where("user_id = ?", userID).Order("created_at DESC").Offset(offset).Limit(imagesPerPage).Find(&images)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Fehler beim Laden der Bilder")
	}

	var galleryImages []user_views.GalleryImage
	for _, img := range images {
		previewPath := ""
		if img.HasThumbnailSmall {
			if img.HasWebp {
				previewPath = "/" + imageprocessor.GetImagePath(&img, "webp", "medium")
			} else if img.HasAVIF {
				previewPath = "/" + imageprocessor.GetImagePath(&img, "avif", "medium")
			} else {
				previewPath = filepath.Join("/", img.FilePath, img.FileName)
			}
		} else {
			previewPath = filepath.Join("/", img.FilePath, img.FileName)
		}

		title := img.FileName
		if img.Title != "" {
			title = img.Title
		}

		galleryImages = append(galleryImages, user_views.GalleryImage{
			ID:          img.ID,
			UUID:        img.UUID,
			Title:       title,
			ShareLink:   img.ShareLink,
			PreviewPath: previewPath,
			CreatedAt:   img.CreatedAt.Format("02.01.2006 15:04"),
		})
	}

	return user_views.GalleryItems(galleryImages, page).Render(c.Context(), c.Response().BodyWriter())
}
