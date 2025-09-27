package middleware

import (
	icuser "github.com/ManuelReschke/PixelFox/internal/pkg/usercontext"
	"github.com/gofiber/fiber/v2"
)

// RequireAuth ensures a logged-in web session; redirects to /login if missing.
func RequireAuth(c *fiber.Ctx) error {
	v := c.Locals(icuser.KeyFromProtected)
	loggedIn := false
	if b, ok := v.(bool); ok {
		loggedIn = b
	}
	if !loggedIn {
		return c.Redirect("/login", fiber.StatusSeeOther)
	}
	return c.Next()
}

// RequireAdmin ensures a logged-in admin; redirects otherwise.
func RequireAdmin(c *fiber.Ctx) error {
	v := c.Locals(icuser.KeyFromProtected)
	loggedIn := false
	if b, ok := v.(bool); ok {
		loggedIn = b
	}
	if !loggedIn {
		return c.Redirect("/login", fiber.StatusSeeOther)
	}
	if isAdmin, ok := c.Locals(icuser.KeyIsAdmin).(bool); !ok || !isAdmin {
		return c.Redirect("/", fiber.StatusSeeOther)
	}
	return c.Next()
}

// RequireAPISessionAuth ensures a logged-in session for API routes and returns JSON 401 instead of redirect.
func RequireAPISessionAuth(c *fiber.Ctx) error {
	v := c.Locals(icuser.KeyFromProtected)
	loggedIn := false
	if b, ok := v.(bool); ok {
		loggedIn = b
	}
	if !loggedIn {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   "unauthorized",
			"message": "login required",
		})
	}
	return c.Next()
}
