package controllers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/sujit-baniya/flash"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/entitlements"
	"github.com/ManuelReschke/PixelFox/internal/pkg/imageprocessor"
	metrics "github.com/ManuelReschke/PixelFox/internal/pkg/metrics/counter"
	"github.com/ManuelReschke/PixelFox/internal/pkg/usercontext"
	"github.com/ManuelReschke/PixelFox/internal/pkg/viewmodel"
	user_views "github.com/ManuelReschke/PixelFox/views/user"
)

// Helper function to convert models.Image to GalleryImage with proper paths
func imageToGalleryImage(img models.Image) user_views.GalleryImage {
	// Use centralized best-preview helper (absolute URL)
	previewPath := imageprocessor.GetBestPreviewURL(&img)
	smallPreviewPath := ""

	// Small preview path for album covers / selection modal
	if p := imageprocessor.GetImageURL(&img, "avif", "small"); p != "" {
		smallPreviewPath = p
	} else if p := imageprocessor.GetImageURL(&img, "webp", "small"); p != "" {
		smallPreviewPath = p
	} else if p := imageprocessor.GetImageURL(&img, "original", "small"); p != "" {
		smallPreviewPath = p
	}
	// Final fallback to original image if no small thumbnail available
	if smallPreviewPath == "" {
		smallPreviewPath = imageprocessor.GetImageURL(&img, "original", "")
	}

	title := img.FileName
	if img.Title != "" {
		title = img.Title
	}

	// Convert to absolute URLs
	if smallPreviewPath != "" {
		smallPreviewPath = imageprocessor.MakeAbsoluteForImage(&img, smallPreviewPath)
	}
	originalPath := imageprocessor.GetImageAbsoluteURL(&img, "original", "")
	return user_views.GalleryImage{
		ID:               img.ID,
		UUID:             img.UUID,
		Title:            title,
		ShareLink:        img.ShareLink,
		PreviewPath:      previewPath,
		SmallPreviewPath: smallPreviewPath,
		OriginalPath:     originalPath,
		Width:            img.Width,
		Height:           img.Height,
		FileSize:         img.FileSize,
		CreatedAt:        img.CreatedAt.Format("02.01.2006 15:04"),
	}
}

func HandleUserAlbums(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	userID := userCtx.UserID
	username := userCtx.Username
	isAdmin := userCtx.IsAdmin

	var albums []models.Album
	if err := database.DB.Where("user_id = ?", userID).Preload("Images.StoragePool").Find(&albums).Error; err != nil {
		flash.WithError(c, fiber.Map{"message": "Fehler beim Laden der Alben"})
		return c.Redirect("/")
	}

	// Convert albums with their images to GalleryImage format
	var albumsWithGalleryImages []user_views.AlbumWithGalleryImages
	for _, album := range albums {
		var galleryImages []user_views.GalleryImage
		for _, img := range album.Images {
			galleryImages = append(galleryImages, imageToGalleryImage(img))
		}
		albumsWithGalleryImages = append(albumsWithGalleryImages, user_views.AlbumWithGalleryImages{
			Album:  album,
			Images: galleryImages,
		})
	}

	csrfToken := c.Locals("csrf").(string)

	albumsIndex := user_views.AlbumsIndex(username, csrfToken, userCtx.Plan, albumsWithGalleryImages)
	albumsPage := user_views.Albums(
		" | Meine Alben", isLoggedIn(c), false, flash.Get(c), username, usercontext.GetUserContext(c).Plan, albumsIndex, isAdmin,
	)

	return adaptor.HTTPHandler(templ.Handler(albumsPage))(c)
}

func HandleUserAlbumCreate(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	userID := userCtx.UserID
	username := userCtx.Username
	isAdmin := userCtx.IsAdmin

	// Enforce album creation limits based on plan (both GET and POST)
	var albumCount int64
	database.DB.Model(&models.Album{}).Where("user_id = ?", userID).Count(&albumCount)
	if !entitlements.CanCreateAlbum(entitlements.Plan(userCtx.Plan), int(albumCount)) {
		flash.WithError(c, fiber.Map{"message": "Du willst mehr? Upgrade auf Premium"})
		return c.Redirect("/user/albums")
	}

	if c.Method() == "POST" {
		title := c.FormValue("title")
		description := c.FormValue("description")
		isPublic := c.FormValue("is_public") == "on"

		if title == "" {
			flash.WithError(c, fiber.Map{"message": "Titel ist erforderlich"})
			return c.Redirect("/user/albums/create")
		}

		album := models.Album{
			UserID:      userID,
			Title:       title,
			Description: description,
			IsPublic:    isPublic,
		}

		if err := database.DB.Create(&album).Error; err != nil {
			flash.WithError(c, fiber.Map{"message": "Fehler beim Erstellen des Albums"})
			return c.Redirect("/user/albums/create")
		}

		flash.WithSuccess(c, fiber.Map{"message": "Album erfolgreich erstellt"})
		return c.Redirect("/user/albums")
	}

	csrfToken := c.Locals("csrf").(string)

	createIndex := user_views.AlbumCreateIndex(username, csrfToken)
	createPage := user_views.AlbumCreate(
		" | Album erstellen", isLoggedIn(c), false, flash.Get(c), username, createIndex, isAdmin,
	)

	return adaptor.HTTPHandler(templ.Handler(createPage))(c)
}

