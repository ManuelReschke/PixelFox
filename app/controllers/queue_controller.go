package controllers

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/redis/go-redis/v9"
	"github.com/sujit-baniya/flash"

	"github.com/ManuelReschke/PixelFox/internal/pkg/cache"
	"github.com/ManuelReschke/PixelFox/internal/pkg/imageprocessor"
	"github.com/ManuelReschke/PixelFox/views"
	"github.com/ManuelReschke/PixelFox/views/admin_views"
)

// HandleAdminQueues displays the admin queue monitor page
func HandleAdminQueues(c *fiber.Ctx) error {
	// Set admin-specific data and view model here
	// We will fetch the queue items initially so the page isn't empty
	queueItems, err := getQueueItems()
	if err != nil {
		queueItems = []admin_views.QueueItem{} // Empty slice if error
	}

	// Render the admin queue dashboard template
	component := admin_views.QueueItems(queueItems, time.Now())
	
	// Wrap in the main home layout with proper title
	home := views.Home(" | Cache & Queue Monitor", isLoggedIn(c), false, flash.Get(c), component, true, nil)
	
	// Convert the templ component to an HTTP handler and serve it
	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleAdminQueuesData returns only the data portion for HTMX updates
func HandleAdminQueuesData(c *fiber.Ctx) error {
	// Get all queue items
	queueItems, err := getQueueItems()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Fehler beim Abrufen der Queue-Daten: %v", err),
		})
	}

	// Render only the queue items component for HTMX refresh
	component := admin_views.QueueItemsTable(queueItems, time.Now())
	return component.Render(c.Context(), c.Response().BodyWriter())
}

// getQueueItems retrieves all items from the cache with their metadata
func getQueueItems() ([]admin_views.QueueItem, error) {
	redisClient := cache.GetClient()
	ctx := context.Background()
	
	// Get all keys (use SCAN for production environments with large key sets)
	keys, err := redisClient.Keys(ctx, "*").Result()
	if err != nil {
		return nil, fmt.Errorf("Fehler beim Abrufen der Cache-Schlüssel: %v", err)
	}
	
	queueItems := make([]admin_views.QueueItem, 0, len(keys))
	
	for _, key := range keys {
		// Get value
		value, err := redisClient.Get(ctx, key).Result()
		if err != nil && err != redis.Nil {
			// Skip this key if there's an error other than key not found
			continue
		}
		
		// Get TTL
		ttl, err := redisClient.TTL(ctx, key).Result()
		if err != nil {
			// If we can't get TTL, use a default
			ttl = -1
		}
		
		// Determine type based on key prefix
		itemType := "unknown"
		if strings.HasPrefix(key, imageprocessor.ImageStatusKeyFormat[:13]) { // Prefix "image:status:"
			itemType = "status"
			// Extract UUID from key
			uuid := strings.TrimPrefix(key, "image:status:")
			// Display a more readable value for status keys
			switch value {
			case imageprocessor.STATUS_PENDING:
				value = "Wartend"
			case imageprocessor.STATUS_PROCESSING:
				value = "In Bearbeitung"
			case imageprocessor.STATUS_COMPLETED:
				value = "Abgeschlossen"
			case imageprocessor.STATUS_FAILED:
				value = "Fehlgeschlagen"
			}
			value = fmt.Sprintf("%s (UUID: %s)", value, uuid)
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
			Value:     value,
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
