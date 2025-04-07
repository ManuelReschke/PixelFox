package flash

import (
	"github.com/gofiber/fiber/v2"
)

// Flash message key in session
const FlashKey = "flash"

// Set sets a flash message in the session
func Set(c *fiber.Ctx, message fiber.Map) {
	c.Locals(FlashKey, message)
}

// Get retrieves the flash message from the session
func Get(c *fiber.Ctx) fiber.Map {
	flashMessage := c.Locals(FlashKey)
	if flashMessage == nil {
		return nil
	}

	return flashMessage.(fiber.Map)
}
