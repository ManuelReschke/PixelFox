//go:build test
// +build test

package jobqueue

import (
	"time"

	"github.com/ManuelReschke/PixelFox/app/models"
)

// TestJobFactory creates test jobs for different types
func TestJobFactory() map[JobType]*Job {
	now := time.Now()

	return map[JobType]*Job{
		JobTypeImageProcessing: {
			ID:     "test-image-job",
			Type:   JobTypeImageProcessing,
			Status: JobStatusPending,
			Payload: ImageProcessingJobPayload{
				ImageID:      123,
				ImageUUID:    "test-image-uuid",
				FilePath:     "/test/path",
				FileName:     "test.jpg",
				FileType:     ".jpg",
				EnableBackup: true,
			}.ToMap(),
			CreatedAt:  now,
			UpdatedAt:  now,
			RetryCount: 0,
			MaxRetries: 3,
		},
		JobTypeS3Backup: {
			ID:     "test-backup-job",
			Type:   JobTypeS3Backup,
			Status: JobStatusPending,
			Payload: S3BackupJobPayload{
				ImageID:   123,
				ImageUUID: "test-backup-uuid",
				FilePath:  "/backup/path",
				FileName:  "backup.jpg",
				FileSize:  2048,
				Provider:  models.BackupProviderS3,
				BackupID:  456,
			}.ToMap(),
			CreatedAt:  now,
			UpdatedAt:  now,
			RetryCount: 0,
			MaxRetries: 3,
		},
		JobTypeS3Delete: {
			ID:     "test-delete-job",
			Type:   JobTypeS3Delete,
			Status: JobStatusPending,
			Payload: S3DeleteJobPayload{
				ImageID:    123,
				ImageUUID:  "test-delete-uuid",
				ObjectKey:  "2024/01/test.jpg",
				BucketName: "test-bucket",
				Provider:   models.BackupProviderS3,
				BackupID:   789,
			}.ToMap(),
			CreatedAt:  now,
			UpdatedAt:  now,
			RetryCount: 0,
			MaxRetries: 3,
		},
	}
}

// WaitForCondition waits for a condition to be true with timeout
func WaitForCondition(condition func() bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}
