package jobqueue

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ManuelReschke/PixelFox/app/models"
)

func TestJobType(t *testing.T) {
	tests := []struct {
		name     string
		jobType  JobType
		expected string
	}{
		{"Image Processing", JobTypeImageProcessing, "image_processing"},
		{"S3 Backup", JobTypeS3Backup, "s3_backup"},
		{"S3 Delete", JobTypeS3Delete, "s3_delete"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.jobType))
		})
	}
}

func TestJobStatus(t *testing.T) {
	tests := []struct {
		name     string
		status   JobStatus
		expected string
	}{
		{"Pending", JobStatusPending, "pending"},
		{"Processing", JobStatusProcessing, "processing"},
		{"Completed", JobStatusCompleted, "completed"},
		{"Failed", JobStatusFailed, "failed"},
		{"Retrying", JobStatusRetrying, "retrying"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.status))
		})
	}
}

func TestJob_IsRetryable(t *testing.T) {
	tests := []struct {
		name      string
		job       *Job
		retryable bool
	}{
		{
			name: "Failed job with retries remaining",
			job: &Job{
				Status:     JobStatusFailed,
				RetryCount: 1,
				MaxRetries: 3,
			},
			retryable: true,
		},
		{
			name: "Failed job with no retries remaining",
			job: &Job{
				Status:     JobStatusFailed,
				RetryCount: 3,
				MaxRetries: 3,
			},
			retryable: false,
		},
		{
			name: "Completed job",
			job: &Job{
				Status:     JobStatusCompleted,
				RetryCount: 1,
				MaxRetries: 3,
			},
			retryable: false,
		},
		{
			name: "Pending job",
			job: &Job{
				Status:     JobStatusPending,
				RetryCount: 0,
				MaxRetries: 3,
			},
			retryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.retryable, tt.job.IsRetryable())
		})
	}
}

func TestJob_MarkAsProcessing(t *testing.T) {
	job := &Job{
		Status: JobStatusPending,
	}

	beforeTime := time.Now()
	job.MarkAsProcessing()
	afterTime := time.Now()

	assert.Equal(t, JobStatusProcessing, job.Status)
	assert.True(t, job.UpdatedAt.After(beforeTime) || job.UpdatedAt.Equal(beforeTime))
	assert.True(t, job.UpdatedAt.Before(afterTime) || job.UpdatedAt.Equal(afterTime))
	assert.NotNil(t, job.ProcessedAt)
	assert.True(t, job.ProcessedAt.After(beforeTime) || job.ProcessedAt.Equal(beforeTime))
	assert.True(t, job.ProcessedAt.Before(afterTime) || job.ProcessedAt.Equal(afterTime))
}

func TestJob_MarkAsCompleted(t *testing.T) {
	job := &Job{
		Status:   JobStatusProcessing,
		ErrorMsg: "some error",
	}

	beforeTime := time.Now()
	job.MarkAsCompleted()
	afterTime := time.Now()

	assert.Equal(t, JobStatusCompleted, job.Status)
	assert.True(t, job.UpdatedAt.After(beforeTime) || job.UpdatedAt.Equal(beforeTime))
	assert.True(t, job.UpdatedAt.Before(afterTime) || job.UpdatedAt.Equal(afterTime))
	assert.NotNil(t, job.CompletedAt)
	assert.True(t, job.CompletedAt.After(beforeTime) || job.CompletedAt.Equal(beforeTime))
	assert.True(t, job.CompletedAt.Before(afterTime) || job.CompletedAt.Equal(afterTime))
	assert.Empty(t, job.ErrorMsg)
}

func TestJob_MarkAsFailed(t *testing.T) {
	job := &Job{
		Status:     JobStatusProcessing,
		RetryCount: 1,
	}

	errorMsg := "processing failed"
	beforeTime := time.Now()
	job.MarkAsFailed(errorMsg)
	afterTime := time.Now()

	assert.Equal(t, JobStatusFailed, job.Status)
	assert.True(t, job.UpdatedAt.After(beforeTime) || job.UpdatedAt.Equal(beforeTime))
	assert.True(t, job.UpdatedAt.Before(afterTime) || job.UpdatedAt.Equal(afterTime))
	assert.Equal(t, errorMsg, job.ErrorMsg)
	assert.Equal(t, 2, job.RetryCount)
}