func HandleUserAlbumEdit(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	userID := userCtx.UserID
	username := userCtx.Username
	isAdmin := userCtx.IsAdmin

	albumIDStr := c.Params("id")
	albumID, err := strconv.ParseUint(albumIDStr, 10, 32)
	if err != nil {
		flash.WithError(c, fiber.Map{"message": "Ungültige Album-ID"})
		return c.Redirect("/user/albums")
	}

	var album models.Album
	if err := database.DB.Where("id = ? AND user_id = ?", albumID, userID).Preload("Images.StoragePool").First(&album).Error; err != nil {
		flash.WithError(c, fiber.Map{"message": "Album nicht gefunden"})
		return c.Redirect("/user/albums")
	}

	// Increment album view counter (buffered in Redis)
	_ = metrics.AddAlbumView(album.ID)

	if c.Method() == "POST" {
		title := c.FormValue("title")
		description := c.FormValue("description")
		coverImageIDStr := c.FormValue("cover_image_id")
		isPublic := c.FormValue("is_public") == "on"

		if title == "" {
			flash.WithError(c, fiber.Map{"message": "Titel ist erforderlich"})
			return c.Redirect("/user/albums/edit/" + albumIDStr)
		}

		album.Title = title
		album.Description = description
		album.IsPublic = isPublic

		if coverImageIDStr != "" {
			coverImageID, err := strconv.ParseUint(coverImageIDStr, 10, 32)
			if err == nil {
				var imageExists bool
				database.DB.Model(&models.AlbumImage{}).Where("album_id = ? AND image_id = ?", album.ID, coverImageID).Select("count(*) > 0").Find(&imageExists)
				if imageExists {
					album.CoverImageID = uint(coverImageID)
				}
			}
		}

		if err := database.DB.Save(&album).Error; err != nil {
			flash.WithError(c, fiber.Map{"message": "Fehler beim Aktualisieren des Albums"})
			return c.Redirect("/user/albums/edit/" + albumIDStr)
		}

		flash.WithSuccess(c, fiber.Map{"message": "Album erfolgreich aktualisiert"})
		return c.Redirect("/user/albums")
	}

	csrfToken := c.Locals("csrf").(string)

	editIndex := user_views.AlbumEditIndex(username, csrfToken, album)
	editPage := user_views.AlbumEdit(
		" | Album bearbeiten", isLoggedIn(c), false, flash.Get(c), username, editIndex, isAdmin,
	)

	return adaptor.HTTPHandler(templ.Handler(editPage))(c)
}

func HandleUserAlbumDelete(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	userID := userCtx.UserID

	albumIDStr := c.Params("id")
	albumID, err := strconv.ParseUint(albumIDStr, 10, 32)
	if err != nil {
		flash.WithError(c, fiber.Map{"message": "Ungültige Album-ID"})
		return c.Redirect("/user/albums")
	}

	var album models.Album
	if err := database.DB.Where("id = ? AND user_id = ?", albumID, userID).First(&album).Error; err != nil {
		flash.WithError(c, fiber.Map{"message": "Album nicht gefunden"})
		return c.Redirect("/user/albums")
	}

	database.DB.Where("album_id = ?", album.ID).Delete(&models.AlbumImage{})

	if err := database.DB.Delete(&album).Error; err != nil {
		flash.WithError(c, fiber.Map{"message": "Fehler beim Löschen des Albums"})
		return c.Redirect("/user/albums")
	}

	flash.WithSuccess(c, fiber.Map{"message": "Album erfolgreich gelöscht"})
	return c.Redirect("/user/albums")
}

