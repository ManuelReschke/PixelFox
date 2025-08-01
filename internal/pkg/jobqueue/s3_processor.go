package jobqueue

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/gofiber/fiber/v2/log"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
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

	// Load S3 configuration
	config, err := s3backup.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load S3 config: %w", err)
	}

	if !config.IsEnabled() {
		return fmt.Errorf("S3 backup is disabled")
	}

	// Create S3 client
	s3Client, err := s3backup.NewClient(config)
	if err != nil {
		return fmt.Errorf("failed to create S3 client: %w", err)
	}

	// Get database connection
	db := database.GetDB()
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}

	// Get the backup record
	var backup models.ImageBackup
	if err := db.First(&backup, payload.BackupID).Error; err != nil {
		return fmt.Errorf("failed to find backup record: %w", err)
	}

	// Mark backup as uploading
	if err := backup.MarkAsUploading(db); err != nil {
		return fmt.Errorf("failed to mark backup as uploading: %w", err)
	}

	// Construct the full file path
	fullPath := filepath.Join(payload.FilePath, payload.FileName)

	// Generate S3 object key
	fileExt := filepath.Ext(payload.FileName)
	now := time.Now()
	objectKey := config.GetObjectKey(payload.ImageUUID, fileExt, now.Year(), int(now.Month()))

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
			continue
		}

		log.Infof("[S3Backup] Enqueued retry job %s for backup %d", job.ID, backup.ID)
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

	// Load S3 configuration
	config, err := s3backup.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load S3 config: %w", err)
	}

	if !config.IsEnabled() {
		return fmt.Errorf("S3 backup is disabled")
	}

	// Create S3 client
	s3Client, err := s3backup.NewClient(config)
	if err != nil {
		return fmt.Errorf("failed to create S3 client: %w", err)
	}

	// Get database connection
	db := database.GetDB()
	if db == nil {
		return fmt.Errorf("database connection is nil")
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
