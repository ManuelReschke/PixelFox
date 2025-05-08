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

// Function types for cache operations (for dependency injection in tests)
type (
	SetCacheFunc    func(key string, value interface{}, expiration time.Duration) error
	GetCacheFunc    func(key string) (string, error)
	GetIntCacheFunc func(key string) (int, error)
	DeleteCacheFunc func(key string) error
)

// Default implementations that use the actual cache package
var (
	SetCacheImplementation    SetCacheFunc    = cache.Set
	GetCacheImplementation    GetCacheFunc    = cache.Get
	GetIntCacheImplementation GetIntCacheFunc = cache.GetInt
	DeleteCacheImplementation DeleteCacheFunc = cache.Delete
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
	err := SetCacheImplementation(key, status, ttl)
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
	err := SetCacheImplementation(cacheKey, timestampStr, ttl)
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
	status, err := GetCacheImplementation(key)
	if err != nil {
		log.Debugf("[ImageProcessor] Cache miss for status %s: %v", imageUUID, err)
	}
	return status, err
}

// GetImageStatusTimestamp gets the timestamp when the status was set
func GetImageStatusTimestamp(imageUUID string) (time.Time, error) {
	if imageUUID == "" {
		return time.Time{}, fmt.Errorf("image UUID is empty")
	}
	cacheKey := fmt.Sprintf(ImageStatusTimestampKeyFormat, imageUUID)
	timestampStr, err := GetCacheImplementation(cacheKey)
	if err != nil {
		return time.Time{}, err
	}

	// Parse the timestamp string
	timestamp, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		log.Errorf("[ImageProcessor] Failed to parse timestamp '%s' for %s: %v", timestampStr, imageUUID, err)
		return time.Time{}, fmt.Errorf("failed to parse timestamp: %w", err)
	}

	return timestamp, nil
}

// DeleteImageStatus deletes all status entries for an image from the cache
func DeleteImageStatus(imageUUID string) error {
	if imageUUID == "" {
		return fmt.Errorf("image UUID is empty")
	}

	// Delete status key
	statusKey := fmt.Sprintf(ImageStatusKeyFormat, imageUUID)
	err := DeleteCacheImplementation(statusKey)
	if err != nil {
		log.Warnf("[ImageProcessor] Failed to delete status key for %s: %v", imageUUID, err)
		// Continue and try to delete timestamp key anyway
	}

	// Delete timestamp key
	timestampKey := fmt.Sprintf(ImageStatusTimestampKeyFormat, imageUUID)
	errTimestamp := DeleteCacheImplementation(timestampKey)
	if errTimestamp != nil {
		log.Warnf("[ImageProcessor] Failed to delete timestamp key for %s: %v", imageUUID, errTimestamp)
		// Return this error only if the first one was nil
		if err == nil {
			err = errTimestamp
		}
	}

	return err
}

// IsImageProcessingComplete checks if image processing is complete using cache and DB fallback.
func IsImageProcessingComplete(imageUUID string) bool {
	if imageUUID == "" {
		log.Warn("[ImageProcessor] Called IsImageProcessingComplete with empty UUID")
		return false
	}

	// 1. Check Cache Status
	status, err := GetImageStatus(imageUUID)
	if err == nil && status != "" {
		// Status found in cache
		switch status {
		case STATUS_COMPLETED:
			// Status is COMPLETED in cache, done.
			log.Debugf("[ImageProcessor] Cache indicates status COMPLETED for %s", imageUUID)
			return true
		case STATUS_FAILED:
			// Status is FAILED in cache, also considered 'complete' for polling
			log.Debugf("[ImageProcessor] Cache indicates status FAILED for %s", imageUUID)
			return true
		default:
			// Status is PENDING or PROCESSING, need additional checks
			log.Debugf("[ImageProcessor] Cache indicates status %s for %s", status, imageUUID)
		}
	} else {
		if err != nil {
			log.Warnf("[ImageProcessor] Error getting processing status from cache for %s: %v", imageUUID, err)
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
