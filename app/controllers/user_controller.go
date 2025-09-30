package controllers

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"time"

	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/sujit-baniya/flash"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/app/repository"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/entitlements"
	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
	"github.com/ManuelReschke/PixelFox/internal/pkg/imageprocessor"
	"github.com/ManuelReschke/PixelFox/internal/pkg/jobqueue"
	"github.com/ManuelReschke/PixelFox/internal/pkg/mail"
	"github.com/ManuelReschke/PixelFox/internal/pkg/session"
	"github.com/ManuelReschke/PixelFox/internal/pkg/usercontext"
	"github.com/ManuelReschke/PixelFox/views"
	email_views "github.com/ManuelReschke/PixelFox/views/email_views"
	user_views "github.com/ManuelReschke/PixelFox/views/user"
)

const sessionNewAPIKey = "user_new_api_key"

// groupLabelForTime returns a human-friendly bucket label for the image timestamp
// Buckets: Heute, Gestern, Ältere (using local time)
func groupLabelForTime(t time.Time) string {
	now := time.Now().In(time.Local)
	y0, m0, d0 := now.Date()
	today := time.Date(y0, m0, d0, 0, 0, 0, 0, time.Local)

	tl := t.In(time.Local)
	y1, m1, d1 := tl.Date()
	day := time.Date(y1, m1, d1, 0, 0, 0, 0, time.Local)

	switch {
	case day.Equal(today):
		return "Heute"
	case day.Equal(today.AddDate(0, 0, -1)):
		return "Gestern"
	default:
		return "Ältere"
	}
}

func HandleUserProfile(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	userID := userCtx.UserID
	username := userCtx.Username
	isAdmin := userCtx.IsAdmin

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
	database.DB.Model(&models.Image{}).Where("user_id = ?", userID).Select("COALESCE(SUM(file_size), 0)").Row().Scan(&totalStorage)

	csrfToken := c.Locals("csrf").(string)

	profileIndex := user_views.ProfileIndex(username, csrfToken, userCtx.Plan, user, int(imageCount), int(albumCount), int64(totalStorage))
	profile := user_views.Profile(
		" | Profil", userCtx.IsLoggedIn, false, flash.Get(c), username, userCtx.Plan, profileIndex, isAdmin,
	)

	handler := adaptor.HTTPHandler(templ.Handler(profile))

	return handler(c)
}

func HandleUserSettings(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	username := userCtx.Username
	isAdmin := userCtx.IsAdmin

	csrfToken := c.Locals("csrf").(string)

	// Load or create user settings
	db := database.GetDB()
	us, _ := models.GetOrCreateUserSettings(db, userCtx.UserID)
	newAPIKey := session.GetSessionValue(c, sessionNewAPIKey)
	if newAPIKey != "" {
		if err := session.SetSessionValue(c, sessionNewAPIKey, ""); err != nil {
			log.Printf("failed to clear api key session value for user %d: %v", userCtx.UserID, err)
		}
	}
	hasAPIKey := us.HasActiveAPIKey()
	maskedAPIKey := ""
	if hasAPIKey && us.APIKeyPrefix != "" {
		maskedAPIKey = fmt.Sprintf("%s********", us.APIKeyPrefix)
	}
	apiKeyCreated := ""
	if us.APIKeyCreatedAt != nil {
		apiKeyCreated = us.APIKeyCreatedAt.In(time.Local).Format("02.01.2006 15:04")
	}
	apiKeyLastUsed := ""
	if us.APIKeyLastUsedAt != nil {
		apiKeyLastUsed = us.APIKeyLastUsedAt.In(time.Local).Format("02.01.2006 15:04")
	}
	// Compute entitlements against app settings
	app := models.GetAppSettings()
	allowedOrig, allowedWebp, allowedAvif := entitlements.AllowedThumbs(entitlements.Plan(us.Plan))
	// Admin toggles may further restrict visibility/enablement (UI can show info)
	adminOrig := app.IsThumbnailOriginalEnabled()
	adminWebp := app.IsThumbnailWebPEnabled()
	adminAvif := app.IsThumbnailAVIFEnabled()

	settingsIndex := user_views.SettingsIndex(username, csrfToken, us.Plan,
		allowedOrig && adminOrig, allowedWebp && adminWebp, allowedAvif && adminAvif,
		us.PrefThumbOriginal, us.PrefThumbWebP, us.PrefThumbAVIF,
		newAPIKey, hasAPIKey, maskedAPIKey, apiKeyCreated, apiKeyLastUsed)
	settings := user_views.Settings(
		" | Einstellungen", userCtx.IsLoggedIn, false, flash.Get(c), username, us.Plan, settingsIndex, isAdmin,
	)

	handler := adaptor.HTTPHandler(templ.Handler(settings))

	return handler(c)
}

