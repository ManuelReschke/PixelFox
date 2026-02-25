package jobqueue

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2/log"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
	"github.com/ManuelReschke/PixelFox/internal/pkg/s3backup"
)

// processS3BackupJob processes an S3 backup job
func (q *Queue) processS3BackupJob(ctx context.Context, job *Job) error {
	// Parse the job payload
	payload, err := S3BackupJobPayloadFromMap(job.Payload)
	if err != nil {
		return fmt.Errorf("failed to parse S3 backup job payload: %w", err)
	}

	log.Infof("[S3Backup] Processing backup job for image %s (ID: %d)", payload.ImageUUID, payload.ImageID)

	// Get database connection first
	db := database.GetDB()
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}

	// Load S3 configuration from storage pools
	config, err := s3backup.LoadConfigFromStoragePool(db)
	if err != nil {
		return fmt.Errorf("failed to load S3 config from storage pools: %w", err)
	}

	if !config.IsEnabled() {
		log.Infof("[S3Backup] No active S3 storage pools configured for backup, skipping backup for image %s", payload.ImageUUID)

		// Get backup record to mark as failed
		var backup models.ImageBackup
		if err := db.First(&backup, payload.BackupID).Error; err == nil {
			backup.MarkAsFailed(db, "No active S3 storage pools configured")
		}
		return nil // Don't return error, just skip backup
	}

	// Create S3 client
	s3Client, err := s3backup.NewClient(config)
	if err != nil {
		return fmt.Errorf("failed to create S3 client: %w", err)
	}

	// Get database connection
	db = database.GetDB()
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}

	// Get the backup record
	var backup models.ImageBackup
	if err := db.First(&backup, payload.BackupID).Error; err != nil {
		return fmt.Errorf("failed to find backup record: %w", err)
	}

	// Get the image record to check storage pool configuration (needed for node routing)
	var image models.Image
	if err := db.Preload("StoragePool").Where("id = ?", payload.ImageID).First(&image).Error; err != nil {
		return fmt.Errorf("failed to find image %d: %w", payload.ImageID, err)
	}

	// Node routing: ensure backup runs on the node that has access to the file (image's storage node)
	nodeID := strings.TrimSpace(env.GetEnv("NODE_ID", ""))
	if nodeID != "" && image.StoragePool != nil {
		poolNode := strings.TrimSpace(image.StoragePool.NodeID)
		if poolNode != "" && !strings.EqualFold(nodeID, poolNode) {
			// Requeue for correct node without counting as a failure
			if err := q.requeueJob(ctx, job); err != nil {
				log.Errorf("[S3Backup] Failed to requeue job %s for node routing: %v", job.ID, err)
			} else {
				log.Infof("[S3Backup] Requeued job %s for node %s (current node %s)", job.ID, poolNode, nodeID)
			}
			return ErrRequeue
		}
	}

	// Claim backup for uploading atomically (DB dedupe). If claim fails, verify current state and skip if already claimed/completed.
	claimed, err := backup.ClaimForUploading(db, nodeID)
	if err != nil {
		return fmt.Errorf("failed to claim backup for uploading: %w", err)
	}
	if !claimed {
		// Reload and inspect state
		if err := db.First(&backup, payload.BackupID).Error; err == nil {
			if backup.Status == models.BackupStatusCompleted {
				log.Infof("[S3Backup] Backup %d already completed; skipping", backup.ID)
				return nil
			}
			if backup.Status == models.BackupStatusUploading {
				// Someone else is processing; skip duplicate job gracefully
				log.Infof("[S3Backup] Backup %d already claimed by %s; skipping", backup.ID, backup.ClaimedBy)
				return nil
			}
		}
		// Otherwise, continue and try once (rare race). Attempt to set uploading (best-effort)
		if ok, err := backup.ClaimForUploading(db, nodeID); err != nil || !ok {
			if err != nil {
				return fmt.Errorf("failed to claim backup (second attempt): %w", err)
			}
			log.Infof("[S3Backup] Backup %d not eligible to claim; skipping", backup.ID)
			return nil
		}
	}

	// Construct the full file path using storage pool-aware path construction
	var fullPath string
	if image.StoragePoolID > 0 && image.StoragePool != nil {
		// Use storage pool base path
		fullPath = filepath.Join(image.StoragePool.BasePath, image.FilePath, image.FileName)
	} else {
		// Fallback to legacy path construction
		fullPath = filepath.Join(payload.FilePath, payload.FileName)
	}

	// Generate S3 object key with day folder
	fileExt := filepath.Ext(payload.FileName)
	now := time.Now()
	objectKey := config.GetObjectKey(payload.ImageUUID, fileExt, now.Year(), int(now.Month()), now.Day())

	// Upload to S3
	log.Infof("[S3Backup] Uploading %s to S3 as %s", fullPath, objectKey)
	result, err := s3Client.UploadFile(fullPath, objectKey)
	if err != nil {
		// Mark backup as failed
		if markErr := backup.MarkAsFailed(db, err.Error()); markErr != nil {
			log.Errorf("[S3Backup] Failed to mark backup as failed: %v", markErr)
		}
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	// Mark backup as completed
	if err := backup.MarkAsCompleted(db, result.BucketName, result.ObjectKey, result.Size); err != nil {
		return fmt.Errorf("failed to mark backup as completed: %w", err)
	}

	log.Infof("[S3Backup] Successfully backed up image %s to s3://%s/%s",
		payload.ImageUUID, result.BucketName, result.ObjectKey)

	return nil
}

