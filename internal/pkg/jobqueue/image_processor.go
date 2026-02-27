package jobqueue

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2/log"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
	"github.com/ManuelReschke/PixelFox/internal/pkg/imageprocessor"
)

var ErrRequeue = fmt.Errorf("requeue job for another node")

// processImageProcessingJob processes an image processing job
func (q *Queue) processImageProcessingJob(ctx context.Context, job *Job) error {
	log.Infof("[JobQueue] Processing image processing job %s", job.ID)

	// Parse the payload
	payload, err := ImageProcessingJobPayloadFromMap(job.Payload)
	if err != nil {
		return fmt.Errorf("failed to parse image processing payload: %w", err)
	}

	// Get database connection
	db := database.GetDB()
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}

	// Find the image in database with storage pool preloaded
	var image models.Image
	if err := db.Preload("StoragePool").Where("uuid = ?", payload.ImageUUID).First(&image).Error; err != nil {
		return fmt.Errorf("failed to find image %s: %w", payload.ImageUUID, err)
	}

	// Optional routing: Only process jobs for this node if NODE_ID is set
	nodeID := strings.TrimSpace(env.GetEnv("NODE_ID", ""))
	if nodeID != "" && image.StoragePool != nil {
		poolNode := strings.TrimSpace(image.StoragePool.NodeID)
		if poolNode != "" && !strings.EqualFold(nodeID, poolNode) {
			// Requeue for another node
			if err := q.requeueJob(ctx, job); err != nil {
				log.Errorf("[JobQueue] Failed to requeue job %s for node routing: %v", job.ID, err)
			} else {
				log.Infof("[JobQueue] Requeued job %s for node %s (current node %s)", job.ID, poolNode, nodeID)
			}
			return ErrRequeue
		}
	}

	// Verify the original file exists using storage pool-aware path construction
	var originalFilePath string
	if image.StoragePoolID > 0 && image.StoragePool != nil {
		// Use storage pool base path
		originalFilePath = filepath.Join(image.StoragePool.BasePath, image.FilePath, image.FileName)
	} else {
		// Fallback to legacy path
		originalFilePath = fmt.Sprintf("%s/%s", payload.FilePath, payload.FileName)
	}

	if _, err := os.Stat(originalFilePath); os.IsNotExist(err) {
		return fmt.Errorf("original file not found: %s", originalFilePath)
	}

	// Set image status to processing using cache
	if err := imageprocessor.SetImageStatus(payload.ImageUUID, imageprocessor.STATUS_PROCESSING); err != nil {
		log.Errorf("[JobQueue] Failed to set processing status for %s: %v", payload.ImageUUID, err)
	}

	// Process the image using the existing imageprocessor logic
	// We'll extract the core processing logic from imageprocessor.processImage
	err = q.processImageCore(&image)
	if err != nil {
		// Set failed status in cache
		if statusErr := imageprocessor.SetImageStatus(payload.ImageUUID, imageprocessor.STATUS_FAILED); statusErr != nil {
			log.Errorf("[JobQueue] Failed to set failed status for %s: %v", payload.ImageUUID, statusErr)
		}
		return fmt.Errorf("image processing failed for %s: %w", payload.ImageUUID, err)
	}

	// Set completed status in cache
	if err := imageprocessor.SetImageStatus(payload.ImageUUID, imageprocessor.STATUS_COMPLETED); err != nil {
		log.Errorf("[JobQueue] Failed to set completed status for %s: %v", payload.ImageUUID, err)
		return fmt.Errorf("failed to set completed status: %w", err)
	}

	log.Infof("[JobQueue] Image processing completed for %s", payload.ImageUUID)

	return nil
}

// processImageCore contains the core image processing logic extracted from imageprocessor
// This is a simplified version that focuses on the essential processing
func (q *Queue) processImageCore(imageModel *models.Image) error {
	log.Debugf("[JobQueue] Processing image core: %s", imageModel.UUID)

	// Use the existing imageprocessor functions but without the queue/worker logic
	// We'll call the core processing function directly

	// Import the processing logic from imageprocessor package
	// This is essentially the same logic as in imageprocessor.processImage
	// but without the queue management and status handling (we handle that here)

	return imageprocessor.ProcessImageSync(imageModel)
}