// HandleUserSettingsPost updates user preferences (clamped by entitlements)
func HandleUserSettingsPost(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	if !userCtx.IsLoggedIn {
		return c.Redirect("/login")
	}
	db := database.GetDB()
	us, err := models.GetOrCreateUserSettings(db, userCtx.UserID)
	if err != nil {
		flash.WithError(c, fiber.Map{"message": "Einstellungen konnten nicht geladen werden"})
		return c.Redirect("/user/settings")
	}
	// Read posted values
	wantOrig := c.FormValue("pref_thumb_original") == "on"
	wantWebp := c.FormValue("pref_thumb_webp") == "on"
	wantAvif := c.FormValue("pref_thumb_avif") == "on"

	app := models.GetAppSettings()
	// Clamp to entitlements and admin settings
	allowOrig, allowWebp, allowAvif := entitlements.AllowedThumbs(entitlements.Plan(us.Plan))
	us.PrefThumbOriginal = wantOrig && allowOrig && app.IsThumbnailOriginalEnabled()
	us.PrefThumbWebP = wantWebp && allowWebp && app.IsThumbnailWebPEnabled()
	us.PrefThumbAVIF = wantAvif && allowAvif && app.IsThumbnailAVIFEnabled()

	if err := db.Save(us).Error; err != nil {
		flash.WithError(c, fiber.Map{"message": "Einstellungen speichern fehlgeschlagen"})
		return c.Redirect("/user/settings")
	}
	flash.WithSuccess(c, fiber.Map{"message": "Einstellungen gespeichert"})
	return c.Redirect("/user/settings")
}

// HandleUserAPIKeyGenerate issues or rotates the user's API key.
func HandleUserAPIKeyGenerate(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	if !userCtx.IsLoggedIn {
		return c.Redirect("/login")
	}
	db := database.GetDB()
	us, err := models.GetOrCreateUserSettings(db, userCtx.UserID)
	if err != nil {
		flash.WithError(c, fiber.Map{"message": "API-Einstellungen konnten nicht geladen werden"})
		return c.Redirect("/user/settings")
	}
	rawKey, err := us.IssueAPIKey()
	if err != nil {
		log.Printf("failed to issue api key for user %d: %v", userCtx.UserID, err)
		flash.WithError(c, fiber.Map{"message": "API-Schlüssel konnte nicht erstellt werden"})
		return c.Redirect("/user/settings")
	}
	if err := db.Save(us).Error; err != nil {
		log.Printf("failed to persist api key for user %d: %v", userCtx.UserID, err)
		flash.WithError(c, fiber.Map{"message": "API-Schlüssel konnte nicht gespeichert werden"})
		return c.Redirect("/user/settings")
	}
	if err := session.SetSessionValue(c, sessionNewAPIKey, rawKey); err != nil {
		log.Printf("failed to stash api key in session for user %d: %v", userCtx.UserID, err)
	}
	flash.WithSuccess(c, fiber.Map{"message": "Neuer API-Schlüssel erstellt. Bitte sicher speichern."})
	return c.Redirect("/user/settings")
}