// EnqueueS3BackupJob creates and enqueues an S3 backup job
func (q *Queue) EnqueueS3BackupJob(imageID uint, imageUUID, filePath, fileName string, fileSize int64, backupID uint) (*Job, error) {
	payload := S3BackupJobPayload{
		ImageID:   imageID,
		ImageUUID: imageUUID,
		FilePath:  filePath,
		FileName:  fileName,
		FileSize:  fileSize,
		Provider:  models.BackupProviderS3,
		BackupID:  backupID,
	}

	return q.EnqueueJob(JobTypeS3Backup, payload.ToMap())
}

// RetryFailedS3Backups finds and retries failed S3 backup jobs
func (q *Queue) RetryFailedS3Backups() error {
	db := database.GetDB()

	// Find failed backups that can be retried
	failedBackups, err := models.FindFailedRetryableBackups(db)
	if err != nil {
		return fmt.Errorf("failed to find failed backups: %w", err)
	}

	log.Infof("[S3Backup] Found %d failed backups to retry", len(failedBackups))

	for _, backup := range failedBackups {

		// Create retry job
		job, err := q.EnqueueS3BackupJob(
			backup.ImageID,
			backup.Image.UUID,
			backup.Image.FilePath,
			backup.Image.FileName,
			backup.Image.FileSize,
			backup.ID,
		)
		if err != nil {
			log.Errorf("[S3Backup] Failed to enqueue retry job for backup %d: %v", backup.ID, err)
			// Record enqueue issue without burning upload retries.
			if updateErr := db.Model(&models.ImageBackup{}).
				Where("id = ?", backup.ID).
				Update("error_message", fmt.Sprintf("enqueue failed: %v", err)).Error; updateErr != nil {
				log.Errorf("[S3Backup] Failed to store enqueue error for backup %d: %v", backup.ID, updateErr)
			}
			continue
		}

		log.Infof("[S3Backup] Enqueued retry job %s for backup %d", job.ID, backup.ID)
	}

	return nil
}

// RecoverStuckS3Uploading marks backups stuck in 'uploading' as failed for retry
func (q *Queue) RecoverStuckS3Uploading(maxAge time.Duration) error {
	db := database.GetDB()
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}
	stuck, err := models.FindStuckUploadingBackups(db, maxAge)
	if err != nil {
		return fmt.Errorf("failed to find stuck uploading backups: %w", err)
	}
	if len(stuck) == 0 {
		return nil
	}
	log.Infof("[S3Backup] Recovering %d stuck 'uploading' backups older than %s", len(stuck), maxAge)
	for i := range stuck {
		b := &stuck[i]
		if err := b.MarkAsFailed(db, "recovered from stuck uploading"); err != nil {
			log.Errorf("[S3Backup] Failed to mark stuck backup %d as failed: %v", b.ID, err)
		}
	}
	return nil
}

