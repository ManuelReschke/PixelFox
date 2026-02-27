package controllers

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/redis/go-redis/v9"
	"github.com/sujit-baniya/flash"

	"github.com/ManuelReschke/PixelFox/app/repository"
	"github.com/ManuelReschke/PixelFox/internal/pkg/imageprocessor"
	"github.com/ManuelReschke/PixelFox/internal/pkg/jobqueue"
	"github.com/ManuelReschke/PixelFox/internal/pkg/usercontext"
	"github.com/ManuelReschke/PixelFox/views"
	"github.com/ManuelReschke/PixelFox/views/admin_views"
)

// ============================================================================
// ADMIN QUEUE CONTROLLER - Repository Pattern
// ============================================================================

// AdminQueueController handles admin queue-related HTTP requests using repository pattern
type AdminQueueController struct {
	queueRepo repository.QueueRepository
}

// NewAdminQueueController creates a new admin queue controller with repository
func NewAdminQueueController(queueRepo repository.QueueRepository) *AdminQueueController {
	return &AdminQueueController{
		queueRepo: queueRepo,
	}
}

// handleError is a helper method for consistent error handling
func (aqc *AdminQueueController) handleError(c *fiber.Ctx, message string, err error) error {
	fm := fiber.Map{
		"type":    "error",
		"message": message + ": " + err.Error(),
	}
	return flash.WithError(c, fm).Redirect("/admin/queues")
}

// HandleAdminQueues displays the admin queue monitor page using repository pattern
func (aqc *AdminQueueController) HandleAdminQueues(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	// Get queue items using repository
	queueItems, err := aqc.getQueueItems()
	if err != nil {
		queueItems = []admin_views.QueueItem{} // Empty slice if error
	}

	// Render the admin queue dashboard template
	component := admin_views.QueueItems(queueItems, time.Now())

	// Wrap in the main home layout with proper title
	home := views.HomeCtx(c, " | Cache & Queue Monitor", userCtx.IsLoggedIn, false, flash.Get(c), component, userCtx.IsAdmin, nil)

	// Convert the templ component to an HTTP handler and serve it
	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleAdminQueuesData returns only the data portion for HTMX updates using repository pattern
func (aqc *AdminQueueController) HandleAdminQueuesData(c *fiber.Ctx) error {
	// Get all queue items using repository
	queueItems, err := aqc.getQueueItems()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Fehler beim Abrufen der Queue-Daten: %v", err),
		})
	}

	// Render only the queue items component for HTMX refresh
	component := admin_views.QueueItemsTable(queueItems, time.Now())
	return component.Render(c.Context(), c.Response().BodyWriter())
}

// HandleAdminQueueDelete deletes a specific cache entry using repository pattern
func (aqc *AdminQueueController) HandleAdminQueueDelete(c *fiber.Ctx) error {
	key := c.Params("key")
	if key == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Schlüssel ist erforderlich")
	}

	// Delete the key using repository
	result, err := aqc.queueRepo.DeleteKey(key)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Fehler beim Löschen: %v", err))
	}

	if result == 0 {
		return c.Status(fiber.StatusNotFound).SendString("Eintrag nicht gefunden")
	}

	// Return empty content to remove the table row
	return c.SendString("")
}

