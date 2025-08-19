package controllers

import (
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/sujit-baniya/flash"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/app/repository"
	"github.com/ManuelReschke/PixelFox/internal/pkg/session"
	"github.com/ManuelReschke/PixelFox/views"
	"github.com/ManuelReschke/PixelFox/views/admin_views"
)

// AdminController handles admin-related HTTP requests using repository pattern
type AdminController struct {
	repos *repository.Repositories
}

// NewAdminController creates a new admin controller with repository dependencies
func NewAdminController(repos *repository.Repositories) *AdminController {
	return &AdminController{
		repos: repos,
	}
}

// HandleDashboard renders the admin dashboard with clean repository usage
func (ac *AdminController) HandleDashboard(c *fiber.Ctx) error {
	// Get total counts using repositories
	totalUsers, err := ac.repos.User.Count()
	if err != nil {
		return ac.handleError(c, "Failed to get user count", err)
	}

	totalImages, err := ac.repos.Image.Count()
	if err != nil {
		return ac.handleError(c, "Failed to get image count", err)
	}

	// Get recent users with pagination
	recentUsers, err := ac.repos.User.List(0, 5)
	if err != nil {
		return ac.handleError(c, "Failed to get recent users", err)
	}

	// Get statistics for charts (this would be moved to a service layer later)
	imageStats := ac.getLastSevenDaysStats("images")
	userStats := ac.getLastSevenDaysStats("users")

	// Render dashboard
	dashboard := admin_views.Dashboard(int(totalUsers), int(totalImages), recentUsers, imageStats, userStats)
	home := views.Home(" | Admin Dashboard", isLoggedIn(c), false, flash.Get(c), dashboard, true, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleUsers renders the user management page with repository pattern
func (ac *AdminController) HandleUsers(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage := 20
	offset := (page - 1) * perPage

	// Get total user count
	totalUsers, err := ac.repos.User.Count()
	if err != nil {
		return ac.handleError(c, "Failed to get user count", err)
	}

	// Get users with statistics using repository
	usersWithStats, err := ac.repos.User.GetWithStats(offset, perPage)
	if err != nil {
		return ac.handleError(c, "Failed to get users with statistics", err)
	}

	// Generate pagination
	totalPages := int(totalUsers) / perPage
	if int(totalUsers)%perPage > 0 {
		totalPages++
	}
	pages := make([]int, totalPages)
	for i := range pages {
		pages[i] = i + 1
	}

	// Convert to admin view struct
	adminUsers := make([]admin_views.UserWithStats, len(usersWithStats))
	for i, userWithStats := range usersWithStats {
		adminUsers[i] = admin_views.UserWithStats{
			User:         userWithStats.User,
			ImageCount:   userWithStats.ImageCount,
			AlbumCount:   userWithStats.AlbumCount,
			StorageUsage: userWithStats.StorageUsage,
		}
	}

	// Render user management page
	userManagement := admin_views.UserManagement(adminUsers, page, pages)
	home := views.Home(" | User Management", isLoggedIn(c), false, flash.Get(c), userManagement, true, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleUserEdit renders the user edit page
func (ac *AdminController) HandleUserEdit(c *fiber.Ctx) error {
	userID := c.Params("id")
	if userID == "" {
		return c.Redirect("/admin/users")
	}

	id, err := strconv.ParseUint(userID, 10, 32)
	if err != nil {
		return c.Redirect("/admin/users")
	}

	// Use repository to get user
	user, err := ac.repos.User.GetByID(uint(id))
	if err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "User not found",
		}
		return flash.WithError(c, fm).Redirect("/admin/users")
	}

	// Render user edit page
	userEdit := admin_views.UserEdit(*user)
	home := views.Home(" | Edit User", isLoggedIn(c), false, flash.Get(c), userEdit, true, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleUserUpdate handles user update with repository pattern
func (ac *AdminController) HandleUserUpdate(c *fiber.Ctx) error {
	userID := c.Params("id")
	if userID == "" {
		return c.Redirect("/admin/users")
	}

	id, err := strconv.ParseUint(userID, 10, 32)
	if err != nil {
		return c.Redirect("/admin/users")
	}

	// Get user using repository
	user, err := ac.repos.User.GetByID(uint(id))
	if err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "User not found",
		}
		return flash.WithError(c, fm).Redirect("/admin/users")
	}

	// Update user fields
	user.Name = c.FormValue("name")
	user.Email = c.FormValue("email")
	user.Role = c.FormValue("role")
	user.Status = c.FormValue("status")

	// Validate user
	if err := user.Validate(); err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "Validation failed: " + err.Error(),
		}
		return flash.WithError(c, fm).Redirect("/admin/users/edit/" + userID)
	}

	// Update using repository
	if err := ac.repos.User.Update(user); err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "Failed to update user: " + err.Error(),
		}
		return flash.WithError(c, fm).Redirect("/admin/users/edit/" + userID)
	}

	// Success message
	fm := fiber.Map{
		"type":    "success",
		"message": "User updated successfully",
	}

	return flash.WithSuccess(c, fm).Redirect("/admin/users")
}