// HandleUserAPIKeyRevoke removes the current API key and marks it as revoked.
func HandleUserAPIKeyRevoke(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	if !userCtx.IsLoggedIn {
		return c.Redirect("/login")
	}
	db := database.GetDB()
	us, err := models.GetOrCreateUserSettings(db, userCtx.UserID)
	if err != nil {
		flash.WithError(c, fiber.Map{"message": "API-Einstellungen konnten nicht geladen werden"})
		return c.Redirect("/user/settings")
	}
	if !us.HasActiveAPIKey() {
		flash.WithInfo(c, fiber.Map{"message": "Es ist kein aktiver API-Schlüssel vorhanden."})
		return c.Redirect("/user/settings")
	}
	us.RevokeAPIKey()
	if err := db.Save(us).Error; err != nil {
		log.Printf("failed to revoke api key for user %d: %v", userCtx.UserID, err)
		flash.WithError(c, fiber.Map{"message": "API-Schlüssel konnte nicht widerrufen werden"})
		return c.Redirect("/user/settings")
	}
	if err := session.SetSessionValue(c, sessionNewAPIKey, ""); err != nil {
		log.Printf("failed to clear api key session value for user %d: %v", userCtx.UserID, err)
	}
	flash.WithSuccess(c, fiber.Map{"message": "API-Schlüssel wurde entfernt."})
	return c.Redirect("/user/settings")
}

func HandleUserImages(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	userID := userCtx.UserID
	username := userCtx.Username
	isAdmin := userCtx.IsAdmin

	// Optional year filter
	selectedYear := 0
	if y := c.Query("year", ""); y != "" {
		if yi, err := strconv.Atoi(y); err == nil && yi > 0 {
			selectedYear = yi
		}
	}

	// Total count for header
	var totalCount int64
	countQuery := database.DB.Model(&models.Image{}).Where("user_id = ?", userID)
	if selectedYear > 0 {
		countQuery = countQuery.Where("YEAR(created_at) = ?", selectedYear)
	}
	countQuery.Count(&totalCount)

	// Initial page: load first 25 items (rest via HTMX)
	var images []models.Image
	query := database.DB.Preload("StoragePool").Where("user_id = ?", userID)
	if selectedYear > 0 {
		query = query.Where("YEAR(created_at) = ?", selectedYear)
	}
	result := query.Order("created_at DESC").Limit(25).Find(&images)
	if result.Error != nil {
		// Fehler beim Laden der Bilder
		flash.WithError(c, fiber.Map{"message": "Fehler beim Laden der Bilder: " + result.Error.Error()})
		return c.Redirect("/")
	}

	// Distinct years for quick filter (descending)
	years := []int{}
	_ = database.DB.Model(&models.Image{}).
		Where("user_id = ?", userID).
		Select("DISTINCT YEAR(created_at) as y").
		Order("y DESC").
		Pluck("y", &years).Error

	// Bereite die Bilderpfade für die Galerie vor
	var galleryImages []user_views.GalleryImage
	prevGroup := ""
	for i, img := range images {
		// Use centralized helper for a cross-node absolute preview URL
		previewPath := imageprocessor.GetBestPreviewURL(&img)

		title := img.FileName
		if img.Title != "" {
			title = img.Title
		}
		// Absolute original URL
		originalPath := imageprocessor.GetImageAbsoluteURL(&img, "original", "")
		group := groupLabelForTime(img.CreatedAt)
		renderHeader := i == 0 || group != prevGroup

		galleryImages = append(galleryImages, user_views.GalleryImage{
			ID:           img.ID,
			UUID:         img.UUID,
			Title:        title,
			ShareLink:    img.ShareLink,
			PreviewPath:  previewPath,
			OriginalPath: originalPath,
			CreatedAt:    img.CreatedAt.Format("02.01.2006 15:04"),
			IsPublic:     img.IsPublic,
			FileName:     img.FileName,
			Width:        img.Width,
			Height:       img.Height,
			FileSize:     img.FileSize,
			GroupLabel:   group,
			RenderHeader: renderHeader,
		})
		prevGroup = group
	}

	// Group into fixed bucket order to guarantee visual order: Heute, Gestern, Ältere
	buckets := map[string][]user_views.GalleryImage{
		"Heute":   {},
		"Gestern": {},
		"Ältere":  {},
	}
	for _, gi := range galleryImages {
		buckets[gi.GroupLabel] = append(buckets[gi.GroupLabel], gi)
	}
	groups := make([]user_views.ImageGroup, 0, 3)
	for _, label := range []string{"Heute", "Gestern", "Ältere"} {
		if items := buckets[label]; len(items) > 0 {
			groups = append(groups, user_views.ImageGroup{Label: label, Items: items})
		}
	}

	imagesGallery := user_views.ImagesGallery(username, groups, int(totalCount), years, selectedYear)
	imagesPage := user_views.Images(
		" | Meine Bilder", userCtx.IsLoggedIn, false, flash.Get(c), username, userCtx.Plan, imagesGallery, isAdmin,
	)

	handler := adaptor.HTTPHandler(templ.Handler(imagesPage))

	return handler(c)
}

