package controllers

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/sujit-baniya/flash"
	"gorm.io/gorm"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
	"github.com/ManuelReschke/PixelFox/internal/pkg/mail"
	"github.com/ManuelReschke/PixelFox/internal/pkg/session"
	"github.com/ManuelReschke/PixelFox/views"
	"github.com/ManuelReschke/PixelFox/views/admin_views"
	email_views "github.com/ManuelReschke/PixelFox/views/email_views"
)

// HandleAdminDashboard renders the admin dashboard
func HandleAdminDashboard(c *fiber.Ctx) error {
	// Get statistics for dashboard
	db := database.GetDB()

	// Count total users
	var totalUsers int64
	db.Model(&models.User{}).Count(&totalUsers)

	// Count total images
	var totalImages int64
	db.Model(&models.Image{}).Count(&totalImages)

	// Get recent users
	var recentUsers []models.User
	db.Order("created_at DESC").Limit(5).Find(&recentUsers)

	// Get data for charts - last 7 days
	imageStats := getLastSevenDaysStats(db, "images")
	userStats := getLastSevenDaysStats(db, "users")

	// Render dashboard
	dashboard := admin_views.Dashboard(int(totalUsers), int(totalImages), recentUsers, imageStats, userStats)
	home := views.Home(" | Admin Dashboard", isLoggedIn(c), false, flash.Get(c), dashboard, true, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// getLastSevenDaysStats returns statistics for the last 7 days
func getLastSevenDaysStats(db *gorm.DB, statsType string) []models.DailyStats {
	// Initialize result slice
	result := make([]models.DailyStats, 7)

	// Get current time
	now := time.Now()

	// Fill the result with dates for the last 7 days
	for i := 0; i < 7; i++ {
		date := now.AddDate(0, 0, -i)
		dateStr := date.Format("2006-01-02")
		result[6-i] = models.DailyStats{Date: dateStr, Count: 0}
	}

	// Query database based on stats type
	if statsType == "images" {
		// Get image counts for each day
		for i, stat := range result {
			startDate, _ := time.Parse("2006-01-02", stat.Date)
			endDate := startDate.AddDate(0, 0, 1)

			var count int64
			db.Model(&models.Image{}).
				Where("created_at >= ? AND created_at < ?", startDate, endDate).
				Count(&count)

			result[i].Count = int(count)
		}
	} else if statsType == "users" {
		// Get user counts for each day
		for i, stat := range result {
			startDate, _ := time.Parse("2006-01-02", stat.Date)
			endDate := startDate.AddDate(0, 0, 1)

			var count int64
			db.Model(&models.User{}).
				Where("created_at >= ? AND created_at < ?", startDate, endDate).
				Count(&count)

			result[i].Count = int(count)
		}
	}

	return result
}

// HandleAdminUsers renders the user management page
func HandleAdminUsers(c *fiber.Ctx) error {
	// Pagination for user management
	db := database.GetDB()
	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage := 20
	offset := (page - 1) * perPage
	// Count total users
	var totalUsers int64
	db.Model(&models.User{}).Count(&totalUsers)
	// Fetch users for current page
	var users []models.User
	db.Order("created_at DESC").Offset(offset).Limit(perPage).Find(&users)

	// Fetch statistics for each user
	var usersWithStats []admin_views.UserWithStats
	for _, user := range users {
		// Get user statistics
		var imageCount int64
		db.Model(&models.Image{}).Where("user_id = ?", user.ID).Count(&imageCount)

		var albumCount int64
		db.Model(&models.Album{}).Where("user_id = ?", user.ID).Count(&albumCount)

		// Calculate storage usage
		var totalStorage int64
		db.Model(&models.Image{}).Where("user_id = ?", user.ID).Select("SUM(file_size)").Row().Scan(&totalStorage)

		usersWithStats = append(usersWithStats, admin_views.UserWithStats{
			User:         user,
			ImageCount:   imageCount,
			AlbumCount:   albumCount,
			StorageUsage: totalStorage,
		})
	}

	// Generate pages slice
	totalPages := int(totalUsers) / perPage
	if int(totalUsers)%perPage > 0 {
		totalPages++
	}
	pages := make([]int, totalPages)
	for i := range pages {
		pages[i] = i + 1
	}

	// Render user management page
	userManagement := admin_views.UserManagement(usersWithStats, page, pages)
	home := views.Home(" | User Management", isLoggedIn(c), false, flash.Get(c), userManagement, true, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleAdminUserEdit renders the user edit page
func HandleAdminUserEdit(c *fiber.Ctx) error {
	// Get user ID from params
	userID := c.Params("id")
	if userID == "" {
		return c.Redirect("/admin/users")
	}

	// Get user from database
	db := database.GetDB()
	var user models.User
	result := db.First(&user, userID)

	if result.Error != nil {
		// User not found
		c.Status(fiber.StatusNotFound)
		return c.Redirect("/admin/users")
	}

	// Render user edit page
	userEdit := admin_views.UserEdit(user)
	home := views.Home(" | Edit User", isLoggedIn(c), false, flash.Get(c), userEdit, true, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleAdminUserUpdate handles the user update form submission
func HandleAdminUserUpdate(c *fiber.Ctx) error {
	// Get user ID from params
	userID := c.Params("id")
	if userID == "" {
		return c.Redirect("/admin/users")
	}

	// Get user from database
	db := database.GetDB()
	var user models.User
	result := db.First(&user, userID)

	if result.Error != nil {
		// User not found
		c.Status(fiber.StatusNotFound)
		return c.Redirect("/admin/users")
	}

	// Get form data
	name := c.FormValue("name")
	email := c.FormValue("email")
	role := c.FormValue("role")
	status := c.FormValue("status")

	// Update user
	user.Name = name
	user.Email = email
	user.Role = role
	user.Status = status

	// Validate and save
	err := user.Validate()
	if err != nil {
		// Validation failed
		fm := fiber.Map{
			"type":    "error",
			"message": "Validation failed: " + err.Error(),
		}
		return flash.WithError(c, fm).Redirect("/admin/users/edit/" + userID)
	}

	// Save to database
	db.Save(&user)

	// Set success flash message
	fm := fiber.Map{
		"type":    "success",
		"message": "User updated successfully",
	}

	return flash.WithSuccess(c, fm).Redirect("/admin/users")
}

// HandleAdminUserDelete handles user deletion
func HandleAdminUserDelete(c *fiber.Ctx) error {
	// Get user ID from params
	userID := c.Params("id")
	if userID == "" {
		return c.Redirect("/admin/users")
	}

	// Get current user ID from session to prevent self-deletion
	sess, _ := session.GetSessionStore().Get(c)
	currentUserID := sess.Get(USER_ID).(uint)

	// Convert current user ID to string for comparison
	currentUserIDStr := strconv.FormatUint(uint64(currentUserID), 10)

	if currentUserIDStr == userID {
		// Prevent self-deletion
		fm := fiber.Map{
			"type":    "error",
			"message": "You cannot delete your own account",
		}
		return flash.WithError(c, fm).Redirect("/admin/users")
	}

	// Delete user from database
	db := database.GetDB()
	db.Delete(&models.User{}, userID)

	// Set success flash message
	fm := fiber.Map{
		"type":    "success",
		"message": "User deleted successfully",
	}

	return flash.WithSuccess(c, fm).Redirect("/admin/users")
}

// HandleAdminResendActivation resends activation email to the user
func HandleAdminResendActivation(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Redirect("/admin/users")
	}
	db := database.GetDB()
	var user models.User
	if err := db.First(&user, id).Error; err != nil {
		return flash.WithError(c, fiber.Map{"type": "error", "message": "Benutzer nicht gefunden"}).Redirect("/admin/users")
	}
	// generate new token
	if err := user.GenerateActivationToken(); err != nil {
		log.Printf("Error generating activation token: %v", err)
	}
	if err := db.Save(&user).Error; err != nil {
		log.Printf("Error saving activation token: %v", err)
	}
	// send email
	domain := env.GetEnv("PUBLIC_DOMAIN", "")
	activationURL := fmt.Sprintf("%s/activate?token=%s", domain, user.ActivationToken)
	rec := httptest.NewRecorder()
	templ.Handler(email_views.ActivationEmail(user.Email, templ.SafeURL(activationURL), user.ActivationToken)).ServeHTTP(rec, &http.Request{})
	if err := mail.SendMail(user.Email, "Aktivierungslink PIXELFOX.cc", rec.Body.String()); err != nil {
		log.Printf("Activation email error: %v", err)
		return flash.WithError(c, fiber.Map{"type": "error", "message": "Aktivierungs-Mail konnte nicht gesendet werden"}).Redirect("/admin/users")
	}
	return flash.WithSuccess(c, fiber.Map{"type": "success", "message": "Aktivierungs-Mail wurde erneut versendet"}).Redirect("/admin/users")
}

// HandleAdminImages renders the image management page
func HandleAdminImages(c *fiber.Ctx) error {
	// Get pagination parameters
	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage := 20 // Number of images per page
	offset := (page - 1) * perPage

	// Get all images with pagination
	db := database.GetDB()
	var images []models.Image
	var totalImages int64

	// Count total images for pagination
	db.Model(&models.Image{}).Count(&totalImages)

	// Get images with user information
	db.Preload("User").Order("created_at DESC").Offset(offset).Limit(perPage).Find(&images)

	// Calculate pagination info
	totalPages := int(totalImages) / perPage
	if int(totalImages)%perPage > 0 {
		totalPages++
	}

	// Render image management page
	imageManagement := admin_views.ImageManagement(images, page, totalPages)
	home := views.Home(" | Image Management", isLoggedIn(c), false, flash.Get(c), imageManagement, true, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleAdminImageEdit renders the image edit page
func HandleAdminImageEdit(c *fiber.Ctx) error {
	// Get image ID from params
	imageUUID := c.Params("uuid")
	if imageUUID == "" {
		return c.Redirect("/admin/images")
	}

	// Get image from database
	db := database.GetDB()
	image, err := models.FindImageByUUID(db, imageUUID)
	if err != nil {
		// Image not found
		fm := fiber.Map{
			"type":    "error",
			"message": "Image not found",
		}
		return flash.WithError(c, fm).Redirect("/admin/images")
	}

	// Preload user information
	db.Model(image).Association("User").Find(&image.User)

	// Render image edit page
	imageEdit := admin_views.ImageEdit(*image)
	home := views.Home(" | Edit Image", isLoggedIn(c), false, flash.Get(c), imageEdit, true, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleAdminImageUpdate handles the image update form submission
func HandleAdminImageUpdate(c *fiber.Ctx) error {
	// Get image ID from params
	imageUUID := c.Params("uuid")
	if imageUUID == "" {
		return c.Redirect("/admin/images")
	}

	// Get image from database
	db := database.GetDB()
	image, err := models.FindImageByUUID(db, imageUUID)
	if err != nil {
		// Image not found
		fm := fiber.Map{
			"type":    "error",
			"message": "Image not found",
		}
		return flash.WithError(c, fm).Redirect("/admin/images")
	}

	// Get form data
	title := c.FormValue("title")
	description := c.FormValue("description")
	isPublicStr := c.FormValue("is_public")
	isPublic := isPublicStr == "on"

	// Update image
	image.Title = title
	image.Description = description
	image.IsPublic = isPublic

	// Save to database
	db.Save(image)

	// Set success flash message
	fm := fiber.Map{
		"type":    "success",
		"message": "Image updated successfully",
	}

	return flash.WithSuccess(c, fm).Redirect("/admin/images")
}

// HandleAdminImageDelete handles image deletion
func HandleAdminImageDelete(c *fiber.Ctx) error {
	// Get image ID from params
	imageUUID := c.Params("uuid")
	if imageUUID == "" {
		return c.Redirect("/admin/images")
	}

	// Get image from database
	db := database.GetDB()
	image, err := models.FindImageByUUID(db, imageUUID)
	if err != nil {
		// Image not found
		fm := fiber.Map{
			"type":    "error",
			"message": "Image not found",
		}
		return flash.WithError(c, fm).Redirect("/admin/images")
	}

	// Delete image files
	// First, get the original file path
	originalPath := filepath.Join(image.FilePath, image.FileName)
	// Delete the original file
	os.Remove(originalPath)

	// Delete all variant files
	variants, err := models.FindVariantsByImageID(db, image.ID)
	if err == nil {
		for _, variant := range variants {
			variantPath := filepath.Join(variant.FilePath, variant.FileName)
			if err := os.Remove(variantPath); err != nil && !os.IsNotExist(err) {
				log.Printf("Failed to delete variant file %s: %v", variantPath, err)
			}
		}

		// Delete variant records from database
		if err := models.DeleteVariantsByImageID(db, image.ID); err != nil {
			log.Printf("Failed to delete variant records for image %d: %v", image.ID, err)
		}
	}

	// Delete image from database
	db.Delete(image)

	// Set success flash message
	fm := fiber.Map{
		"type":    "success",
		"message": "Image deleted successfully",
	}

	return flash.WithSuccess(c, fm).Redirect("/admin/images")
}

// HandleAdminSearch handles search functionality for admin area
func HandleAdminSearch(c *fiber.Ctx) error {
	// Get search parameters
	searchType := c.Query("type", "users") // Default to users if not specified
	query := c.Query("q", "")
	query = strings.TrimSpace(query)

	// If no query provided, redirect back
	if query == "" {
		fm := fiber.Map{
			"type":    "error",
			"message": "Bitte gib einen Suchbegriff ein",
		}
		return flash.WithError(c, fm).Redirect("/admin/" + searchType)
	}

	db := database.GetDB()

	// Handle search based on type
	switch searchType {
	case "users":
		return handleUserSearch(c, db, query)
	case "images":
		return handleImageSearch(c, db, query)
	default:
		return c.Redirect("/admin")
	}
}

// handleUserSearch searches for users and displays results
func handleUserSearch(c *fiber.Ctx, db *gorm.DB, query string) error {
	// Search for users by name or email
	var users []models.User
	db.Where("name LIKE ? OR email LIKE ?", "%"+query+"%", "%"+query+"%").Find(&users)

	// Fetch statistics for each user
	var usersWithStats []admin_views.UserWithStats
	for _, user := range users {
		// Get user statistics
		var imageCount int64
		db.Model(&models.Image{}).Where("user_id = ?", user.ID).Count(&imageCount)

		var albumCount int64
		db.Model(&models.Album{}).Where("user_id = ?", user.ID).Count(&albumCount)

		// Calculate storage usage
		var totalStorage int64
		db.Model(&models.Image{}).Where("user_id = ?", user.ID).Select("SUM(file_size)").Row().Scan(&totalStorage)

		usersWithStats = append(usersWithStats, admin_views.UserWithStats{
			User:         user,
			ImageCount:   imageCount,
			AlbumCount:   albumCount,
			StorageUsage: totalStorage,
		})
	}

	// Set flash message with search info
	fm := fiber.Map{
		"type":    "info",
		"message": "Suchergebnisse für '" + query + "': " + strconv.Itoa(len(users)) + " Benutzer gefunden",
	}

	flash.WithInfo(c, fm)

	// Default pagination for search results
	currentPage := 1
	pages := []int{1}
	// Render user management page with search results
	userManagement := admin_views.UserManagement(usersWithStats, currentPage, pages)
	home := views.Home(" | Benutzersuche", isLoggedIn(c), false, flash.Get(c), userManagement, true, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// handleImageSearch searches for images and displays results
func handleImageSearch(c *fiber.Ctx, db *gorm.DB, query string) error {
	// Search for images by title, description or UUID
	var images []models.Image
	db.Where("title LIKE ? OR description LIKE ? OR uuid LIKE ?", "%"+query+"%", "%"+query+"%", "%"+query+"%").Find(&images)

	// Preload user information for each image
	for i := range images {
		db.Model(&images[i]).Association("User").Find(&images[i].User)
	}

	// Set flash message with search info
	fm := fiber.Map{
		"type":    "info",
		"message": "Suchergebnisse für '" + query + "': " + strconv.Itoa(len(images)) + " Bilder gefunden",
	}

	flash.WithInfo(c, fm)

	// Render image management page with search results
	imageManagement := admin_views.ImageManagement(images, 1, 1) // No pagination for search results
	home := views.Home(" | Bildersuche", isLoggedIn(c), false, flash.Get(c), imageManagement, true, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleAdminPages renders the page management page
func HandleAdminPages(c *fiber.Ctx) error {
	// Get all pages from database
	db := database.GetDB()
	pages, err := models.GetAllPages(db)
	if err != nil {
		// Handle error
		fm := fiber.Map{
			"type":    "error",
			"message": "Error loading pages",
		}
		return flash.WithError(c, fm).Redirect("/admin")
	}

	// Render page management page
	pageManagement := admin_views.PageManagement(pages)
	home := views.Home(" | Page Management", isLoggedIn(c), false, flash.Get(c), pageManagement, true, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleAdminPageCreate renders the page creation form
func HandleAdminPageCreate(c *fiber.Ctx) error {
	// Get CSRF token
	csrfToken := c.Locals("csrf").(string)

	// Render page creation form
	pageCreate := admin_views.PageEdit(models.Page{}, false, csrfToken)
	home := views.Home(" | Create Page", isLoggedIn(c), false, flash.Get(c), pageCreate, true, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleAdminPageStore handles page creation
func HandleAdminPageStore(c *fiber.Ctx) error {
	// Get form data
	title := c.FormValue("title")
	slug := c.FormValue("slug")
	content := c.FormValue("content")
	isActiveStr := c.FormValue("is_active")
	isActive := isActiveStr == "on"

	// Create new page
	page := models.Page{
		Title:    title,
		Slug:     slug,
		Content:  content,
		IsActive: isActive,
	}

	// Validate page
	err := page.Validate()
	if err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "Validation failed: " + err.Error(),
		}
		return flash.WithError(c, fm).Redirect("/admin/pages/create")
	}

	// Save to database
	db := database.GetDB()
	result := db.Create(&page)
	if result.Error != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "Error creating page: " + result.Error.Error(),
		}
		return flash.WithError(c, fm).Redirect("/admin/pages/create")
	}

	// Set success flash message
	fm := fiber.Map{
		"type":    "success",
		"message": "Page created successfully",
	}

	return flash.WithSuccess(c, fm).Redirect("/admin/pages")
}

// HandleAdminPageEdit renders the page edit form
func HandleAdminPageEdit(c *fiber.Ctx) error {
	// Get page ID from params
	pageID := c.Params("id")
	if pageID == "" {
		return c.Redirect("/admin/pages")
	}

	// Convert ID to uint
	id, err := strconv.ParseUint(pageID, 10, 32)
	if err != nil {
		return c.Redirect("/admin/pages")
	}

	// Get page from database
	db := database.GetDB()
	page, err := models.FindPageByID(db, uint(id))
	if err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "Page not found",
		}
		return flash.WithError(c, fm).Redirect("/admin/pages")
	}

	// Get CSRF token
	csrfToken := c.Locals("csrf").(string)

	// Render page edit form
	pageEdit := admin_views.PageEdit(*page, true, csrfToken)
	home := views.Home(" | Edit Page", isLoggedIn(c), false, flash.Get(c), pageEdit, true, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleAdminPageUpdate handles page update
func HandleAdminPageUpdate(c *fiber.Ctx) error {
	// Get page ID from params
	pageID := c.Params("id")
	if pageID == "" {
		return c.Redirect("/admin/pages")
	}

	// Convert ID to uint
	id, err := strconv.ParseUint(pageID, 10, 32)
	if err != nil {
		return c.Redirect("/admin/pages")
	}

	// Get page from database
	db := database.GetDB()
	page, err := models.FindPageByID(db, uint(id))
	if err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "Page not found",
		}
		return flash.WithError(c, fm).Redirect("/admin/pages")
	}

	// Get form data
	title := c.FormValue("title")
	slug := c.FormValue("slug")
	content := c.FormValue("content")
	isActiveStr := c.FormValue("is_active")
	isActive := isActiveStr == "on"

	// Update page
	page.Title = title
	page.Slug = slug
	page.Content = content
	page.IsActive = isActive

	// Validate page
	err = page.Validate()
	if err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "Validation failed: " + err.Error(),
		}
		return flash.WithError(c, fm).Redirect("/admin/pages/edit/" + pageID)
	}

	// Save to database
	db.Save(page)

	// Set success flash message
	fm := fiber.Map{
		"type":    "success",
		"message": "Page updated successfully",
	}

	return flash.WithSuccess(c, fm).Redirect("/admin/pages")
}

// HandleAdminPageDelete handles page deletion
func HandleAdminPageDelete(c *fiber.Ctx) error {
	// Get page ID from params
	pageID := c.Params("id")
	if pageID == "" {
		return c.Redirect("/admin/pages")
	}

	// Convert ID to uint
	id, err := strconv.ParseUint(pageID, 10, 32)
	if err != nil {
		return c.Redirect("/admin/pages")
	}

	// Delete page from database
	db := database.GetDB()
	db.Delete(&models.Page{}, uint(id))

	// Set success flash message
	fm := fiber.Map{
		"type":    "success",
		"message": "Page deleted successfully",
	}

	return flash.WithSuccess(c, fm).Redirect("/admin/pages")
}

// HandleAdminSettings renders the settings page
func HandleAdminSettings(c *fiber.Ctx) error {
	// Get current settings
	settings := models.GetAppSettings()

	// Get CSRF token
	csrfToken := c.Locals("csrf").(string)

	// Render settings page
	settingsView := admin_views.Settings(*settings, csrfToken)
	home := views.Home(" | Einstellungen", isLoggedIn(c), false, fiber.Map(flash.Get(c)), settingsView, true, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleAdminSettingsUpdate handles settings update
func HandleAdminSettingsUpdate(c *fiber.Ctx) error {
	// Get form data
	siteTitle := c.FormValue("site_title")
	siteDescription := c.FormValue("site_description")
	imageUploadEnabledStr := c.FormValue("image_upload_enabled")
	imageUploadEnabled := imageUploadEnabledStr == "on"

	// Get thumbnail format settings
	thumbnailOriginalEnabledStr := c.FormValue("thumbnail_original_enabled")
	thumbnailOriginalEnabled := thumbnailOriginalEnabledStr == "on"

	thumbnailWebPEnabledStr := c.FormValue("thumbnail_webp_enabled")
	thumbnailWebPEnabled := thumbnailWebPEnabledStr == "on"

	thumbnailAVIFEnabledStr := c.FormValue("thumbnail_avif_enabled")
	thumbnailAVIFEnabled := thumbnailAVIFEnabledStr == "on"

	// Create new settings
	newSettings := &models.AppSettings{
		SiteTitle:                siteTitle,
		SiteDescription:          siteDescription,
		ImageUploadEnabled:       imageUploadEnabled,
		ThumbnailOriginalEnabled: thumbnailOriginalEnabled,
		ThumbnailWebPEnabled:     thumbnailWebPEnabled,
		ThumbnailAVIFEnabled:     thumbnailAVIFEnabled,
	}

	// Save settings
	db := database.GetDB()
	err := models.SaveSettings(db, newSettings)
	if err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "Fehler beim Speichern der Einstellungen: " + err.Error(),
		}
		return flash.WithError(c, fm).Redirect("/admin/settings")
	}

	// Set success flash message
	fm := fiber.Map{
		"type":    "success",
		"message": "Einstellungen erfolgreich gespeichert",
	}

	return flash.WithSuccess(c, fm).Redirect("/admin/settings")
}