func HandleUserAlbumView(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	userID := userCtx.UserID
	username := userCtx.Username
	isAdmin := userCtx.IsAdmin

	albumIDStr := c.Params("id")
	albumID, err := strconv.ParseUint(albumIDStr, 10, 32)
	if err != nil {
		flash.WithError(c, fiber.Map{"message": "Ungültige Album-ID"})
		return c.Redirect("/user/albums")
	}

	var album models.Album
	if err := database.DB.Where("id = ? AND user_id = ?", albumID, userID).Preload("Images.StoragePool").First(&album).Error; err != nil {
		flash.WithError(c, fiber.Map{"message": "Album nicht gefunden"})
		return c.Redirect("/user/albums")
	}

	var userImages []models.Image
	database.DB.Preload("StoragePool").Where("user_id = ?", userID).Find(&userImages)

	// Convert user images to GalleryImage format
	var galleryUserImages []user_views.GalleryImage
	for _, img := range userImages {
		galleryUserImages = append(galleryUserImages, imageToGalleryImage(img))
	}

	// Convert album images to GalleryImage format
	var galleryAlbumImages []user_views.GalleryImage
	for _, img := range album.Images {
		galleryAlbumImages = append(galleryAlbumImages, imageToGalleryImage(img))
	}

	csrfToken := c.Locals("csrf").(string)

	viewIndex := user_views.AlbumViewIndex(username, csrfToken, album, galleryAlbumImages, galleryUserImages)
	viewPage := user_views.AlbumView(
		" | "+album.Title, isLoggedIn(c), false, flash.Get(c), username, viewIndex, isAdmin,
	)

	return adaptor.HTTPHandler(templ.Handler(viewPage))(c)
}

func HandleUserAlbumAddImage(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	userID := userCtx.UserID

	albumIDStr := c.Params("id")
	albumID, err := strconv.ParseUint(albumIDStr, 10, 32)
	if err != nil {
		flash.WithError(c, fiber.Map{"message": "Ungültige Album-ID"})
		return c.Redirect("/user/albums")
	}

	imageIDStr := c.FormValue("image_id")
	imageID, err := strconv.ParseUint(imageIDStr, 10, 32)
	if err != nil {
		flash.WithError(c, fiber.Map{"message": "Ungültige Bild-ID"})
		return c.Redirect("/user/albums/" + albumIDStr)
	}

	var album models.Album
	if err := database.DB.Where("id = ? AND user_id = ?", albumID, userID).First(&album).Error; err != nil {
		flash.WithError(c, fiber.Map{"message": "Album nicht gefunden"})
		return c.Redirect("/user/albums")
	}

	var image models.Image
	if err := database.DB.Where("id = ? AND user_id = ?", imageID, userID).First(&image).Error; err != nil {
		flash.WithError(c, fiber.Map{"message": "Bild nicht gefunden"})
		return c.Redirect("/user/albums/" + albumIDStr)
	}

	var exists models.AlbumImage
	if err := database.DB.Where("album_id = ? AND image_id = ?", albumID, imageID).First(&exists).Error; err == nil {
		flash.WithError(c, fiber.Map{"message": "Bild ist bereits im Album"})
		return c.Redirect("/user/albums/" + albumIDStr)
	}

	albumImage := models.AlbumImage{
		AlbumID: uint(albumID),
		ImageID: uint(imageID),
	}

	if err := database.DB.Create(&albumImage).Error; err != nil {
		flash.WithError(c, fiber.Map{"message": "Fehler beim Hinzufügen des Bildes"})
		return c.Redirect("/user/albums/" + albumIDStr)
	}

	flash.WithSuccess(c, fiber.Map{"message": "Bild erfolgreich hinzugefügt"})
	return c.Redirect("/user/albums/" + albumIDStr)
}

func HandleUserAlbumRemoveImage(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	userID := userCtx.UserID

	albumIDStr := c.Params("id")
	albumID, err := strconv.ParseUint(albumIDStr, 10, 32)
	if err != nil {
		flash.WithError(c, fiber.Map{"message": "Ungültige Album-ID"})
		return c.Redirect("/user/albums")
	}

	imageIDStr := c.Params("image_id")
	imageID, err := strconv.ParseUint(imageIDStr, 10, 32)
	if err != nil {
		flash.WithError(c, fiber.Map{"message": "Ungültige Bild-ID"})
		return c.Redirect("/user/albums/" + albumIDStr)
	}

	var album models.Album
	if err := database.DB.Where("id = ? AND user_id = ?", albumID, userID).First(&album).Error; err != nil {
		flash.WithError(c, fiber.Map{"message": "Album nicht gefunden"})
		return c.Redirect("/user/albums")
	}

	if err := database.DB.Where("album_id = ? AND image_id = ?", albumID, imageID).Delete(&models.AlbumImage{}).Error; err != nil {
		flash.WithError(c, fiber.Map{"message": "Fehler beim Entfernen des Bildes"})
		return c.Redirect("/user/albums/" + albumIDStr)
	}

	if album.CoverImageID == uint(imageID) {
		album.CoverImageID = 0
		database.DB.Save(&album)
	}

	flash.WithSuccess(c, fiber.Map{"message": "Bild erfolgreich entfernt"})
	return c.Redirect("/user/albums/" + albumIDStr)
}