func HandleLoadMoreImages(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	userID := userCtx.UserID

	page, err := strconv.Atoi(c.Query("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}

	const imagesPerPage = 25
	offset := (page - 1) * imagesPerPage

	// Optional year filter
	selectedYear := 0
	if y := c.Query("year", ""); y != "" {
		if yi, err := strconv.Atoi(y); err == nil && yi > 0 {
			selectedYear = yi
		}
	}

	var images []models.Image
	query := database.DB.Preload("StoragePool").Where("user_id = ?", userID)
	if selectedYear > 0 {
		query = query.Where("YEAR(created_at) = ?", selectedYear)
	}
	result := query.Order("created_at DESC").Offset(offset).Limit(imagesPerPage).Find(&images)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Fehler beim Laden der Bilder")
	}

	// Last rendered group from the previous batch (for header suppression at boundary)
	lastGroup := c.Query("last_group", "")

	var galleryImages []user_views.GalleryImage
	prevGroup := ""
	for i, img := range images {
		previewPath := imageprocessor.GetBestPreviewURL(&img)

		title := img.FileName
		if img.Title != "" {
			title = img.Title
		}

		originalPath := imageprocessor.GetImageAbsoluteURL(&img, "original", "")
		group := groupLabelForTime(img.CreatedAt)
		renderHeader := (i == 0 && group != lastGroup) || (i > 0 && group != prevGroup)

		galleryImages = append(galleryImages, user_views.GalleryImage{
			ID:           img.ID,
			UUID:         img.UUID,
			Title:        title,
			ShareLink:    img.ShareLink,
			PreviewPath:  previewPath,
			OriginalPath: originalPath,
			CreatedAt:    img.CreatedAt.Format("02.01.2006 15:04"),
			IsPublic:     img.IsPublic,
			FileName:     img.FileName,
			Width:        img.Width,
			Height:       img.Height,
			FileSize:     img.FileSize,
			GroupLabel:   group,
			RenderHeader: renderHeader,
		})
		prevGroup = group
	}
	// Group into fixed bucket order to guarantee visual order for subsequent pages as well
	buckets := map[string][]user_views.GalleryImage{
		"Heute":   {},
		"Gestern": {},
		"Ältere":  {},
	}
	for _, gi := range galleryImages {
		buckets[gi.GroupLabel] = append(buckets[gi.GroupLabel], gi)
	}
	groups := make([]user_views.ImageGroup, 0, 3)
	for _, label := range []string{"Heute", "Gestern", "Ältere"} {
		if items := buckets[label]; len(items) > 0 {
			groups = append(groups, user_views.ImageGroup{Label: label, Items: items})
		}
	}

	return user_views.GalleryGroups(groups, page, lastGroup, selectedYear).Render(c.Context(), c.Response().BodyWriter())
}

// HandleUserImageEdit allows users to edit their own images
func HandleUserImageEdit(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	userID := userCtx.UserID
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
	page := views.HomeCtx(c, fmt.Sprintf("| Bild %s bearbeiten", image.Title), userCtx.IsLoggedIn, false, flash.Get(c), userEdit, userCtx.IsAdmin, nil)
	handler := adaptor.HTTPHandler(templ.Handler(page))
	return handler(c)
}

// HandleUserImageUpdate processes the edit form
func HandleUserImageUpdate(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	userID := userCtx.UserID
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
	userCtx := usercontext.GetUserContext(c)
	userID := userCtx.UserID
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

	// Enqueue async DeleteImage job
	queue := jobqueue.GetManager().GetQueue()
	initiated := userID
	if _, err := queue.EnqueueDeleteImageJob(image.ID, image.UUID, nil, &initiated); err != nil {
		flash.WithError(c, fiber.Map{"type": "error", "message": "Löschauftrag konnte nicht erstellt werden"})
		return c.Redirect("/user/images")
	}

	// Immediate soft-delete via repository to hide image from UI
	imgRepo := repository.GetGlobalFactory().GetImageRepository()
	if err := imgRepo.Delete(image.ID); err != nil {
		flash.WithError(c, fiber.Map{"type": "error", "message": "Fehler beim Entfernen in der Datenbank"})
		return c.Redirect("/user/images")
	}

	flash.WithSuccess(c, fiber.Map{"type": "success", "message": "Löschung eingeplant"})
	return c.Redirect("/user/images")
}

// enqueueUserS3DeleteJobsIfEnabled creates S3 delete jobs for completed backups if S3 backup is enabled
func enqueueUserS3DeleteJobsIfEnabled(image *models.Image) {
	// Check admin setting for S3 backup enablement
	settings := models.GetAppSettings()
	if settings == nil || !settings.IsS3BackupEnabled() {
		fmt.Printf("[S3Delete] S3 backup disabled by settings, skipping delete for image %s\n", image.UUID)
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

func HandleUserProfileEdit(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	userID := userCtx.UserID
	username := userCtx.Username
	isAdmin := userCtx.IsAdmin

	// Get user data from database
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		flash.WithError(c, fiber.Map{"message": "User not found"})
		return c.Redirect("/")
	}

	csrfToken := c.Locals("csrf").(string)

	profileEditIndex := user_views.ProfileEditIndex(username, csrfToken, user)
	profileEdit := user_views.ProfileEdit(
		" | Profil bearbeiten", userCtx.IsLoggedIn, false, flash.Get(c), username, profileEditIndex, isAdmin,
	)

	handler := adaptor.HTTPHandler(templ.Handler(profileEdit))
	return handler(c)
}

func HandleUserProfileEditPost(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	userID := userCtx.UserID
	formType := c.FormValue("form_type")

	// Get user from database
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		flash.WithError(c, fiber.Map{"message": "User not found"})
		return c.Redirect("/user/profile/edit")
	}

	switch formType {
	case "profile":
		return handleProfileUpdate(c, &user)
	case "password":
		return handlePasswordUpdate(c, &user)
	default:
		flash.WithError(c, fiber.Map{"message": "Ungültiger Formulartyp"})
		return c.Redirect("/user/profile/edit")
	}
}

func handleProfileUpdate(c *fiber.Ctx, user *models.User) error {
	newName := c.FormValue("name")
	newEmail := c.FormValue("email")

	// Validate input
	if newName == "" || newEmail == "" {
		flash.WithError(c, fiber.Map{"message": "Bitte alle Felder ausfüllen"})
		if c.Get("HX-Request") == "true" {
			c.Set("HX-Redirect", "/user/profile/edit")
			return c.SendStatus(fiber.StatusNoContent)
		}
		return c.Redirect("/user/profile/edit")
	}

	// Check if name is taken by another user (optional - names don't need to be unique)
	// Removing this check since names don't need to be unique like usernames

	// Check if email is taken by another user
	if newEmail != user.Email {
		var existingUser models.User
		if database.DB.Where("email = ? AND id != ?", newEmail, user.ID).First(&existingUser).Error == nil {
			flash.WithError(c, fiber.Map{"message": "E-Mail-Adresse bereits vergeben"})
			if c.Get("HX-Request") == "true" {
				c.Set("HX-Redirect", "/user/profile/edit")
				return c.SendStatus(fiber.StatusNoContent)
			}
			return c.Redirect("/user/profile/edit")
		}
	}

	// Update user data
	user.Name = newName

	// Handle email change securely
	emailChanged := user.Email != newEmail
	if emailChanged {
		// Don't change the actual email yet - store as pending
		user.PendingEmail = newEmail

		// Generate email change token
		if err := user.GenerateEmailChangeToken(); err != nil {
			flash.WithError(c, fiber.Map{"message": "Fehler beim Generieren des Bestätigungstokens"})
			if c.Get("HX-Request") == "true" {
				c.Set("HX-Redirect", "/user/profile/edit")
				return c.SendStatus(fiber.StatusNoContent)
			}
			return c.Redirect("/user/profile/edit")
		}

		// Send verification email to new address
		domain := env.GetEnv("PUBLIC_DOMAIN", "")
		verificationURL := fmt.Sprintf("%s/user/profile/verify-email-change?token=%s", domain, user.EmailChangeToken)
		rec := httptest.NewRecorder()
		templ.Handler(email_views.EmailChangeEmail(user.Email, user.PendingEmail, templ.SafeURL(verificationURL), user.EmailChangeToken)).ServeHTTP(rec, &http.Request{})
		body := rec.Body.String()

		go func() {
			if err := mail.SendMail(user.PendingEmail, "E-Mail-Adresse bestätigen - PIXELFOX.cc", body); err != nil {
				log.Printf("Email change verification email error: %v", err)
			}
		}()
	}

	if err := database.DB.Save(user).Error; err != nil {
		flash.WithError(c, fiber.Map{"message": "Fehler beim Speichern der Änderungen"})
		if c.Get("HX-Request") == "true" {
			c.Set("HX-Redirect", "/user/profile/edit")
			return c.SendStatus(fiber.StatusNoContent)
		}
		return c.Redirect("/user/profile/edit")
	}

	// Update session if name changed
	if newName != user.Name {
		sess, _ := session.GetSessionStore().Get(c)
		sess.Set(usercontext.KeyUsername, newName)
		sess.Save()
	}

	// Success response - always redirect with flash message for consistency
	successMsg := "Profil erfolgreich aktualisiert"
	if emailChanged {
		successMsg = "Profil aktualisiert! Bestätigungslink wurde an die neue E-Mail-Adresse gesendet."
	}
	flash.WithSuccess(c, fiber.Map{"message": successMsg})

	// For HTMX requests, return redirect instruction
	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", "/user/profile/edit")
		return c.SendStatus(fiber.StatusNoContent)
	}

	return c.Redirect("/user/profile/edit")
}