func TestJob_MarkAsRetrying(t *testing.T) {
	job := &Job{
		Status: JobStatusFailed,
	}

	beforeTime := time.Now()
	job.MarkAsRetrying()
	afterTime := time.Now()

	assert.Equal(t, JobStatusRetrying, job.Status)
	assert.True(t, job.UpdatedAt.After(beforeTime) || job.UpdatedAt.Equal(beforeTime))
	assert.True(t, job.UpdatedAt.Before(afterTime) || job.UpdatedAt.Equal(afterTime))
}

func TestImageProcessingJobPayload_ToMap(t *testing.T) {
	payload := ImageProcessingJobPayload{
		ImageID:      123,
		ImageUUID:    "test-uuid-123",
		FilePath:     "/path/to/file",
		FileName:     "test.jpg",
		FileType:     ".jpg",
		EnableBackup: true,
	}

	result := payload.ToMap()

	expected := map[string]interface{}{
		"image_id":      uint(123),
		"image_uuid":    "test-uuid-123",
		"file_path":     "/path/to/file",
		"file_name":     "test.jpg",
		"file_type":     ".jpg",
		"enable_backup": true,
	}

	assert.Equal(t, expected, result)
}

func TestImageProcessingJobPayloadFromMap(t *testing.T) {
	data := map[string]interface{}{
		"image_id":      float64(123), // JSON numbers are float64
		"image_uuid":    "test-uuid-123",
		"file_path":     "/path/to/file",
		"file_name":     "test.jpg",
		"file_type":     ".jpg",
		"enable_backup": true,
	}

	payload, err := ImageProcessingJobPayloadFromMap(data)
	require.NoError(t, err)

	expected := &ImageProcessingJobPayload{
		ImageID:      123,
		ImageUUID:    "test-uuid-123",
		FilePath:     "/path/to/file",
		FileName:     "test.jpg",
		FileType:     ".jpg",
		EnableBackup: true,
	}

	assert.Equal(t, expected, payload)
}

func TestImageProcessingJobPayloadFromMap_InvalidData(t *testing.T) {
	// Test with invalid JSON structure
	data := map[string]interface{}{
		"image_id": make(chan int), // channels can't be marshaled to JSON
	}

	payload, err := ImageProcessingJobPayloadFromMap(data)
	assert.Error(t, err)
	assert.Nil(t, payload)
}

func TestS3BackupJobPayload_ToMap(t *testing.T) {
	payload := S3BackupJobPayload{
		ImageID:   456,
		ImageUUID: "backup-uuid-456",
		FilePath:  "/backup/path",
		FileName:  "backup.png",
		FileSize:  1024,
		Provider:  models.BackupProviderS3,
		BackupID:  789,
	}

	result := payload.ToMap()

	expected := map[string]interface{}{
		"image_id":   uint(456),
		"image_uuid": "backup-uuid-456",
		"file_path":  "/backup/path",
		"file_name":  "backup.png",
		"file_size":  int64(1024),
		"provider":   string(models.BackupProviderS3),
		"backup_id":  uint(789),
	}

	assert.Equal(t, expected, result)
}

func TestS3BackupJobPayloadFromMap(t *testing.T) {
	data := map[string]interface{}{
		"image_id":   float64(456),
		"image_uuid": "backup-uuid-456",
		"file_path":  "/backup/path",
		"file_name":  "backup.png",
		"file_size":  float64(1024),
		"provider":   "s3",
		"backup_id":  float64(789),
	}

	payload, err := S3BackupJobPayloadFromMap(data)
	require.NoError(t, err)

	expected := &S3BackupJobPayload{
		ImageID:   456,
		ImageUUID: "backup-uuid-456",
		FilePath:  "/backup/path",
		FileName:  "backup.png",
		FileSize:  1024,
		Provider:  models.BackupProviderS3,
		BackupID:  789,
	}

	assert.Equal(t, expected, payload)
}

func TestS3DeleteJobPayload_ToMap(t *testing.T) {
	payload := S3DeleteJobPayload{
		ImageID:    789,
		ImageUUID:  "delete-uuid-789",
		ObjectKey:  "2024/01/test-file.jpg",
		BucketName: "test-bucket",
		Provider:   models.BackupProviderS3,
		BackupID:   101112,
	}

	result := payload.ToMap()

	expected := map[string]interface{}{
		"image_id":    uint(789),
		"image_uuid":  "delete-uuid-789",
		"object_key":  "2024/01/test-file.jpg",
		"bucket_name": "test-bucket",
		"provider":    string(models.BackupProviderS3),
		"backup_id":   uint(101112),
	}

	assert.Equal(t, expected, result)
}

