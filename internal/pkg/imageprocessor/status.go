package imageprocessor

import (
	"fmt"
	"github.com/gofiber/fiber/v2/log"
	"strings"
	"time"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/cache"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
)

// Cache key format for image processing status
const (
	ImageStatusKeyFormat          = "image:status:%s"           // Format: image:status:<uuid>
	ImageStatusTimestampKeyFormat = "image:status:timestamp:%s" // Format: image:status:timestamp:<uuid>
)

// Status constants for image processing
const (
	STATUS_PENDING    = "pending"    // Image is queued for processing
	STATUS_PROCESSING = "processing" // Image is currently being processed
	STATUS_COMPLETED  = "completed"  // Image processing is complete
	STATUS_FAILED     = "failed"     // Image processing failed
)

// TTL für verschiedene Status
const (
	PENDING_TTL    = 30 * time.Minute // Längere Zeit für unverarbeitete Bilder
	PROCESSING_TTL = 30 * time.Minute // Längere Zeit für Bilder in Verarbeitung
	COMPLETED_TTL  = 5 * time.Minute  // Kurze Zeit für erfolgreich verarbeitete Bilder
	FAILED_TTL     = 1 * time.Hour    // Längere Zeit für fehlgeschlagene Bilder (für Fehleranalyse)
)

// SetImageStatus sets the processing status of an image in the cache
func SetImageStatus(imageUUID string, status string) error {
	if imageUUID == "" || status == "" {
		log.Errorf("[ImageProcessor] Invalid arguments for SetImageStatus (UUID: %s, Status: %s)", imageUUID, status)
		return fmt.Errorf("invalid UUID or status for setting image status")
	}
	key := fmt.Sprintf(ImageStatusKeyFormat, imageUUID)
	log.Debugf("[ImageProcessor] Setting cache status for %s to %s", imageUUID, status)

	// Set timestamp as well
	if err := SetImageStatusTimestamp(imageUUID, time.Now(), status); err != nil {
		// Log the error but continue setting the main status
		log.Warnf("[ImageProcessor] Failed to set status timestamp for %s while setting status %s: %v", imageUUID, status, err)
	}

	// Determine TTL based on the status
	ttl := PENDING_TTL // Default
	switch status {
	case STATUS_PENDING:
		ttl = PENDING_TTL
	case STATUS_PROCESSING:
		ttl = PROCESSING_TTL
	case STATUS_COMPLETED:
		ttl = COMPLETED_TTL
	case STATUS_FAILED:
		ttl = FAILED_TTL
	default:
		log.Warnf("[ImageProcessor] Unknown status '%s' encountered when setting TTL for %s. Using default TTL.", status, imageUUID)
	}

	log.Debugf("[ImageProcessor] Setting cache key '%s' with status '%s' and TTL %v", key, status, ttl)
	err := cache.Set(key, status, ttl)
	if err != nil {
		log.Errorf("[ImageProcessor] Failed to set cache status for %s: %v", imageUUID, err)
	}
	return err
}

// SetImageStatusTimestamp sets the timestamp when the status was set, using TTL appropriate for the *current* status being set.
func SetImageStatusTimestamp(imageUUID string, timestamp time.Time, currentStatus string) error {
	cacheKey := fmt.Sprintf(ImageStatusTimestampKeyFormat, imageUUID)
	timestampStr := timestamp.Format(time.RFC3339)

	// Determine TTL based on the *current* status being set
	ttl := PENDING_TTL // Default
	switch currentStatus {
	case STATUS_PENDING:
		ttl = PENDING_TTL
	case STATUS_PROCESSING:
		ttl = PROCESSING_TTL
	case STATUS_COMPLETED:
		ttl = COMPLETED_TTL
	case STATUS_FAILED:
		ttl = FAILED_TTL
	default:
		log.Warnf("[ImageProcessor] Unknown status '%s' encountered when setting timestamp TTL for %s. Using default TTL.", currentStatus, imageUUID)
	}

	log.Debugf("[ImageProcessor] Setting cache timestamp key '%s' with value '%s' and TTL %v", cacheKey, timestampStr, ttl)
	err := cache.Set(cacheKey, timestampStr, ttl)
	if err != nil {
		log.Errorf("[ImageProcessor] Failed to set cache timestamp for %s: %v", imageUUID, err)
	}
	return err
}

