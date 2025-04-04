package controllers

import (
	"path/filepath"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/imageprocessor"
	"github.com/ManuelReschke/PixelFox/internal/pkg/session"
	userviews "github.com/ManuelReschke/PixelFox/views/user"
	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/sujit-baniya/flash"
)

func HandleUserProfile(c *fiber.Ctx) error {
	// Get user information from session
	sess, _ := session.GetSessionStore().Get(c)
	_ = sess.Get(USER_ID) // Using _ to avoid unused variable warning
	username := sess.Get(USER_NAME).(string)

	// Get CSRF token for forms
	csrfToken := c.Locals("csrf").(string)

	// Render the profile page
	profileIndex := userviews.ProfileIndex(username, csrfToken)
	profile := userviews.Profile(
		" | Profil", getFromProtected(c), false, flash.Get(c), username, profileIndex,
	)

	handler := adaptor.HTTPHandler(templ.Handler(profile))

	return handler(c)
}

func HandleUserSettings(c *fiber.Ctx) error {
	// Get user information from session
	sess, _ := session.GetSessionStore().Get(c)
	_ = sess.Get(USER_ID) // Using _ to avoid unused variable warning
	username := sess.Get(USER_NAME).(string)

	// Get CSRF token for forms
	csrfToken := c.Locals("csrf").(string)

	// Render the settings page
	settingsIndex := userviews.SettingsIndex(username, csrfToken)
	settings := userviews.Settings(
		" | Einstellungen", getFromProtected(c), false, flash.Get(c), username, settingsIndex,
	)

	handler := adaptor.HTTPHandler(templ.Handler(settings))

	return handler(c)
}

func HandleUserImages(c *fiber.Ctx) error {
	// Get user information from session
	sess, _ := session.GetSessionStore().Get(c)
	userID := sess.Get(USER_ID).(uint)
	username := sess.Get(USER_NAME).(string)

	// Lade alle Bilder des Benutzers aus der Datenbank
	var images []models.Image
	result := database.DB.Where("user_id = ?", userID).Order("created_at DESC").Find(&images)
	if result.Error != nil {
		// Fehler beim Laden der Bilder
		flash.WithError(c, fiber.Map{"message": "Fehler beim Laden der Bilder: " + result.Error.Error()})
		return c.Redirect("/")
	}

	// Bereite die Bilderpfade für die Galerie vor
	var galleryImages []userviews.GalleryImage
	for _, img := range images {
		// Bestimme den Pfad zum mittleren Thumbnail
		previewPath := ""
		if img.HasThumbnails {
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

		galleryImages = append(galleryImages, userviews.GalleryImage{
			ID:          img.ID,
			UUID:        img.UUID,
			Title:       title,
			ShareLink:   img.ShareLink,
			PreviewPath: previewPath,
			CreatedAt:   img.CreatedAt.Format("02.01.2006 15:04"),
		})
	}

	// Render the gallery page
	imagesGallery := userviews.ImagesGallery(username, galleryImages)
	imagesPage := userviews.Images(
		" | Meine Bilder", getFromProtected(c), false, flash.Get(c), username, imagesGallery,
	)

	handler := adaptor.HTTPHandler(templ.Handler(imagesPage))

	return handler(c)
}