// HandleUserAlbumSetCover sets the cover image for an album
func HandleUserAlbumSetCover(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	userID := userCtx.UserID

	albumIDStr := c.Params("id")
	albumID, err := strconv.ParseUint(albumIDStr, 10, 32)
	if err != nil {
		flash.WithError(c, fiber.Map{"message": "Ungültige Album-ID"})
		return c.Redirect("/user/albums")
	}

	// Verify album ownership
	var album models.Album
	if err := database.DB.Where("id = ? AND user_id = ?", albumID, userID).First(&album).Error; err != nil {
		flash.WithError(c, fiber.Map{"message": "Album nicht gefunden"})
		return c.Redirect("/user/albums")
	}

	// Parse image id
	imageIDStr := c.FormValue("image_id")
	imageID, err := strconv.ParseUint(imageIDStr, 10, 32)
	if err != nil || imageID == 0 {
		flash.WithError(c, fiber.Map{"message": "Ungültige Bild-ID"})
		return c.Redirect("/user/albums/" + albumIDStr)
	}

	// Ensure the image belongs to the user
	var image models.Image
	if err := database.DB.Where("id = ? AND user_id = ?", imageID, userID).First(&image).Error; err != nil {
		flash.WithError(c, fiber.Map{"message": "Bild nicht gefunden"})
		return c.Redirect("/user/albums/" + albumIDStr)
	}

	// Ensure the image is part of the album
	var rel models.AlbumImage
	if err := database.DB.Where("album_id = ? AND image_id = ?", albumID, imageID).First(&rel).Error; err != nil {
		flash.WithError(c, fiber.Map{"message": "Bild ist nicht in diesem Album"})
		return c.Redirect("/user/albums/" + albumIDStr)
	}

	album.CoverImageID = uint(imageID)
	if err := database.DB.Save(&album).Error; err != nil {
		flash.WithError(c, fiber.Map{"message": "Fehler beim Setzen des Cover-Bildes"})
		return c.Redirect("/user/albums/" + albumIDStr)
	}

	flash.WithSuccess(c, fiber.Map{"message": "Cover-Bild aktualisiert"})
	return c.Redirect("/user/albums/" + albumIDStr)
}

// HandleAlbumShareLink renders a public view for an album using its share link
func HandleAlbumShareLink(c *fiber.Ctx) error {
	sharelink := c.Params("sharelink")
	if sharelink == "" {
		return c.Redirect("/")
	}

	var album models.Album
	// Load album by share link with images and storage pool for proper URLs
	if err := database.DB.Preload("Images.StoragePool").Where("share_link = ?", sharelink).First(&album).Error; err != nil {
		return c.Redirect("/")
	}

	// Build gallery images
	var galleryAlbumImages []user_views.GalleryImage
	for _, img := range album.Images {
		galleryAlbumImages = append(galleryAlbumImages, imageToGalleryImage(img))
	}

	// Build Open Graph model using cover image, title and shortened description
	var cover *models.Image
	if album.CoverImageID != 0 {
		for i := range album.Images {
			if album.Images[i].ID == album.CoverImageID {
				cover = &album.Images[i]
				break
			}
		}
	}
	if cover == nil && len(album.Images) > 0 {
		cover = &album.Images[0]
	}

	coverURL := ""
	if cover != nil {
		coverURL = imageprocessor.GetBestPreviewURL(cover)
	}

	ogTitle := album.Title
	ogDesc := strings.TrimSpace(album.Description)
	if ogDesc == "" {
		ogDesc = album.Title
	}
	ogDesc = truncateForOG(ogDesc, 180)

	shareURL := fmt.Sprintf("%s/a/%s", c.BaseURL(), album.ShareLink)
	og := &viewmodel.OpenGraph{
		URL:         shareURL,
		Image:       coverURL,
		ImageAlt:    ogTitle,
		Title:       ogTitle,
		Description: ogDesc,
	}

	pageTitle := fmt.Sprintf(" | %s", album.Title)
	cmp := user_views.PublicAlbumIndex(album, galleryAlbumImages)
	page := user_views.PublicAlbum(pageTitle, false, false, nil, "", cmp, false, og)
	// Increment album view counter for public views as well
	_ = metrics.AddAlbumView(album.ID)
	return adaptor.HTTPHandler(templ.Handler(page))(c)
}

// truncateForOG shortens a string to max characters without breaking words when possible
func truncateForOG(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	// Attempt to cut at last space before max
	cut := s[:max]
	if idx := strings.LastIndex(cut, " "); idx > 40 { // keep at least some length
		cut = cut[:idx]
	}
	return strings.TrimSpace(cut) + "…"
}
