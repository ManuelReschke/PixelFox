package controllers

import "github.com/gofiber/fiber/v2"

func getFromProtected(c *fiber.Ctx) bool {
	var fromProtected bool
	if protectedValue := c.Locals(FROM_PROTECTED); protectedValue != nil {
		fromProtected = protectedValue.(bool)
	}

	return fromProtected
}
