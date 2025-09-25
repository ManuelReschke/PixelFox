package controllers

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/sujit-baniya/flash"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/app/repository"
	"github.com/ManuelReschke/PixelFox/internal/pkg/cache"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/entitlements"
	"github.com/ManuelReschke/PixelFox/internal/pkg/imageprocessor"
	"github.com/ManuelReschke/PixelFox/internal/pkg/usercontext"
	"github.com/ManuelReschke/PixelFox/views"
	upload_views "github.com/ManuelReschke/PixelFox/views/upload"
	"strings"
)

type batchItem struct {
	UUID      string `json:"uuid"`
	ViewURL   string `json:"view_url"`
	Duplicate bool   `json:"duplicate"`
}

type batchPayload struct {
	Items []batchItem `json:"items"`
}

type batchStored struct {
	UserID    uint        `json:"user_id"`
	Items     []batchItem `json:"items"`
	CreatedAt int64       `json:"created_at"`
}

// HandleCreateUploadBatch stores a temporary batch result in cache and returns a batch_id
func HandleCreateUploadBatch(c *fiber.Ctx) error {
	user := usercontext.GetUserContext(c)
	if !user.IsLoggedIn {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	var payload batchPayload
	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid payload"})
	}
	if len(payload.Items) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "no items"})
	}
	// sanitize: cap items length to a safe bound
	if len(payload.Items) > 100 {
		payload.Items = payload.Items[:100]
	}

	batch := batchStored{
		UserID:    user.UserID,
		Items:     payload.Items,
		CreatedAt: time.Now().Unix(),
	}
	b, _ := json.Marshal(batch)
	// Use time-based + user-bound unique ID (unix + random suffix)
	batchID := fmt.Sprintf("%d-%d", user.UserID, time.Now().UnixNano())
	key := fmt.Sprintf("upload:batch:%s", batchID)
	if err := cache.Set(key, string(b), 30*time.Minute); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to persist batch"})
	}
	return c.JSON(fiber.Map{"batch_id": batchID, "expires_at": time.Now().Add(30 * time.Minute).Unix()})
}

// HandleUploadBatchView renders an ephemeral batch result page; single-use: delete on first view
func HandleUploadBatchView(c *fiber.Ctx) error {
	user := usercontext.GetUserContext(c)
	if !user.IsLoggedIn {
		return c.Redirect("/login")
	}
	batchID := c.Params("id")
	if batchID == "" {
		return c.Redirect("/user/images")
	}
	key := fmt.Sprintf("upload:batch:%s", batchID)
	raw, err := cache.Get(key)
	if err != nil || raw == "" {
		// Batch not found or consumed
		flash.WithInfo(c, fiber.Map{"message": "Upload-Zusammenfassung nicht mehr verfügbar"})
		return c.Redirect("/user/images")
	}
	// Consume key (single-use)
	_ = cache.Delete(key)

	var stored batchStored
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		flash.WithError(c, fiber.Map{"message": "Fehler beim Laden der Upload-Zusammenfassung"})
		return c.Redirect("/user/images")
	}
	if stored.UserID != user.UserID {
		flash.WithError(c, fiber.Map{"message": "Zugriff verweigert"})
		return c.Redirect("/user/images")
	}

	// Build view model with previews, edit and share links
	_ = database.GetDB() // referenced to ensure DB initialized
	imgRepo := repository.GetGlobalFactory().GetImageRepository()
	var items []upload_views.BatchItem
	baseURL := c.BaseURL()
	for _, it := range stored.Items {
		if it.UUID == "" {
			continue
		}
		img, err := imgRepo.GetByUUID(it.UUID)
		preview := ""
		shareURL := it.ViewURL
		if err == nil && img != nil {
			preview = imageprocessor.GetBestPreviewURL(img)
			if shareURL == "" {
				shareURL = "/i/" + img.ShareLink
			}
		}
		if shareURL != "" {
			shareURL = imageprocessor.MakeAbsoluteURL(baseURL, shareURL)
		}
		items = append(items, upload_views.BatchItem{
			UUID:      it.UUID,
			ShareURL:  shareURL,
			EditURL:   "/user/images/edit/" + it.UUID,
			Preview:   preview,
			Duplicate: it.Duplicate,
		})
	}

	csrfToken := c.Locals("csrf").(string)
	cmp := upload_views.BatchResultIndex(csrfToken, batchID, items)
	page := views.HomeCtx(c, " | Uploads", true, false, flash.Get(c), cmp, user.IsAdmin, nil)
	handler := adaptor.HTTPHandler(templ.Handler(page))
	return handler(c)
}

// HandleUploadBatchSaveAsAlbum converts the posted list of UUIDs into a new album for the user
func HandleUploadBatchSaveAsAlbum(c *fiber.Ctx) error {
	user := usercontext.GetUserContext(c)
	if !user.IsLoggedIn {
		return c.Redirect("/login")
	}
	// Read UUIDs as CSV from form
	csv := strings.TrimSpace(c.FormValue("uuids"))
	if csv == "" {
		flash.WithError(c, fiber.Map{"message": "Keine Bilder übermittelt"})
		return c.Redirect("/user/albums")
	}
	// Enforce album creation limits per plan
	var albumCount int64
	database.DB.Model(&models.Album{}).Where("user_id = ?", user.UserID).Count(&albumCount)
	if !entitlements.CanCreateAlbum(entitlements.Plan(user.Plan), int(albumCount)) {
		flash.WithError(c, fiber.Map{"message": "Du willst mehr? Upgrade auf Premium"})
		return c.Redirect("/user/albums")
	}

	// Create album with default title
	title := fmt.Sprintf("Uploads %s", time.Now().Format("2006-01-02 15:04"))
	album := models.Album{UserID: user.UserID, Title: title, Description: "", IsPublic: false}
	if err := database.DB.Create(&album).Error; err != nil {
		flash.WithError(c, fiber.Map{"message": "Album konnte nicht erstellt werden"})
		return c.Redirect("/user/albums")
	}

	uuids := strings.Split(csv, ",")
	imgRepo := repository.GetGlobalFactory().GetImageRepository()
	firstImageID := uint(0)
	added := 0
	for _, u := range uuids {
		uuid := strings.TrimSpace(u)
		if uuid == "" {
			continue
		}
		img, err := imgRepo.GetByUUID(uuid)
		if err != nil || img == nil || img.UserID != user.UserID {
			continue
		}
		// Check not already in album
		var exists models.AlbumImage
		if err := database.DB.Where("album_id = ? AND image_id = ?", album.ID, img.ID).First(&exists).Error; err == nil {
			continue
		}
		rel := models.AlbumImage{AlbumID: album.ID, ImageID: img.ID}
		if err := database.DB.Create(&rel).Error; err == nil {
			added++
			if firstImageID == 0 {
				firstImageID = img.ID
			}
		}
	}

	if added == 0 {
		// Nothing added -> remove empty album
		_ = database.DB.Delete(&album).Error
		flash.WithError(c, fiber.Map{"message": "Keine gültigen Bilder gefunden"})
		return c.Redirect("/user/albums")
	}
	// Set cover if available
	if firstImageID != 0 {
		album.CoverImageID = firstImageID
		_ = database.DB.Save(&album).Error
	}

	flash.WithSuccess(c, fiber.Map{"message": fmt.Sprintf("Album erstellt (%d Bilder)", added)})
	return c.Redirect(fmt.Sprintf("/user/albums/%d", album.ID))
}