// getQueueItems retrieves all items from the cache with their metadata using repository pattern
func (aqc *AdminQueueController) getQueueItems() ([]admin_views.QueueItem, error) {
	// Get all keys using repository
	keys, err := aqc.queueRepo.GetAllKeys()
	if err != nil {
		return nil, fmt.Errorf("Fehler beim Abrufen der Cache-Schlüssel: %v", err)
	}

	queueItems := make([]admin_views.QueueItem, 0, len(keys))

	for _, key := range keys {
		// Get value using repository
		value, err := aqc.queueRepo.GetValue(key)
		if err != nil && err != redis.Nil {
			// Skip this key if there's an error other than key not found
			continue
		}

		// Get TTL using repository
		ttl, err := aqc.queueRepo.GetTTL(key)
		if err != nil {
			// If we can't get TTL, use a default
			ttl = -1
		}

		// Determine type based on key prefix
		itemType := "unknown"
		displayValue := value

		if strings.HasPrefix(key, imageprocessor.ImageStatusKeyFormat[:13]) { // Prefix "image:status:"
			itemType = "image_status"
			// Extract UUID from key
			uuid := strings.TrimPrefix(key, "image:status:")
			// Display a more readable value for status keys
			switch value {
			case imageprocessor.STATUS_PENDING:
				displayValue = "Wartend"
			case imageprocessor.STATUS_PROCESSING:
				displayValue = "In Bearbeitung"
			case imageprocessor.STATUS_COMPLETED:
				displayValue = "Abgeschlossen"
			case imageprocessor.STATUS_FAILED:
				displayValue = "Fehlgeschlagen"
			}
			displayValue = fmt.Sprintf("%s (UUID: %s)", displayValue, uuid)
		} else if strings.HasPrefix(key, jobqueue.JobKeyPrefix) { // Job data
			itemType = "job"
			// Extract job ID and try to get status from job data
			jobID := strings.TrimPrefix(key, jobqueue.JobKeyPrefix)
			displayValue = fmt.Sprintf("Job %s: %s", jobID, aqc.getJobStatusFromValue(value))
		} else if key == jobqueue.JobQueueKey {
			itemType = "job_queue"
			queueSize, _ := aqc.queueRepo.GetListLength(key)
			displayValue = fmt.Sprintf("Warteschlange (%d Jobs)", queueSize)
		} else if key == jobqueue.JobProcessingKey {
			itemType = "job_processing"
			processingSize, _ := aqc.queueRepo.GetListLength(key)
			displayValue = fmt.Sprintf("In Bearbeitung (%d Jobs)", processingSize)
		} else if key == jobqueue.JobStatsKey {
			itemType = "job_stats"
			displayValue = "Job-Statistiken"
		} else if strings.HasPrefix(key, "analytics:") {
			itemType = "analytics"
		} else if strings.HasPrefix(key, "session:") {
			itemType = "session"
		}

		// Get memory usage (approximate for the value only)
		size := int64(len(value))

		// Use current time as creation time since Redis doesn't store this
		// In a real application, you might want to store creation time separately
		createdAt := time.Now()
		if ttl > 0 {
			// If TTL exists, we can estimate when the key was created by subtracting
			// from a known maximum TTL (assuming consistent TTL policy)
			// This is a rough approximation
			maxTTL := 24 * time.Hour // Assume 24-hour maximum TTL
			estimatedAge := maxTTL - ttl
			if estimatedAge > 0 && estimatedAge < maxTTL {
				createdAt = time.Now().Add(-estimatedAge)
			}
		}

		queueItems = append(queueItems, admin_views.QueueItem{
			Key:       key,
			Value:     displayValue,
			Type:      itemType,
			TTL:       ttl,
			Size:      size,
			CreatedAt: createdAt,
		})
	}

	// Sort by type and then by creation time (newest first)
	sort.Slice(queueItems, func(i, j int) bool {
		if queueItems[i].Type != queueItems[j].Type {
			return queueItems[i].Type < queueItems[j].Type
		}
		return queueItems[i].CreatedAt.After(queueItems[j].CreatedAt)
	})

	return queueItems, nil
}

// getJobStatusFromValue extracts job status from JSON job data
func (aqc *AdminQueueController) getJobStatusFromValue(jsonValue string) string {
	// Simple extraction without full JSON parsing for performance
	if strings.Contains(jsonValue, `"status":"pending"`) {
		return "Wartend"
	} else if strings.Contains(jsonValue, `"status":"processing"`) {
		return "In Bearbeitung"
	} else if strings.Contains(jsonValue, `"status":"completed"`) {
		return "Abgeschlossen"
	} else if strings.Contains(jsonValue, `"status":"failed"`) {
		return "Fehlgeschlagen"
	} else if strings.Contains(jsonValue, `"status":"retrying"`) {
		return "Wird wiederholt"
	}
	return "Unbekannt"
}

// ============================================================================
// GLOBAL ADMIN QUEUE CONTROLLER INSTANCE - Singleton Pattern
// ============================================================================

var adminQueueController *AdminQueueController

// InitializeAdminQueueController initializes the global admin queue controller
func InitializeAdminQueueController() {
	queueRepo := repository.GetGlobalFactory().GetQueueRepository()
	adminQueueController = NewAdminQueueController(queueRepo)
}

// GetAdminQueueController returns the global admin queue controller instance
func GetAdminQueueController() *AdminQueueController {
	if adminQueueController == nil {
		InitializeAdminQueueController()
	}
	return adminQueueController
}