func handlePasswordUpdate(c *fiber.Ctx, user *models.User) error {
	currentPassword := c.FormValue("current_password")
	newPassword := c.FormValue("new_password")
	confirmPassword := c.FormValue("confirm_password")

	// Validate input
	if currentPassword == "" || newPassword == "" || confirmPassword == "" {
		flash.WithError(c, fiber.Map{"message": "Bitte alle Passwort-Felder ausfüllen"})
		if c.Get("HX-Request") == "true" {
			c.Set("HX-Redirect", "/user/profile/edit")
			return c.SendStatus(fiber.StatusNoContent)
		}
		return c.Redirect("/user/profile/edit")
	}

	// Check if new passwords match
	if newPassword != confirmPassword {
		flash.WithError(c, fiber.Map{"message": "Neue Passwörter stimmen nicht überein"})
		if c.Get("HX-Request") == "true" {
			c.Set("HX-Redirect", "/user/profile/edit")
			return c.SendStatus(fiber.StatusNoContent)
		}
		return c.Redirect("/user/profile/edit")
	}

	// Validate current password
	if !user.CheckPassword(currentPassword) {
		flash.WithError(c, fiber.Map{"message": "Aktuelles Passwort ist falsch"})
		if c.Get("HX-Request") == "true" {
			c.Set("HX-Redirect", "/user/profile/edit")
			return c.SendStatus(fiber.StatusNoContent)
		}
		return c.Redirect("/user/profile/edit")
	}

	// Update password
	if err := user.SetPassword(newPassword); err != nil {
		flash.WithError(c, fiber.Map{"message": "Fehler beim Setzen des neuen Passworts"})
		if c.Get("HX-Request") == "true" {
			c.Set("HX-Redirect", "/user/profile/edit")
			return c.SendStatus(fiber.StatusNoContent)
		}
		return c.Redirect("/user/profile/edit")
	}

	// Save to database
	if err := database.DB.Save(user).Error; err != nil {
		flash.WithError(c, fiber.Map{"message": "Fehler beim Speichern des neuen Passworts"})
		if c.Get("HX-Request") == "true" {
			c.Set("HX-Redirect", "/user/profile/edit")
			return c.SendStatus(fiber.StatusNoContent)
		}
		return c.Redirect("/user/profile/edit")
	}

	// Success response - always redirect with flash message for consistency
	flash.WithSuccess(c, fiber.Map{"message": "Passwort erfolgreich geändert"})

	// For HTMX requests, return redirect instruction
	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", "/user/profile/edit")
		return c.SendStatus(fiber.StatusNoContent)
	}

	return c.Redirect("/user/profile/edit")
}