// GetImageStatus retrieves the processing status of an image from the cache
func GetImageStatus(imageUUID string) (string, error) {
	if imageUUID == "" {
		return "", fmt.Errorf("image UUID is empty")
	}
	key := fmt.Sprintf(ImageStatusKeyFormat, imageUUID)
	status, err := cache.Get(key)
	// Don't log error here, as cache miss is a normal scenario (handled by caller)
	// Log only unexpected errors if cache.Get provides them
	if err != nil && err.Error() != "cache: key not found" { // Adjust error check based on your cache library
		log.Errorf("[ImageProcessor] Error retrieving cache status for %s: %v", imageUUID, err)
	}
	return status, err
}

// GetImageStatusTimestamp gets the timestamp when the status was set
func GetImageStatusTimestamp(imageUUID string) (time.Time, error) {
	if imageUUID == "" {
		return time.Time{}, fmt.Errorf("image UUID is empty")
	}
	cacheKey := fmt.Sprintf(ImageStatusTimestampKeyFormat, imageUUID)
	timestampStr, err := cache.Get(cacheKey)
	if err != nil {
		// Don't log error here, cache miss is normal
		if err.Error() != "cache: key not found" { // Adjust error check
			log.Errorf("[ImageProcessor] Error retrieving cache timestamp for %s: %v", imageUUID, err)
		}
		return time.Time{}, err
	}

	timestamp, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		log.Errorf("[ImageProcessor] Failed to parse timestamp '%s' from cache for %s: %v", timestampStr, imageUUID, err)
		return time.Time{}, err
	}
	return timestamp, nil
}

// DeleteImageStatus deletes all status entries for an image from the cache
func DeleteImageStatus(imageUUID string) error {
	if imageUUID == "" {
		return fmt.Errorf("image UUID is empty")
	}
	statusKey := fmt.Sprintf(ImageStatusKeyFormat, imageUUID)
	timestampKey := fmt.Sprintf(ImageStatusTimestampKeyFormat, imageUUID)

	log.Debugf("[ImageProcessor] Deleting cache keys: %s, %s", statusKey, timestampKey)

	err1 := cache.Delete(statusKey)
	err2 := cache.Delete(timestampKey)

	// Combine errors if necessary, but often logging is sufficient
	if err1 != nil {
		log.Warnf("[ImageProcessor] Failed to delete status cache key %s: %v", statusKey, err1)
	}
	if err2 != nil {
		log.Warnf("[ImageProcessor] Failed to delete timestamp cache key %s: %v", timestampKey, err2)
	}

	// Return the first error encountered, or nil if both succeed
	if err1 != nil {
		return err1
	}
	return err2 // Returns nil if err1 was nil and err2 is nil
}