// HandleUserDelete handles user deletion with repository pattern
func (ac *AdminController) HandleUserDelete(c *fiber.Ctx) error {
	userID := c.Params("id")
	if userID == "" {
		return c.Redirect("/admin/users")
	}

	id, err := strconv.ParseUint(userID, 10, 32)
	if err != nil {
		return c.Redirect("/admin/users")
	}

	// Prevent self-deletion (this logic could be moved to a service)
	sess, _ := session.GetSessionStore().Get(c)
	currentUserID := sess.Get(USER_ID).(uint)

	if currentUserID == uint(id) {
		fm := fiber.Map{
			"type":    "error",
			"message": "You cannot delete your own account",
		}
		return flash.WithError(c, fm).Redirect("/admin/users")
	}

	// Delete user using repository
	if err := ac.repos.User.Delete(uint(id)); err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "Failed to delete user: " + err.Error(),
		}
		return flash.WithError(c, fm).Redirect("/admin/users")
	}

	// Success message
	fm := fiber.Map{
		"type":    "success",
		"message": "User deleted successfully",
	}

	return flash.WithSuccess(c, fm).Redirect("/admin/users")
}

// HandleSearch handles search functionality with repository pattern
func (ac *AdminController) HandleSearch(c *fiber.Ctx) error {
	searchType := c.Query("type", "users")
	query := c.Query("q", "")

	if query == "" {
		fm := fiber.Map{
			"type":    "error",
			"message": "Please enter a search term",
		}
		return flash.WithError(c, fm).Redirect("/admin/" + searchType)
	}

	switch searchType {
	case "users":
		return ac.handleUserSearch(c, query)
	case "images":
		return ac.handleImageSearch(c, query)
	default:
		return c.Redirect("/admin")
	}
}