// HandleEmailChangeVerification handles the email change verification from the link in email
func HandleEmailChangeVerification(c *fiber.Ctx) error {
	token := c.Query("token")
	if token == "" {
		flash.WithError(c, fiber.Map{"message": "Ungültiger Bestätigungslink"})
		return c.Redirect("/")
	}

	// Find user by token
	var user models.User
	result := database.DB.Where("email_change_token = ?", token).First(&user)
	if result.Error != nil {
		flash.WithError(c, fiber.Map{"message": "Ungültiger oder abgelaufener Bestätigungslink"})
		return c.Redirect("/")
	}

	// Verify token is still valid
	if !user.IsEmailChangeTokenValid(token) {
		flash.WithError(c, fiber.Map{"message": "Bestätigungslink ist abgelaufen"})
		return c.Redirect("/")
	}

	// Change email and clear pending change
	user.Email = user.PendingEmail
	user.ClearEmailChangeRequest()

	// Save changes
	if err := database.DB.Save(&user).Error; err != nil {
		flash.WithError(c, fiber.Map{"message": "Fehler beim Bestätigen der E-Mail-Adresse"})
		return c.Redirect("/")
	}

	flash.WithSuccess(c, fiber.Map{"message": "E-Mail-Adresse erfolgreich geändert!"})
	return c.Redirect("/user/profile")
}

