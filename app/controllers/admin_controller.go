package controllers

import (
	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/flash"
	"github.com/ManuelReschke/PixelFox/internal/pkg/session"
	"github.com/ManuelReschke/PixelFox/views"
	"github.com/ManuelReschke/PixelFox/views/admin_views"

	"strconv"

	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
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

	// Render dashboard
	dashboard := admin_views.Dashboard(int(totalUsers), int(totalImages), recentUsers)
	home := views.Home(" | Admin Dashboard", getFromProtected(c), false, flash.Get(c), dashboard, true, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleAdminUsers renders the user management page
func HandleAdminUsers(c *fiber.Ctx) error {
	// Get all users
	db := database.GetDB()
	var users []models.User
	db.Order("created_at DESC").Find(&users)

	// Render user management page
	userManagement := admin_views.UserManagement(users)
	home := views.Home(" | User Management", getFromProtected(c), false, flash.Get(c), userManagement, true, nil)

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
	home := views.Home(" | Edit User", getFromProtected(c), false, flash.Get(c), userEdit, true, nil)

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
		flash.Set(c, fiber.Map{
			"type":    "error",
			"message": "Validation failed: " + err.Error(),
		})
		return c.Redirect("/admin/users/edit/" + userID)
	}

	// Save to database
	db.Save(&user)

	// Set success flash message
	flash.Set(c, fiber.Map{
		"type":    "success",
		"message": "User updated successfully",
	})

	return c.Redirect("/admin/users")
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
		flash.Set(c, fiber.Map{
			"type":    "error",
			"message": "You cannot delete your own account",
		})
		return c.Redirect("/admin/users")
	}

	// Delete user from database
	db := database.GetDB()
	db.Delete(&models.User{}, userID)

	// Set success flash message
	flash.Set(c, fiber.Map{
		"type":    "success",
		"message": "User deleted successfully",
	})

	return c.Redirect("/admin/users")
}