// processS3DeleteJob processes an S3 delete job
func (q *Queue) processS3DeleteJob(ctx context.Context, job *Job) error {
	// Parse the job payload
	payload, err := S3DeleteJobPayloadFromMap(job.Payload)
	if err != nil {
		return fmt.Errorf("failed to parse S3 delete job payload: %w", err)
	}

	log.Infof("[S3Delete] Processing delete job for image %s (ID: %d)", payload.ImageUUID, payload.ImageID)

	// Get database connection
	db := database.GetDB()
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}

	// Load S3 configuration from storage pools
	config, err := s3backup.LoadConfigFromStoragePool(db)
	if err != nil {
		return fmt.Errorf("failed to load S3 config from storage pools: %w", err)
	}

	if !config.IsEnabled() {
		log.Infof("[S3Delete] No active S3 storage pools configured for delete, skipping delete for image %s", payload.ImageUUID)
		return nil // Don't return error, just skip delete
	}

	// Create S3 client
	s3Client, err := s3backup.NewClient(config)
	if err != nil {
		return fmt.Errorf("failed to create S3 client: %w", err)
	}

	// Get the backup record
	var backup models.ImageBackup
	if err := db.First(&backup, payload.BackupID).Error; err != nil {
		return fmt.Errorf("failed to find backup record: %w", err)
	}

	// Delete from S3
	log.Infof("[S3Delete] Deleting s3://%s/%s", payload.BucketName, payload.ObjectKey)
	err = s3Client.DeleteFile(payload.ObjectKey)
	if err != nil {
		// Mark backup as failed to delete
		if markErr := backup.MarkAsDeleted(db, fmt.Sprintf("Failed to delete from S3: %v", err)); markErr != nil {
			log.Errorf("[S3Delete] Failed to mark backup as failed: %v", markErr)
		}
		return fmt.Errorf("failed to delete from S3: %w", err)
	}

	// Mark backup as deleted in database
	if err := backup.MarkAsDeleted(db, "Successfully deleted from S3"); err != nil {
		return fmt.Errorf("failed to mark backup as deleted: %w", err)
	}

	log.Infof("[S3Delete] Successfully deleted image %s from s3://%s/%s",
		payload.ImageUUID, payload.BucketName, payload.ObjectKey)

	// If no completed backups remain for this image, hard-delete DB records to avoid bloat
	var remaining int64
	if err := db.Model(&models.ImageBackup{}).
		Where("image_id = ? AND status = ?", payload.ImageID, models.BackupStatusCompleted).
		Count(&remaining).Error; err == nil {
		if remaining == 0 {
			// Hard delete variants/metadata/image (idempotent if already removed)
			_ = db.Unscoped().Where("image_id = ?", payload.ImageID).Delete(&models.ImageVariant{}).Error
			_ = db.Unscoped().Where("image_id = ?", payload.ImageID).Delete(&models.ImageMetadata{}).Error
			_ = db.Unscoped().Delete(&models.Image{}, payload.ImageID).Error
			log.Infof("[S3Delete] Hard-deleted DB records for image %s (no backups remain)", payload.ImageUUID)
		}
	}

	return nil
}

// EnqueueS3DeleteJob creates and enqueues an S3 delete job
func (q *Queue) EnqueueS3DeleteJob(imageID uint, imageUUID, objectKey, bucketName string, backupID uint) (*Job, error) {
	payload := S3DeleteJobPayload{
		ImageID:    imageID,
		ImageUUID:  imageUUID,
		ObjectKey:  objectKey,
		BucketName: bucketName,
		Provider:   models.BackupProviderS3,
		BackupID:   backupID,
	}

	return q.EnqueueJob(JobTypeS3Delete, payload.ToMap())
}

// ProcessDelayedS3Backups finds images older than the configured delay and enqueues backup jobs
func (q *Queue) ProcessDelayedS3Backups() error {
	// Get current app settings to check backup delay
	settings := models.GetAppSettings()
	delayMinutes := settings.GetS3BackupDelayMinutes()

	// If delay is 0 or negative, no delayed processing needed
	if delayMinutes <= 0 {
		return nil
	}

	// Get database connection
	db := database.GetDB()
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}

	// Calculate cutoff time (current time minus delay)
	cutoffTime := time.Now().Add(-time.Duration(delayMinutes) * time.Minute)

	// Find pending backups for images older than the delay time
	var pendingBackups []models.ImageBackup
	err := db.Preload("Image").
		Joins("JOIN images ON image_backups.image_id = images.id").
		Where("image_backups.status = ? AND images.created_at <= ?", models.BackupStatusPending, cutoffTime).
		Find(&pendingBackups).Error

	if err != nil {
		return fmt.Errorf("failed to find pending delayed backups: %w", err)
	}

	if len(pendingBackups) == 0 {
		log.Info("[DelayedS3Backup] No delayed backups ready for processing")
		return nil
	}

	log.Infof("[DelayedS3Backup] Found %d delayed backups ready for processing", len(pendingBackups))

	// Process each pending backup
	for _, backup := range pendingBackups {
		image := backup.Image

		log.Infof("[DelayedS3Backup] Enqueueing delayed backup for image %s (ID: %d)", image.UUID, image.ID)

		// Create S3 backup job
		job, err := q.EnqueueS3BackupJob(
			image.ID,
			image.UUID,
			image.FilePath,
			image.FileName,
			image.FileSize,
			backup.ID,
		)
		if err != nil {
			log.Errorf("[DelayedS3Backup] Failed to enqueue delayed backup job for image %s: %v", image.UUID, err)
			// Keep pending status and only record enqueue issue.
			if updateErr := db.Model(&models.ImageBackup{}).
				Where("id = ?", backup.ID).
				Update("error_message", fmt.Sprintf("enqueue failed: %v", err)).Error; updateErr != nil {
				log.Errorf("[DelayedS3Backup] Failed to store enqueue error for backup %d: %v", backup.ID, updateErr)
			}
			continue
		}

		log.Infof("[DelayedS3Backup] Enqueued delayed backup job %s for image %s", job.ID, image.UUID)
	}

	return nil
}