// HandleCancelEmailChange allows user to cancel pending email change
func HandleCancelEmailChange(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	userID := userCtx.UserID

	// Get user from database
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		flash.WithError(c, fiber.Map{"message": "User not found"})
		return c.Redirect("/user/profile/edit")
	}

	// Clear pending email change
	user.ClearEmailChangeRequest()

	// Save changes
	if err := database.DB.Save(&user).Error; err != nil {
		flash.WithError(c, fiber.Map{"message": "Fehler beim Abbrechen der E-Mail-Änderung"})
		return c.Redirect("/user/profile/edit")
	}

	flash.WithSuccess(c, fiber.Map{"message": "E-Mail-Änderung abgebrochen"})
	return c.Redirect("/user/profile/edit")
}

// HandleResendEmailChange resends the email change verification email
func HandleResendEmailChange(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	userID := userCtx.UserID

	// Get user from database
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		flash.WithError(c, fiber.Map{"message": "User not found"})
		return c.Redirect("/user/profile/edit")
	}

	// Check if user has pending email change
	if !user.HasPendingEmailChange() {
		flash.WithError(c, fiber.Map{"message": "Keine ausstehende E-Mail-Änderung gefunden"})
		return c.Redirect("/user/profile/edit")
	}

	// Send verification email to new address
	domain := env.GetEnv("PUBLIC_DOMAIN", "")
	verificationURL := fmt.Sprintf("%s/user/profile/verify-email-change?token=%s", domain, user.EmailChangeToken)
	rec := httptest.NewRecorder()
	templ.Handler(email_views.EmailChangeEmail(user.Email, user.PendingEmail, templ.SafeURL(verificationURL), user.EmailChangeToken)).ServeHTTP(rec, &http.Request{})
	body := rec.Body.String()

	go func() {
		if err := mail.SendMail(user.PendingEmail, "E-Mail-Adresse bestätigen - PIXELFOX.cc", body); err != nil {
			log.Printf("Email change verification email error: %v", err)
		}
	}()

	flash.WithSuccess(c, fiber.Map{"message": "Bestätigungslink wurde erneut gesendet"})
	return c.Redirect("/user/profile/edit")
}
