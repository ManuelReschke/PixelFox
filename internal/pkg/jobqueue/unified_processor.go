package jobqueue

import (
	"fmt"
	"os"

	"github.com/gofiber/fiber/v2/log"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/imageprocessor"
)

// EnqueueImageProcessing enqueues an image processing job in the unified queue
// This replaces the old imageprocessor.ProcessImage function
func EnqueueImageProcessing(image *models.Image, enableBackup bool) error {
	if image == nil || image.UUID == "" {
		return fmt.Errorf("cannot enqueue invalid image data")
	}

	log.Infof("[UnifiedQueue] Enqueueing image processing job for %s (backup: %t)", image.UUID, enableBackup)

	// Set initial status to PENDING using the cache
	if err := imageprocessor.SetImageStatus(image.UUID, imageprocessor.STATUS_PENDING); err != nil {
		log.Errorf("[UnifiedQueue] Failed to set initial PENDING status for %s: %v", image.UUID, err)
		return fmt.Errorf("failed to set initial pending status for %s: %w", image.UUID, err)
	}

	// Create image processing payload
	payload := ImageProcessingJobPayload{
		ImageID:      image.ID,
		ImageUUID:    image.UUID,
		FilePath:     image.FilePath,
		FileName:     image.FileName,
		FileType:     image.FileType,
		EnableBackup: enableBackup,
	}

	// Get the global queue manager
	manager := GetManager()
	queue := manager.GetQueue()

	// Enqueue the job
	job, err := queue.EnqueueJob(JobTypeImageProcessing, payload.ToMap())
	if err != nil {
		// Set failed status in cache on enqueue failure
		if statusErr := imageprocessor.SetImageStatus(image.UUID, imageprocessor.STATUS_FAILED); statusErr != nil {
			log.Errorf("[UnifiedQueue] Additionally failed to set FAILED status for %s: %v", image.UUID, statusErr)
		}
		return fmt.Errorf("failed to enqueue image processing job for %s: %w", image.UUID, err)
	}

	log.Infof("[UnifiedQueue] Successfully enqueued image processing job %s for image %s", job.ID, image.UUID)
	return nil
}

// ProcessImageUnified is the new unified function that replaces imageprocessor.ProcessImage
// This function should be used instead of the old imageprocessor.ProcessImage
func ProcessImageUnified(image *models.Image) error {
	// Check if S3 backup is enabled via environment variable
	enableBackup := os.Getenv("S3_BACKUP_ENABLED") == "true"
	return EnqueueImageProcessing(image, enableBackup)
}