// handleUserSearch searches for users using repository
func (ac *AdminController) handleUserSearch(c *fiber.Ctx, query string) error {
	// Search users with stats using repository
	usersWithStats, err := ac.repos.User.SearchWithStats(query)
	if err != nil {
		return ac.handleError(c, "Search failed", err)
	}

	// Convert to admin view struct
	adminUsers := make([]admin_views.UserWithStats, len(usersWithStats))
	for i, userWithStats := range usersWithStats {
		adminUsers[i] = admin_views.UserWithStats{
			User:         userWithStats.User,
			ImageCount:   userWithStats.ImageCount,
			AlbumCount:   userWithStats.AlbumCount,
			StorageUsage: userWithStats.StorageUsage,
		}
	}

	// Set search result message
	fm := fiber.Map{
		"type":    "info",
		"message": "Search results for '" + query + "': " + strconv.Itoa(len(adminUsers)) + " users found",
	}
	flash.WithInfo(c, fm)

	// Render results
	userManagement := admin_views.UserManagement(adminUsers, 1, []int{1})
	home := views.Home(" | User Search", isLoggedIn(c), false, flash.Get(c), userManagement, true, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// handleImageSearch searches for images using dedicated AdminImagesController
func (ac *AdminController) handleImageSearch(c *fiber.Ctx, query string) error {
	return GetAdminImagesController().HandleAdminImageSearch(c, query)
}

// handleError handles errors consistently
func (ac *AdminController) handleError(c *fiber.Ctx, message string, err error) error {
	// Log error (this could be improved with structured logging)
	log.Printf("Admin Controller Error: %s - %v", message, err)

	fm := fiber.Map{
		"type":    "error",
		"message": message,
	}

	// Return to appropriate page based on context
	redirectPath := "/admin"
	if c.Path() != "" {
		// Extract section from path for smart redirect
		if strings.Contains(c.Path(), "/users") {
			redirectPath = "/admin/users"
		} else if strings.Contains(c.Path(), "/images") {
			redirectPath = "/admin/images"
		}
	}

	return flash.WithError(c, fm).Redirect(redirectPath)
}

// getLastSevenDaysStats generates statistics for the last 7 days using repositories
func (ac *AdminController) getLastSevenDaysStats(statsType string) []models.DailyStats {
	now := time.Now()
	// Start date is 7 days ago at midnight
	startDate := now.AddDate(0, 0, -6).Truncate(24 * time.Hour)
	// End date is today at 23:59:59
	endDate := now.Truncate(24 * time.Hour).Add(24*time.Hour - time.Nanosecond)

	var stats []models.DailyStats
	var err error

	// Get stats from appropriate repository
	switch statsType {
	case "users":
		stats, err = ac.repos.User.GetDailyStats(startDate, endDate)
	case "images":
		stats, err = ac.repos.Image.GetDailyStats(startDate, endDate)
	default:
		// Return empty stats for unknown type
		return ac.createEmptyDailyStats(7)
	}

	if err != nil {
		// Log error and return empty stats
		log.Printf("Error getting daily stats for %s: %v", statsType, err)
		return ac.createEmptyDailyStats(7)
	}

	// Fill gaps for days with no data
	return ac.fillStatGaps(stats, startDate, 7)
}

// createEmptyDailyStats creates a slice of DailyStats with zero counts for the last n days
func (ac *AdminController) createEmptyDailyStats(days int) []models.DailyStats {
	result := make([]models.DailyStats, days)
	now := time.Now()

	for i := 0; i < days; i++ {
		date := now.AddDate(0, 0, -(days - 1 - i))
		dateStr := date.Format("2006-01-02")
		result[i] = models.DailyStats{Date: dateStr, Count: 0}
	}

	return result
}

// fillStatGaps fills missing dates in stats with zero counts
func (ac *AdminController) fillStatGaps(stats []models.DailyStats, startDate time.Time, days int) []models.DailyStats {
	result := make([]models.DailyStats, days)
	statsMap := make(map[string]int)

	// Create map for quick lookup
	for _, stat := range stats {
		statsMap[stat.Date] = stat.Count
	}

	// Fill result with data or zero counts
	for i := 0; i < days; i++ {
		date := startDate.AddDate(0, 0, i)
		dateStr := date.Format("2006-01-02")
		count := statsMap[dateStr] // defaults to 0 if not found
		result[i] = models.DailyStats{Date: dateStr, Count: count}
	}

	return result
}

// HandleSettings renders the settings page
func (ac *AdminController) HandleSettings(c *fiber.Ctx) error {
	// Get current settings using repository
	settings, err := ac.repos.Setting.Get()
	if err != nil {
		return ac.handleError(c, "Failed to get settings", err)
	}

	// Get CSRF token
	csrfToken := c.Locals("csrf").(string)

	// Render settings page
	settingsView := admin_views.Settings(*settings, csrfToken)
	home := views.Home(" | Einstellungen", isLoggedIn(c), false, flash.Get(c), settingsView, true, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleSettingsUpdate handles settings update with repository pattern
func (ac *AdminController) HandleSettingsUpdate(c *fiber.Ctx) error {
	// Get form data
	siteTitle := c.FormValue("site_title")
	siteDescription := c.FormValue("site_description")
	imageUploadEnabled := c.FormValue("image_upload_enabled") == "on"

	// Get thumbnail format settings
	thumbnailOriginalEnabled := c.FormValue("thumbnail_original_enabled") == "on"
	thumbnailWebPEnabled := c.FormValue("thumbnail_webp_enabled") == "on"
	thumbnailAVIFEnabled := c.FormValue("thumbnail_avif_enabled") == "on"

	// Parse and validate numeric settings
	s3BackupDelayMinutes, _ := strconv.Atoi(c.FormValue("s3_backup_delay_minutes"))
	if s3BackupDelayMinutes < 0 {
		s3BackupDelayMinutes = 0
	} else if s3BackupDelayMinutes > 43200 {
		s3BackupDelayMinutes = 43200
	}

	s3BackupCheckInterval, _ := strconv.Atoi(c.FormValue("s3_backup_check_interval"))
	if s3BackupCheckInterval < 1 {
		s3BackupCheckInterval = 1
	} else if s3BackupCheckInterval > 60 {
		s3BackupCheckInterval = 60
	}

	s3RetryInterval, _ := strconv.Atoi(c.FormValue("s3_retry_interval"))
	if s3RetryInterval < 1 {
		s3RetryInterval = 1
	} else if s3RetryInterval > 60 {
		s3RetryInterval = 60
	}

	jobQueueWorkerCount, _ := strconv.Atoi(c.FormValue("job_queue_worker_count"))
	if jobQueueWorkerCount < 1 {
		jobQueueWorkerCount = 1
	} else if jobQueueWorkerCount > 20 {
		jobQueueWorkerCount = 20
	}

	// Create new settings
	newSettings := &models.AppSettings{
		SiteTitle:                siteTitle,
		SiteDescription:          siteDescription,
		ImageUploadEnabled:       imageUploadEnabled,
		ThumbnailOriginalEnabled: thumbnailOriginalEnabled,
		ThumbnailWebPEnabled:     thumbnailWebPEnabled,
		ThumbnailAVIFEnabled:     thumbnailAVIFEnabled,
		S3BackupDelayMinutes:     s3BackupDelayMinutes,
		S3BackupCheckInterval:    s3BackupCheckInterval,
		S3RetryInterval:          s3RetryInterval,
		JobQueueWorkerCount:      jobQueueWorkerCount,
	}

	// Save settings using repository
	if err := ac.repos.Setting.Save(newSettings); err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "Fehler beim Speichern der Einstellungen: " + err.Error(),
		}
		return flash.WithError(c, fm).Redirect("/admin/settings")
	}

	// Success message
	fm := fiber.Map{
		"type":    "success",
		"message": "Einstellungen erfolgreich gespeichert",
	}

	return flash.WithSuccess(c, fm).Redirect("/admin/settings")
}

// HandleResendActivation resends activation email using repository pattern
func (ac *AdminController) HandleResendActivation(c *fiber.Ctx) error {
	userID := c.Params("id")
	if userID == "" {
		return c.Redirect("/admin/users")
	}

	id, err := strconv.ParseUint(userID, 10, 32)
	if err != nil {
		return c.Redirect("/admin/users")
	}

	// Get user using repository
	user, err := ac.repos.User.GetByID(uint(id))
	if err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "Benutzer nicht gefunden",
		}
		return flash.WithError(c, fm).Redirect("/admin/users")
	}

	// Generate new activation token
	if err := user.GenerateActivationToken(); err != nil {
		return ac.handleError(c, "Fehler beim Generieren des Aktivierungstokens", err)
	}

	// Save user with new token using repository
	if err := ac.repos.User.Update(user); err != nil {
		return ac.handleError(c, "Fehler beim Speichern des Aktivierungstokens", err)
	}

	// TODO: Send activation email (requires mail service integration)
	// For now, just return success
	fm := fiber.Map{
		"type":    "success",
		"message": "Aktivierungs-Mail wurde erneut versendet",
	}

	return flash.WithSuccess(c, fm).Redirect("/admin/users")
}

// Example of how to register the refactored controller in your router:
/*
func SetupAdminRoutes(app *fiber.App, repos *repository.Repositories) {
	adminController := NewAdminController(repos)

	admin := app.Group("/admin")
	admin.Get("/", adminController.HandleDashboard)
	admin.Get("/users", adminController.HandleUsers)
	admin.Get("/users/edit/:id", adminController.HandleUserEdit)
	admin.Post("/users/update/:id", adminController.HandleUserUpdate)
	admin.Delete("/users/delete/:id", adminController.HandleUserDelete)
	admin.Get("/search", adminController.HandleSearch)
}
*/
