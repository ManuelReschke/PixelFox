package controllers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/ManuelReschke/PixelFox/app/repository"
	"github.com/ManuelReschke/PixelFox/internal/pkg/usercontext"
)

// HandleGetImageResourceAPI returns the canonical image resource including direct links and variants
// Security: API Key required via router middleware
func HandleGetImageResourceAPI(c *fiber.Ctx) error {
	user := usercontext.GetUserContext(c)
	if !user.IsLoggedIn {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized", "message": "Missing or invalid authentication"})
	}

	uuid := c.Params("uuid")
	if uuid == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request", "message": "uuid missing"})
	}

	imgRepo := repository.GetGlobalFactory().GetImageRepository()
	image, err := imgRepo.GetByUUID(uuid)
	if err != nil || image == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not_found", "message": "image not found"})
	}
	// Access: owner or public
	if image.UserID != user.UserID && !image.IsPublic {
		// Do not leak existence
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not_found", "message": "image not found"})
	}

	payload := buildUploadResponseExtras(image)
	// Ensure no duplicate flag in resource view
	if _, ok := payload["duplicate"]; ok {
		delete(payload, "duplicate")
	}
	return c.JSON(payload)
}
