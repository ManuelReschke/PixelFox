package jobqueue

import (
	"fmt"

	"github.com/gofiber/fiber/v2/log"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
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
		PoolID:       image.StoragePoolID,
	}

	// Optional: lookup node id for routing hint
	if image.StoragePool != nil && image.StoragePool.NodeID != "" {
		payload.NodeID = image.StoragePool.NodeID
	} else {
		// try to fetch pool
		db := database.GetDB()
		if db != nil && image.StoragePoolID > 0 {
			if pool, err := models.FindStoragePoolByID(db, image.StoragePoolID); err == nil && pool != nil {
				payload.NodeID = pool.NodeID
			}
		}
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
	// Check if S3 backup is enabled via storage pools
	db := database.GetDB()
	if db == nil {
		log.Errorf("[UnifiedQueue] Database connection is nil, disabling backup for image %s", image.UUID)
		return EnqueueImageProcessing(image, false)
	}

	// Check if there are any active S3 storage pools
	s3Pool, err := models.FindHighestPriorityS3Pool(db)
	if err != nil {
		log.Errorf("[UnifiedQueue] Failed to check S3 storage pools for image %s: %v", image.UUID, err)
		return EnqueueImageProcessing(image, false)
	}

	enableBackup := s3Pool != nil
	log.Infof("[UnifiedQueue] Processing image %s with S3 backup enabled: %t", image.UUID, enableBackup)
	return EnqueueImageProcessing(image, enableBackup)
}
