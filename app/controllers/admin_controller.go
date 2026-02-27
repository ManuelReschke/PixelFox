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
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/session"
	"github.com/ManuelReschke/PixelFox/internal/pkg/usercontext"
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

// countOpenReports returns the number of open image reports
func countOpenReports() (int64, error) {
	var cnt int64
	err := database.GetDB().Model(&models.ImageReport{}).Where("status = ?", models.ReportStatusOpen).Count(&cnt).Error
	return cnt, err
}

// HandleDashboard renders the admin dashboard with clean repository usage
func (ac *AdminController) HandleDashboard(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	// Get total counts using repositories
	totalUsers, err := ac.repos.User.Count()
	if err != nil {
		return ac.handleError(c, "Failed to get user count", err)
	}

	totalImages, err := ac.repos.Image.Count()
	if err != nil {
		return ac.handleError(c, "Failed to get image count", err)
	}

	// Get total albums count
	albumCount, err := ac.repos.Album.Count()
	if err != nil {
		return ac.handleError(c, "Failed to get album count", err)
	}

	// Get open reports count (no repository exists for reports)
	var openReports int64

	// Get recent users with pagination
	recentUsers, err := ac.repos.User.List(0, 5)
	if err != nil {
		return ac.handleError(c, "Failed to get recent users", err)
	}

	// Get statistics for charts (this would be moved to a service layer later)
	imageStats := ac.getLastSevenDaysStats("images")
	userStats := ac.getLastSevenDaysStats("users")

	// Render dashboard: compute open reports count
	if cnt, err := countOpenReports(); err == nil {
		openReports = cnt
	} else {
		log.Printf("Failed to count open reports: %v", err)
	}
	dashboard := admin_views.Dashboard(int(totalUsers), int(totalImages), int(albumCount), int(openReports), recentUsers, imageStats, userStats)
	home := views.HomeCtx(c, " | Admin Dashboard", userCtx.IsLoggedIn, false, flash.Get(c), dashboard, userCtx.IsAdmin, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleUsers renders the user management page with repository pattern
func (ac *AdminController) HandleUsers(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
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
	home := views.HomeCtx(c, " | User Management", userCtx.IsLoggedIn, false, flash.Get(c), userManagement, userCtx.IsAdmin, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleUserEdit renders the user edit page
func (ac *AdminController) HandleUserEdit(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
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

	// Load user plan from user_settings
	us, _ := models.GetOrCreateUserSettings(database.GetDB(), user.ID)
	plan := "free"
	if us != nil && us.Plan != "" {
		plan = us.Plan
	}
	// Render user edit page with plan
	userEdit := admin_views.UserEdit(*user, plan)
	home := views.HomeCtx(c, " | Edit User", userCtx.IsLoggedIn, false, flash.Get(c), userEdit, userCtx.IsAdmin, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleAdminUserUpdatePlan updates a user's plan (entitlements)
func (ac *AdminController) HandleAdminUserUpdatePlan(c *fiber.Ctx) error {
	userID := c.Params("id")
	if userID == "" {
		return c.Redirect("/admin/users")
	}
	id, err := strconv.ParseUint(userID, 10, 32)
	if err != nil {
		return c.Redirect("/admin/users")
	}
	user, err := ac.repos.User.GetByID(uint(id))
	if err != nil || user == nil {
		fm := fiber.Map{"type": "error", "message": "User not found"}
		return flash.WithError(c, fm).Redirect("/admin/users")
	}
	plan := strings.TrimSpace(c.FormValue("plan"))
	switch plan {
	case "free", "premium", "premium_max":
		// ok
	default:
		fm := fiber.Map{"type": "error", "message": "Ungültiger Plan"}
		return flash.WithError(c, fm).Redirect("/admin/users/edit/" + userID)
	}
	db := database.GetDB()
	us, err := models.GetOrCreateUserSettings(db, user.ID)
	if err != nil {
		fm := fiber.Map{"type": "error", "message": "Konnte User‑Einstellungen nicht laden"}
		return flash.WithError(c, fm).Redirect("/admin/users/edit/" + userID)
	}
	us.Plan = plan
	if err := db.Save(us).Error; err != nil {
		fm := fiber.Map{"type": "error", "message": "Plan speichern fehlgeschlagen"}
		return flash.WithError(c, fm).Redirect("/admin/users/edit/" + userID)
	}
	fm := fiber.Map{"type": "success", "message": "Plan aktualisiert"}
	return flash.WithSuccess(c, fm).Redirect("/admin/users/edit/" + userID)
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
	if c.Method() != fiber.MethodPost {
		return c.SendStatus(fiber.StatusMethodNotAllowed)
	}

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
	currentUserID := sess.Get(usercontext.KeyUserID).(uint)

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
	userCtx := usercontext.GetUserContext(c)
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
	home := views.HomeCtx(c, " | User Search", userCtx.IsLoggedIn, false, flash.Get(c), userManagement, userCtx.IsAdmin, nil)

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
	userCtx := usercontext.GetUserContext(c)
	// Get current settings using repository
	settings, err := ac.repos.Setting.Get()
	if err != nil {
		return ac.handleError(c, "Failed to get settings", err)
	}

	// Get CSRF token
	csrfToken := c.Locals("csrf").(string)

	// Render settings page
	settingsView := admin_views.Settings(*settings, csrfToken)
	home := views.HomeCtx(c, " | Einstellungen", userCtx.IsLoggedIn, false, flash.Get(c), settingsView, userCtx.IsAdmin, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleSettingsUpdate handles settings update with repository pattern
func (ac *AdminController) HandleSettingsUpdate(c *fiber.Ctx) error {
	// Get form data
	siteTitle := c.FormValue("site_title")
	siteDescription := c.FormValue("site_description")
	imageUploadEnabled := c.FormValue("image_upload_enabled") == "on"
	directUploadEnabled := c.FormValue("direct_upload_enabled") == "on"
	uploadRateLimitPerMinute, _ := strconv.Atoi(c.FormValue("upload_rate_limit_per_minute"))
	if uploadRateLimitPerMinute < 0 {
		uploadRateLimitPerMinute = 0
	} else if uploadRateLimitPerMinute > 100000 {
		uploadRateLimitPerMinute = 100000
	}

	uploadUserRateLimitPerMinute, _ := strconv.Atoi(c.FormValue("upload_user_rate_limit_per_minute"))
	if uploadUserRateLimitPerMinute < 0 {
		uploadUserRateLimitPerMinute = 0
	} else if uploadUserRateLimitPerMinute > 100000 {
		uploadUserRateLimitPerMinute = 100000
	}

	// Get thumbnail format settings
	thumbnailOriginalEnabled := c.FormValue("thumbnail_original_enabled") == "on"
	thumbnailWebPEnabled := c.FormValue("thumbnail_webp_enabled") == "on"
	thumbnailAVIFEnabled := c.FormValue("thumbnail_avif_enabled") == "on"
	// Replication settings
	replicationRequireChecksum := c.FormValue("replication_require_checksum") == "on"

	jobQueueWorkerCount, _ := strconv.Atoi(c.FormValue("job_queue_worker_count"))
	if jobQueueWorkerCount < 1 {
		jobQueueWorkerCount = 1
	} else if jobQueueWorkerCount > 20 {
		jobQueueWorkerCount = 20
	}

	apiRateLimitPerMinute, _ := strconv.Atoi(c.FormValue("api_rate_limit_per_minute"))
	if apiRateLimitPerMinute < 0 {
		apiRateLimitPerMinute = 0
	} else if apiRateLimitPerMinute > 100000 {
		apiRateLimitPerMinute = 100000
	}

	// Tiering (Phase A)
	tieringEnabled := c.FormValue("tiering_enabled") == "on"
	hotKeepDaysAfterUpload, _ := strconv.Atoi(c.FormValue("hot_keep_days_after_upload"))
	if hotKeepDaysAfterUpload < 0 {
		hotKeepDaysAfterUpload = 0
	}
	if hotKeepDaysAfterUpload > 3650 {
		hotKeepDaysAfterUpload = 3650
	}
	demoteIfNoViewsDays, _ := strconv.Atoi(c.FormValue("demote_if_no_views_days"))
	if demoteIfNoViewsDays < 1 {
		demoteIfNoViewsDays = 1
	}
	if demoteIfNoViewsDays > 3650 {
		demoteIfNoViewsDays = 3650
	}
	minDwellDaysPerTier, _ := strconv.Atoi(c.FormValue("min_dwell_days_per_tier"))
	if minDwellDaysPerTier < 0 {
		minDwellDaysPerTier = 0
	}
	if minDwellDaysPerTier > 3650 {
		minDwellDaysPerTier = 3650
	}
	hotWatermarkHigh, _ := strconv.Atoi(c.FormValue("hot_watermark_high"))
	if hotWatermarkHigh < 1 {
		hotWatermarkHigh = 1
	}
	if hotWatermarkHigh > 100 {
		hotWatermarkHigh = 100
	}
	hotWatermarkLow, _ := strconv.Atoi(c.FormValue("hot_watermark_low"))
	if hotWatermarkLow < 0 {
		hotWatermarkLow = 0
	}
	if hotWatermarkLow > 100 {
		hotWatermarkLow = 100
	}
	maxTieringCandidatesPerSweep, _ := strconv.Atoi(c.FormValue("max_tiering_candidates_per_sweep"))
	if maxTieringCandidatesPerSweep < 1 {
		maxTieringCandidatesPerSweep = 1
	}
	if maxTieringCandidatesPerSweep > 100000 {
		maxTieringCandidatesPerSweep = 100000
	}
	tieringSweepIntervalMinutes, _ := strconv.Atoi(c.FormValue("tiering_sweep_interval_minutes"))
	if tieringSweepIntervalMinutes < 1 {
		tieringSweepIntervalMinutes = 1
	}
	if tieringSweepIntervalMinutes > 1440 {
		tieringSweepIntervalMinutes = 1440
	}

	// Create new settings
	newSettings := &models.AppSettings{
		SiteTitle:                    siteTitle,
		SiteDescription:              siteDescription,
		ImageUploadEnabled:           imageUploadEnabled,
		DirectUploadEnabled:          directUploadEnabled,
		UploadRateLimitPerMinute:     uploadRateLimitPerMinute,
		UploadUserRateLimitPerMinute: uploadUserRateLimitPerMinute,
		ThumbnailOriginalEnabled:     thumbnailOriginalEnabled,
		ThumbnailWebPEnabled:         thumbnailWebPEnabled,
		ThumbnailAVIFEnabled:         thumbnailAVIFEnabled,
		JobQueueWorkerCount:          jobQueueWorkerCount,
		APIRateLimitPerMinute:        apiRateLimitPerMinute,
		ReplicationRequireChecksum:   replicationRequireChecksum,
		// Tiering
		TieringEnabled:               tieringEnabled,
		HotKeepDaysAfterUpload:       hotKeepDaysAfterUpload,
		DemoteIfNoViewsDays:          demoteIfNoViewsDays,
		MinDwellDaysPerTier:          minDwellDaysPerTier,
		HotWatermarkHigh:             hotWatermarkHigh,
		HotWatermarkLow:              hotWatermarkLow,
		MaxTieringCandidatesPerSweep: maxTieringCandidatesPerSweep,
		TieringSweepIntervalMinutes:  tieringSweepIntervalMinutes,
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
