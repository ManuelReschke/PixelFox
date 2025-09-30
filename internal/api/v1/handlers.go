package apiv1

import (
	"github.com/gofiber/fiber/v2"

	// Delegate to existing controllers to keep behavior consistent
	"github.com/ManuelReschke/PixelFox/app/controllers"
	"github.com/ManuelReschke/PixelFox/app/repository"
	"github.com/ManuelReschke/PixelFox/internal/pkg/imageprocessor"
)

// APIServer implements the ServerInterface
type APIServer struct{}

// NewAPIServer creates a new API server instance
func NewAPIServer() *APIServer {
	return &APIServer{}
}

// GetPing handles the ping endpoint
func (s *APIServer) GetPing(c *fiber.Ctx) error {
	response := Pong{
		Ping: "pong",
	}

	return c.Status(fiber.StatusOK).JSON(response)
}

// GetUserProfile returns account information for the authenticated user (API key).
// Security is enforced via API key middleware attached in the router.
func (s *APIServer) GetUserProfile(c *fiber.Ctx) error {
	return controllers.HandleGetUserAccount(c)
}

// PostDirectUpload is documented in the public spec to describe the storage upload endpoint,
// but the actual upload should be performed against the `upload_url` returned by
// POST /upload/sessions. We don't serve storage uploads on the public v1 base path.
func (s *APIServer) PostDirectUpload(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
		"error":   "not_implemented",
		"message": "Direct uploads must be sent to the 'upload_url' returned by /api/v1/upload/sessions",
	})
}

// GetImage returns metadata for an image resource by UUID (API key protected).
// Delegates to the existing controller for consistent response shape.
func (s *APIServer) GetImage(c *fiber.Ctx, uuid string) error {
	// Controller reads uuid from route params; wrapper already set it.
	return controllers.HandleGetImageResourceAPI(c)
}

// GetImageStatus returns processing status for an image (JSON)
func (s *APIServer) GetImageStatus(c *fiber.Ctx, uuid string) error {
	if uuid == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bad_request", "message": "uuid missing"})
	}
	complete := imageprocessor.IsImageProcessingComplete(uuid)

	// try to fetch view url when complete
	var viewURL string
	if complete {
		imgRepo := repository.GetGlobalFactory().GetImageRepository()
		if image, err := imgRepo.GetByUUID(uuid); err == nil && image != nil {
			viewURL = "/i/" + image.ShareLink
		}
	}
	return c.JSON(fiber.Map{"complete": complete, "view_url": viewURL})
}

// PostUserUploadSession issues a direct upload session via API key authentication.
// Security is enforced via API key middleware attached in the router.
func (s *APIServer) PostUserUploadSession(c *fiber.Ctx) error {
	return controllers.HandleCreateUploadSessionAPI(c)
}
