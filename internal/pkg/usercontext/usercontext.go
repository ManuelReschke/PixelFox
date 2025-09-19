package usercontext

import "github.com/gofiber/fiber/v2"

// UserContext represents the complete user context for a request
type UserContext struct {
	UserID     uint   `json:"user_id"`
	Username   string `json:"username"`
	IsLoggedIn bool   `json:"is_logged_in"`
	IsAdmin    bool   `json:"is_admin"`
	Plan       string `json:"plan"`
}

// GetUserContext retrieves the user context from fiber context
// Returns a default anonymous context if none is set
func GetUserContext(c *fiber.Ctx) UserContext {
	if ctx := c.Locals("USER_CONTEXT"); ctx != nil {
		return ctx.(UserContext)
	}
	return UserContext{IsLoggedIn: false, IsAdmin: false}
}

// IsLoggedIn checks if the current user is logged in
func IsLoggedIn(c *fiber.Ctx) bool {
	return GetUserContext(c).IsLoggedIn
}

// IsAdmin checks if the current user is an admin
func IsAdmin(c *fiber.Ctx) bool {
	return GetUserContext(c).IsAdmin
}

// GetUserID returns the current user's ID, or 0 if not logged in
func GetUserID(c *fiber.Ctx) uint {
	return GetUserContext(c).UserID
}

// GetUsername returns the current user's username, or empty string if not logged in
func GetUsername(c *fiber.Ctx) string {
	return GetUserContext(c).Username
}
