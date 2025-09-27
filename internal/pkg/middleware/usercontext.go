package middleware

import (
	"strings"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/session"
	"github.com/ManuelReschke/PixelFox/internal/pkg/usercontext"
	"github.com/gofiber/fiber/v2"
)

// UserContextMiddleware sets up the complete user context for every request
// This centralizes user session handling and eliminates code duplication
func UserContextMiddleware(c *fiber.Ctx) error {
	// Avoid interfering with Goth/Fiber session handling on OAuth routes.
	// Goth uses its own fiber session store and relies on per-request locals.
	// We skip our app session on /auth/* to prevent cross-store collisions.
	if strings.HasPrefix(c.Path(), "/auth/") {
		return c.Next()
	}
	// Get session with error handling
	sess, err := session.GetSessionStore().Get(c)
	if err != nil {
		// On error: set as anonymous user
		c.Locals("USER_CONTEXT", usercontext.UserContext{
			IsLoggedIn: false,
			IsAdmin:    false,
		})
		// Set legacy compatibility locals
		c.Locals(usercontext.KeyFromProtected, false)
		c.Locals(usercontext.KeyIsAdmin, false)
		return c.Next()
	}

	// Get user ID from session
	userID := sess.Get(usercontext.KeyUserID)
	if userID == nil {
		// Anonymous user - no session data
		c.Locals("USER_CONTEXT", usercontext.UserContext{
			IsLoggedIn: false,
			IsAdmin:    false,
		})
		// Set legacy compatibility locals
		c.Locals(usercontext.KeyFromProtected, false)
		c.Locals(usercontext.KeyIsAdmin, false)
		return c.Next()
	}

	// User is logged in - get additional data
	username := session.GetSessionValue(c, usercontext.KeyUsername)
	isAdminRaw := sess.Get(usercontext.KeyIsAdmin)
	isAdminBool := false
	if b, ok := isAdminRaw.(bool); ok {
		isAdminBool = b
	}

	// Determine plan with session-first strategy
	plan := session.GetSessionValue(c, "user_plan")
	if plan == "" {
		plan = "free"
		if db := database.GetDB(); db != nil {
			if us, err := models.GetOrCreateUserSettings(db, userID.(uint)); err == nil && us != nil && us.Plan != "" {
				plan = us.Plan
			}
		}
		// cache in session for subsequent requests
		_ = session.SetSessionValue(c, "user_plan", plan)
	}
	// Set complete user context
	userCtx := usercontext.UserContext{
		UserID:     userID.(uint),
		Username:   username,
		IsLoggedIn: true,
		IsAdmin:    isAdminBool,
		Plan:       plan,
	}
	c.Locals("USER_CONTEXT", userCtx)

	// Legacy compatibility - keep existing Locals for backward compatibility
	c.Locals(usercontext.KeyFromProtected, true)
	c.Locals(usercontext.KeyUsername, username)
	c.Locals(usercontext.KeyUserID, userID.(uint))
	c.Locals(usercontext.KeyIsAdmin, userCtx.IsAdmin)

	// Store username in user's individual session (multi-user safe)
	session.SetSessionValue(c, usercontext.KeyUsername, username)

	return c.Next()
}
