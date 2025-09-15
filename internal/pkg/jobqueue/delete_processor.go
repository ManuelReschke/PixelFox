package jobqueue

import (
	"context"
	"fmt"
	"github.com/gofiber/fiber/v2/log"
	"time"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/imageprocessor"
)

// EnqueueDeleteImageJob enqueues an asynchronous delete job for an image
func (q *Queue) EnqueueDeleteImageJob(imageID uint, imageUUID string, fromReportID *uint, initiatedBy *uint) (*Job, error) {
	payload := DeleteImageJobPayload{
		ImageID:       imageID,
		ImageUUID:     imageUUID,
		FromReportID:  fromReportID,
		InitiatedByID: initiatedBy,
	}
	return q.EnqueueJob(JobTypeDeleteImage, payload.ToMap())
}

// processDeleteImageJob processes the asynchronous delete job
func (q *Queue) processDeleteImageJob(ctx context.Context, job *Job) error {
	payload, perr := DeleteImageJobPayloadFromMap(job.Payload)
	if perr != nil {
		return fmt.Errorf("failed to parse delete image payload: %w", perr)
	}

	db := database.GetDB()
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}

	// Try to load image by ID; fallback to UUID
	var image models.Image
	if payload.ImageID > 0 {
		if err := db.First(&image, payload.ImageID).Error; err != nil {
			log.Warnf("[DeleteImageJob] Image %d not found by ID, trying UUID %s", payload.ImageID, payload.ImageUUID)
		}
	}
	if image.ID == 0 && payload.ImageUUID != "" {
		if err := db.Where("uuid = ?", payload.ImageUUID).First(&image).Error; err != nil {
			// If image already deleted from DB, nothing more to do
			log.Infof("[DeleteImageJob] Image %s not found in DB (already deleted)", payload.ImageUUID)
			return nil
		}
	}
	if image.ID == 0 {
		return nil // nothing to do
	}

	// Enqueue S3 backup deletions (if any) before file removal
	// We reuse the S3 delete job enqueuing logic from queue
	backups, berr := models.FindCompletedBackupsByImageID(db, image.ID)
	if berr == nil {
		for _, b := range backups {
			if _, err := q.EnqueueS3DeleteJob(image.ID, image.UUID, b.ObjectKey, b.BucketName, b.ID); err != nil {
				log.Errorf("[DeleteImageJob] Failed to enqueue S3 delete for backup %d: %v", b.ID, err)
			}
		}
	}

	// Delete files + soft-delete DB records (variants + image). This is idempotent enough.
	if err := imageprocessor.DeleteImageAndVariants(&image); err != nil {
		return fmt.Errorf("failed to delete image and variants: %w", err)
	}

	// If there are no backups, we can safely hard-delete DB records now to avoid DB bloat
	if berr == nil && len(backups) == 0 {
		// Hard delete variants + metadata + image
		_ = db.Unscoped().Where("image_id = ?", image.ID).Delete(&models.ImageVariant{}).Error
		_ = db.Unscoped().Where("image_id = ?", image.ID).Delete(&models.ImageMetadata{}).Error
		_ = db.Unscoped().Delete(&image).Error
		log.Infof("[DeleteImageJob] Hard-deleted DB records for image %s (no backups)", image.UUID)
	} else {
		log.Infof("[DeleteImageJob] Left DB records soft-deleted for image %s (backups pending)", image.UUID)
	}

	// If this deletion was triggered from a report, mark it resolved (idempotent)
	if payload.FromReportID != nil {
		now := time.Now()
		_ = db.Model(&models.ImageReport{}).
			Where("id = ?", *payload.FromReportID).
			Updates(map[string]interface{}{
				"status":         models.ReportStatusResolved,
				"resolved_by_id": payload.InitiatedByID,
				"resolved_at":    now,
			}).Error
	}

	log.Infof("[DeleteImageJob] Completed delete for image %s (ID: %d)", image.UUID, image.ID)
	return nil
}
