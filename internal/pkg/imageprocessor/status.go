package imageprocessor

import (
	"fmt"
	"strings"
	"time"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/cache"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
)

// Cache key format for image processing status
const (
	ImageStatusKeyFormat = "image:status:%s" // Format: image:status:<uuid>
	ImageStatusTimestampKeyFormat = "image:status:timestamp:%s" // Format: image:status:timestamp:<uuid>
)

// Status constants for image processing
const (
	STATUS_PENDING   = "pending"   // Image is queued for processing
	STATUS_PROCESSING = "processing" // Image is currently being processed
	STATUS_COMPLETED  = "completed"  // Image processing is complete
	STATUS_FAILED     = "failed"     // Image processing failed
)

// TTL für verschiedene Status
const (
	PENDING_TTL   = 30 * time.Minute  // Längere Zeit für unverarbeitete Bilder
	PROCESSING_TTL = 30 * time.Minute  // Längere Zeit für Bilder in Verarbeitung
	COMPLETED_TTL = 5 * time.Minute   // Kurze Zeit für erfolgreich verarbeitete Bilder
	FAILED_TTL    = 1 * time.Hour     // Längere Zeit für fehlgeschlagene Bilder (für Fehleranalyse)
)

// SetImageStatus sets the processing status of an image in the cache
func SetImageStatus(imageUUID string, status string) error {
	key := fmt.Sprintf(ImageStatusKeyFormat, imageUUID)
	// Setze auch den Zeitstempel
	SetImageStatusTimestamp(imageUUID, time.Now())
	
	// Bestimme TTL basierend auf dem Status
	ttl := PENDING_TTL // Standard
	switch status {
	case STATUS_PENDING:
		ttl = PENDING_TTL
	case STATUS_PROCESSING:
		ttl = PROCESSING_TTL
	case STATUS_COMPLETED:
		// Für erfolgreiche Verarbeitungen setzen wir ein kurzes TTL
		// Das genügt für eventuelles Polling nach Verarbeitungsabschluss
		ttl = COMPLETED_TTL
	case STATUS_FAILED:
		ttl = FAILED_TTL
	}
	
	return cache.Set(key, status, ttl)
}

// SetImageStatusTimestamp sets the timestamp when the status was set
func SetImageStatusTimestamp(imageUUID string, timestamp time.Time) error {
	cacheKey := fmt.Sprintf(ImageStatusTimestampKeyFormat, imageUUID)
	timestampStr := timestamp.Format(time.RFC3339)
	
	// Zeitstempel mit gleichem TTL wie Status speichern
	ttl := PENDING_TTL // Standard
	
	// Versuche, den aktuellen Status zu lesen, um das passende TTL zu verwenden
	status, err := GetImageStatus(imageUUID)
	if err == nil {
		switch status {
		case STATUS_PENDING:
			ttl = PENDING_TTL
		case STATUS_PROCESSING:
			ttl = PROCESSING_TTL
		case STATUS_COMPLETED:
			ttl = COMPLETED_TTL
		case STATUS_FAILED:
			ttl = FAILED_TTL
		}
	}
	
	return cache.Set(cacheKey, timestampStr, ttl)
}

// GetImageStatus retrieves the processing status of an image from the cache
func GetImageStatus(imageUUID string) (string, error) {
	key := fmt.Sprintf(ImageStatusKeyFormat, imageUUID)
	return cache.Get(key)
}

// GetImageStatusTimestamp gets the timestamp when the status was set
func GetImageStatusTimestamp(imageUUID string) (time.Time, error) {
	cacheKey := fmt.Sprintf(ImageStatusTimestampKeyFormat, imageUUID)
	timestampStr, err := cache.Get(cacheKey)
	if err != nil {
		return time.Time{}, err
	}

	// Parse the timestamp
	timestamp, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		return time.Time{}, err
	}

	return timestamp, nil
}