func TestS3DeleteJobPayloadFromMap(t *testing.T) {
	data := map[string]interface{}{
		"image_id":    float64(789),
		"image_uuid":  "delete-uuid-789",
		"object_key":  "2024/01/test-file.jpg",
		"bucket_name": "test-bucket",
		"provider":    "s3",
		"backup_id":   float64(101112),
	}

	payload, err := S3DeleteJobPayloadFromMap(data)
	require.NoError(t, err)

	expected := &S3DeleteJobPayload{
		ImageID:    789,
		ImageUUID:  "delete-uuid-789",
		ObjectKey:  "2024/01/test-file.jpg",
		BucketName: "test-bucket",
		Provider:   models.BackupProviderS3,
		BackupID:   101112,
	}

	assert.Equal(t, expected, payload)
}

func TestPayloadRoundTrip(t *testing.T) {
	t.Run("ImageProcessingJobPayload", func(t *testing.T) {
		original := ImageProcessingJobPayload{
			ImageID:      123,
			ImageUUID:    "round-trip-test",
			FilePath:     "/test/path",
			FileName:     "roundtrip.jpg",
			FileType:     ".jpg",
			EnableBackup: false,
		}

		// Convert to map and back
		data := original.ToMap()
		result, err := ImageProcessingJobPayloadFromMap(data)
		require.NoError(t, err)

		assert.Equal(t, &original, result)
	})

	t.Run("S3BackupJobPayload", func(t *testing.T) {
		original := S3BackupJobPayload{
			ImageID:   456,
			ImageUUID: "backup-roundtrip",
			FilePath:  "/backup/test",
			FileName:  "backup.png",
			FileSize:  2048,
			Provider:  models.BackupProviderS3,
			BackupID:  999,
		}

		// Convert to map and back
		data := original.ToMap()
		result, err := S3BackupJobPayloadFromMap(data)
		require.NoError(t, err)

		assert.Equal(t, &original, result)
	})

	t.Run("S3DeleteJobPayload", func(t *testing.T) {
		original := S3DeleteJobPayload{
			ImageID:    789,
			ImageUUID:  "delete-roundtrip",
			ObjectKey:  "2024/test/delete.jpg",
			BucketName: "roundtrip-bucket",
			Provider:   models.BackupProviderS3,
			BackupID:   111213,
		}

		// Convert to map and back
		data := original.ToMap()
		result, err := S3DeleteJobPayloadFromMap(data)
		require.NoError(t, err)

		assert.Equal(t, &original, result)
	})
}

func TestJobJSONSerialization(t *testing.T) {
	now := time.Now()
	processedAt := now.Add(time.Minute)
	completedAt := now.Add(2 * time.Minute)

	job := &Job{
		ID:          "test-job-123",
		Type:        JobTypeImageProcessing,
		Status:      JobStatusCompleted,
		Payload:     map[string]interface{}{"test": "data"},
		CreatedAt:   now,
		UpdatedAt:   now.Add(time.Second),
		ProcessedAt: &processedAt,
		CompletedAt: &completedAt,
		ErrorMsg:    "",
		RetryCount:  0,
		MaxRetries:  3,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(job)
	require.NoError(t, err)

	// Unmarshal back
	var result Job
	err = json.Unmarshal(jsonData, &result)
	require.NoError(t, err)

	// Compare (times may have slight precision differences)
	assert.Equal(t, job.ID, result.ID)
	assert.Equal(t, job.Type, result.Type)
	assert.Equal(t, job.Status, result.Status)
	assert.Equal(t, job.Payload, result.Payload)
	assert.Equal(t, job.ErrorMsg, result.ErrorMsg)
	assert.Equal(t, job.RetryCount, result.RetryCount)
	assert.Equal(t, job.MaxRetries, result.MaxRetries)

	// Time comparisons (allowing for minor precision differences)
	assert.True(t, job.CreatedAt.Sub(result.CreatedAt) < time.Millisecond)
	assert.True(t, job.UpdatedAt.Sub(result.UpdatedAt) < time.Millisecond)
	assert.NotNil(t, result.ProcessedAt)
	assert.True(t, job.ProcessedAt.Sub(*result.ProcessedAt) < time.Millisecond)
	assert.NotNil(t, result.CompletedAt)
	assert.True(t, job.CompletedAt.Sub(*result.CompletedAt) < time.Millisecond)
}
