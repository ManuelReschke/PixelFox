package middleware

import (
	"github.com/ManuelReschke/PixelFox/app/controllers"
	"github.com/ManuelReschke/PixelFox/internal/pkg/session"
	"github.com/ManuelReschke/PixelFox/internal/pkg/usercontext"
	"github.com/gofiber/fiber/v2"
)

// UserContextMiddleware sets up the complete user context for every request
// This centralizes user session handling and eliminates code duplication
func UserContextMiddleware(c *fiber.Ctx) error {
	// Get session with error handling
	sess, err := session.GetSessionStore().Get(c)
	if err != nil {
		// On error: set as anonymous user
		c.Locals("USER_CONTEXT", usercontext.UserContext{
			IsLoggedIn: false,
			IsAdmin:    false,
		})
		// Set legacy compatibility locals
		c.Locals(controllers.FROM_PROTECTED, false)
		c.Locals(controllers.USER_IS_ADMIN, false)
		return c.Next()
	}

	// Get user ID from session
	userID := sess.Get(controllers.USER_ID)
	if userID == nil {
		// Anonymous user - no session data
		c.Locals("USER_CONTEXT", usercontext.UserContext{
			IsLoggedIn: false,
			IsAdmin:    false,
		})
		// Set legacy compatibility locals
		c.Locals(controllers.FROM_PROTECTED, false)
		c.Locals(controllers.USER_IS_ADMIN, false)
		return c.Next()
	}

	// User is logged in - get additional data
	username := session.GetSessionValue(c, controllers.USER_NAME)
	isAdmin := sess.Get(controllers.USER_IS_ADMIN)

	// Set complete user context
	userCtx := usercontext.UserContext{
		UserID:     userID.(uint),
		Username:   username,
		IsLoggedIn: true,
		IsAdmin:    isAdmin != nil && isAdmin.(bool),
	}
	c.Locals("USER_CONTEXT", userCtx)

	// Legacy compatibility - keep existing Locals for backward compatibility
	c.Locals(controllers.FROM_PROTECTED, true)
	c.Locals(controllers.USER_NAME, username)
	c.Locals(controllers.USER_ID, userID.(uint))
	c.Locals(controllers.USER_IS_ADMIN, userCtx.IsAdmin)

	// Store username in user's individual session (multi-user safe)
	session.SetSessionValue(c, controllers.USER_NAME, username)

	return c.Next()
}