// IsImageProcessingComplete checks if image processing is complete using cache and DB fallback.
func IsImageProcessingComplete(imageUUID string) bool {
	if imageUUID == "" {
		return false
	} // Cannot check status without UUID

	// 1. Check Cache Status
	status, err := GetImageStatus(imageUUID)
	if err == nil { // Cache hit
		if status == STATUS_COMPLETED {
			log.Debugf("[ImageProcessor] Cache status for %s is COMPLETED.", imageUUID)
			// Optional: Consider deleting the cache entry now or rely on TTL
			// go DeleteImageStatus(imageUUID) // Maybe too aggressive? TTL might be better.
			return true
		}
		if status == STATUS_FAILED {
			log.Debugf("[ImageProcessor] Cache status for %s is FAILED. Considered 'complete' for polling purposes.", imageUUID)
			// Treat FAILED as complete in terms of stopping polling, though processing failed.
			return true
		}
		// If status is PENDING or PROCESSING, check timestamp below
	} else {
		// Log only unexpected errors, not cache misses
		if err.Error() != "cache: key not found" { // Adjust error check based on your cache library
			log.Errorf("[ImageProcessor] Error checking cache status for %s: %v", imageUUID, err)
			// If cache check fails unexpectedly, maybe fallback to DB? Or return false?
			// Let's fallback to DB check for robustness.
		} else {
			log.Debugf("[ImageProcessor] Cache miss for status %s.", imageUUID)
		}
	}

	// 2. Check Database Flags (if cache miss or status is PENDING/PROCESSING)
	db := database.GetDB()
	if db == nil {
		log.Error("[ImageProcessor] DB connection nil in IsImageProcessingComplete for %s.", imageUUID)
		return false // Cannot check DB, assume not complete
	}
	image, dbErr := models.FindImageByUUID(db, imageUUID)
	if dbErr != nil {
		log.Warnf("[ImageProcessor] Could not find image %s in DB for status check: %v", imageUUID, dbErr)
		// If not found in DB and no cache status, it's likely not processed or doesn't exist.
		return false
	}

	// Determine if optimization was expected based on file type
	fileExt := strings.ToLower(strings.TrimPrefix(image.FileType, "."))
	isGif := fileExt == "gif"
	// isWebP := fileExt == "webp" // <-- Removed unused variable
	isAVIFInput := fileExt == "avif" // Check if the *input* was AVIF
	// Optimization is skipped if input is GIF or AVIF. WebP input might still generate thumbnails.
	optimizationSkipped := isGif || isAVIFInput

	dbIndicatesComplete := false
	if optimizationSkipped {
		// For GIF/AVIF input, completion means thumbnails exist (or it's old)
		dbIndicatesComplete = image.HasThumbnailSmall || image.HasThumbnailMedium
	} else {
		// For other types, completion means optimized versions OR thumbnails exist
		dbIndicatesComplete = image.HasWebp || image.HasAVIF || image.HasThumbnailSmall || image.HasThumbnailMedium
	}

	if dbIndicatesComplete {
		log.Debugf("[ImageProcessor] DB flags indicate completion for %s. Updating cache.", imageUUID)
		// Update cache to COMPLETED as DB confirms processing happened
		_ = SetImageStatus(imageUUID, STATUS_COMPLETED) // Ignore error here, main goal is return true
		return true
	}

	// 3. Check Age / Timeout (if cache miss or status is PENDING/PROCESSING and DB doesn't show completion)

	// Check for old images without cache status (fallback)
	if status == "" && time.Since(image.CreatedAt) > 5*time.Minute {
		log.Warnf("[ImageProcessor] Image %s has no cache status but is older than 5 mins. Assuming completed.", imageUUID)
		_ = SetImageStatus(imageUUID, STATUS_COMPLETED) // Set cache status
		return true
	}

	// Check for PENDING/PROCESSING timeout
	if status == STATUS_PENDING || status == STATUS_PROCESSING {
		timestamp, tsErr := GetImageStatusTimestamp(imageUUID)
		if tsErr == nil {
			// Use a longer timeout than 60s, maybe closer to PENDING_TTL/2 ?
			processingTimeout := 15 * time.Minute
			if time.Since(timestamp) > processingTimeout {
				log.Warnf("[ImageProcessor] Image %s status is %s for over %v. Assuming failed/stuck.", imageUUID, status, processingTimeout)
				_ = SetImageStatus(imageUUID, STATUS_FAILED) // Mark as failed in cache
				return true                                  // Treat as 'complete' for polling purposes
			} else {
				log.Debugf("[ImageProcessor] Image %s is %s, but within timeout (%v).", imageUUID, status, processingTimeout)
			}
		} else {
			// No timestamp found, cannot check timeout reliably. Assume not complete yet.
			log.Warnf("[ImageProcessor] Image %s is %s but timestamp missing. Cannot check timeout.", imageUUID, status)
		}
	}

	// 4. Default: Not Complete
	log.Debugf("[ImageProcessor] Image %s processing is not considered complete yet.", imageUUID)
	return false
}
