package apiv1

import (
	"github.com/gofiber/fiber/v2"
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
