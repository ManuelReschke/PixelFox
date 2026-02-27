package jobqueue

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobType(t *testing.T) {
	tests := []struct {
		name     string
		jobType  JobType
		expected string
	}{
		{"Image Processing", JobTypeImageProcessing, "image_processing"},
		{"Pool Move Enqueue", JobTypePoolMoveEnqueue, "pool_move_enqueue"},
		{"Move Image", JobTypeMoveImage, "move_image"},
		{"Delete Image", JobTypeDeleteImage, "delete_image"},
		{"Reconcile Variants", JobTypeReconcileVariants, "reconcile_variants"},
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
		ImageID:   123,
		ImageUUID: "test-uuid-123",
		FilePath:  "/path/to/file",
		FileName:  "test.jpg",
		FileType:  ".jpg",
	}

	result := payload.ToMap()

	expected := map[string]interface{}{
		"image_id":   uint(123),
		"image_uuid": "test-uuid-123",
		"file_path":  "/path/to/file",
		"file_name":  "test.jpg",
		"file_type":  ".jpg",
		"pool_id":    uint(0),
		"node_id":    "",
	}

	assert.Equal(t, expected, result)
}

func TestImageProcessingJobPayloadFromMap(t *testing.T) {
	data := map[string]interface{}{
		"image_id":   float64(123), // JSON numbers are float64
		"image_uuid": "test-uuid-123",
		"file_path":  "/path/to/file",
		"file_name":  "test.jpg",
		"file_type":  ".jpg",
	}

	payload, err := ImageProcessingJobPayloadFromMap(data)
	require.NoError(t, err)

	expected := &ImageProcessingJobPayload{
		ImageID:   123,
		ImageUUID: "test-uuid-123",
		FilePath:  "/path/to/file",
		FileName:  "test.jpg",
		FileType:  ".jpg",
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

func TestPayloadRoundTrip(t *testing.T) {
	t.Run("ImageProcessingJobPayload", func(t *testing.T) {
		original := ImageProcessingJobPayload{
			ImageID:   123,
			ImageUUID: "round-trip-test",
			FilePath:  "/test/path",
			FileName:  "roundtrip.jpg",
			FileType:  ".jpg",
		}

		// Convert to map and back
		data := original.ToMap()
		result, err := ImageProcessingJobPayloadFromMap(data)
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