// DeleteImageStatus löscht alle Status-Einträge für ein Bild aus dem Cache
func DeleteImageStatus(imageUUID string) error {
	// Status-Key löschen
	statusKey := fmt.Sprintf(ImageStatusKeyFormat, imageUUID)
	err1 := cache.Delete(statusKey)
	
	// Timestamp-Key löschen
	timestampKey := fmt.Sprintf(ImageStatusTimestampKeyFormat, imageUUID)
	err2 := cache.Delete(timestampKey)
	
	// Wenn mindestens eine Löschung erfolgreich war, keinen Fehler zurückgeben
	if err1 == nil || err2 == nil {
		return nil
	}
	
	// Sonst den ersten Fehler zurückgeben
	if err1 != nil {
		return err1
	}
	return err2
}

// IsImageProcessingComplete checks if image processing is complete
func IsImageProcessingComplete(imageUUID string) bool {
	// First, we check the cache status
	status, err := GetImageStatus(imageUUID)
	if err == nil && status == STATUS_COMPLETED {
		// If the cache status is COMPLETED, processing is definitely complete
		// Lösche Status-Einträge, da die Verarbeitung abgeschlossen ist
		go DeleteImageStatus(imageUUID)
		return true
	}

	// If there is no cache status or it is not COMPLETED,
	// we check the database to see if the image already has optimized versions
	db := database.GetDB()
	image, err := models.FindImageByUUID(db, imageUUID)
	if err != nil {
		// If we can't find the image, we assume it hasn't been processed
		return false
	}

	// For old images: If there is no status in the cache and the image is older than 5 minutes,
	// we consider it processed (regardless of whether it has thumbnails or not)
	if status == "" && time.Since(image.CreatedAt) > 5*time.Minute {
		// Set the status to COMPLETED so that the original image is displayed
		SetImageStatus(imageUUID, STATUS_COMPLETED)
		return true
	}

	// Check the file type to determine if optimization was skipped
	fileExt := image.FileType
	isGif := strings.ToLower(fileExt) == ".gif"
	isWebP := strings.ToLower(fileExt) == ".webp"
	isAVIF := strings.ToLower(fileExt) == ".avif"
	skipOptimization := isGif || isWebP || isAVIF

	// For images where optimization is skipped (GIF, WebP, AVIF),
	// we only check if thumbnails were created
	if skipOptimization {
		// For these formats, only thumbnails are created, not optimized versions
		if image.HasThumbnailSmall || image.HasThumbnailMedium {
			// Since we know the image has been processed, we update the cache
			SetImageStatus(imageUUID, STATUS_COMPLETED)
			return true
		}
		
		// Special case for old WebP/GIF/AVIF images without thumbnails:
		// If the image is older than 5 minutes, we consider it processed
		if time.Since(image.CreatedAt) > 5*time.Minute {
			// Set the status to COMPLETED so that the original image is displayed
			SetImageStatus(imageUUID, STATUS_COMPLETED)
			return true
		}
	} else {
		// For normal images, we check if optimized versions or thumbnails were created
		if image.HasWebp || image.HasAVIF || image.HasThumbnailSmall || image.HasThumbnailMedium {
			// Since we know the image has been processed, we update the cache
			SetImageStatus(imageUUID, STATUS_COMPLETED)
			return true
		}
	}

	// Check if processing is taking too long or has failed
	// If the status is PENDING or PROCESSING, we check how long it has been in this status
	if status == STATUS_PENDING || status == STATUS_PROCESSING {
		// Get the timestamp when the status was set
		timestamp, err := GetImageStatusTimestamp(imageUUID)
		if err == nil {
			// If the status was set more than 60 seconds ago, we assume
			// that processing has failed or is taking too long
			if time.Since(timestamp) > 60*time.Second {
				// Set the status to COMPLETED so that the original image is displayed
				SetImageStatus(imageUUID, STATUS_COMPLETED)
				return true
			}
		}
	}

	// If neither the cache nor the database indicate that the image has been processed,
	// we assume it is still being processed
	return false
}
