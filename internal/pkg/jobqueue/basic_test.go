package jobqueue

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBasicJobTypes tests the basic job type constants
func TestBasicJobTypes(t *testing.T) {
	assert.Equal(t, "image_processing", string(JobTypeImageProcessing))
	assert.Equal(t, "pool_move_enqueue", string(JobTypePoolMoveEnqueue))
	assert.Equal(t, "move_image", string(JobTypeMoveImage))
	assert.Equal(t, "delete_image", string(JobTypeDeleteImage))
	assert.Equal(t, "reconcile_variants", string(JobTypeReconcileVariants))
}

// TestBasicJobStatus tests the basic job status constants
func TestBasicJobStatus(t *testing.T) {
	assert.Equal(t, "pending", string(JobStatusPending))
	assert.Equal(t, "processing", string(JobStatusProcessing))
	assert.Equal(t, "completed", string(JobStatusCompleted))
	assert.Equal(t, "failed", string(JobStatusFailed))
	assert.Equal(t, "retrying", string(JobStatusRetrying))
}

// TestJob_BasicMethods tests basic job methods
func TestJob_BasicMethods(t *testing.T) {
	job := &Job{
		Status:     JobStatusFailed,
		RetryCount: 1,
		MaxRetries: 3,
	}

	// Test IsRetryable
	assert.True(t, job.IsRetryable())

	job.RetryCount = 3
	assert.False(t, job.IsRetryable())

	// Test status transitions
	beforeTime := time.Now()

	job.MarkAsProcessing()
	assert.Equal(t, JobStatusProcessing, job.Status)
	assert.NotNil(t, job.ProcessedAt)
	assert.True(t, job.UpdatedAt.After(beforeTime))

	job.MarkAsCompleted()
	assert.Equal(t, JobStatusCompleted, job.Status)
	assert.NotNil(t, job.CompletedAt)
	assert.Empty(t, job.ErrorMsg)

	job.MarkAsFailed("test error")
	assert.Equal(t, JobStatusFailed, job.Status)
	assert.Equal(t, "test error", job.ErrorMsg)
	assert.Equal(t, 4, job.RetryCount)

	job.MarkAsRetrying()
	assert.Equal(t, JobStatusRetrying, job.Status)
}

// TestImageProcessingJobPayload_Serialization tests payload serialization
func TestImageProcessingJobPayload_Serialization(t *testing.T) {
	payload := ImageProcessingJobPayload{
		ImageID:   123,
		ImageUUID: "test-uuid",
		FilePath:  "/test/path",
		FileName:  "test.jpg",
		FileType:  ".jpg",
	}

	// Test ToMap
	data := payload.ToMap()
	expected := map[string]interface{}{
		"image_id":   uint(123),
		"image_uuid": "test-uuid",
		"file_path":  "/test/path",
		"file_name":  "test.jpg",
		"file_type":  ".jpg",
		"pool_id":    uint(0),
		"node_id":    "",
	}
	assert.Equal(t, expected, data)

	// Test FromMap
	result, err := ImageProcessingJobPayloadFromMap(data)
	require.NoError(t, err)
	assert.Equal(t, &payload, result)
}

// TestJobSerialization tests full job JSON serialization
func TestJobSerialization(t *testing.T) {
	now := time.Now()
	job := &Job{
		ID:         "test-job-123",
		Type:       JobTypeImageProcessing,
		Status:     JobStatusPending,
		Payload:    map[string]interface{}{"test": "data"},
		CreatedAt:  now,
		UpdatedAt:  now,
		RetryCount: 0,
		MaxRetries: 3,
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(job)
	require.NoError(t, err)

	// Test JSON unmarshaling
	var result Job
	err = json.Unmarshal(jsonData, &result)
	require.NoError(t, err)

	assert.Equal(t, job.ID, result.ID)
	assert.Equal(t, job.Type, result.Type)
	assert.Equal(t, job.Status, result.Status)
	assert.Equal(t, job.Payload, result.Payload)
	assert.Equal(t, job.RetryCount, result.RetryCount)
	assert.Equal(t, job.MaxRetries, result.MaxRetries)
}

// TestBasicNewQueue tests queue creation
func TestBasicNewQueue(t *testing.T) {
	t.Run("Valid worker count", func(t *testing.T) {
		queue := NewQueue(5)
		assert.NotNil(t, queue)
		assert.Equal(t, 5, queue.workers)
		assert.Equal(t, 5, cap(queue.workerPool))
		assert.NotNil(t, queue.stopCh)
		assert.False(t, queue.running)
	})

	t.Run("Zero workers defaults to 3", func(t *testing.T) {
		queue := NewQueue(0)
		assert.Equal(t, 3, queue.workers)
		assert.Equal(t, 3, cap(queue.workerPool))
	})

	t.Run("Negative workers defaults to 3", func(t *testing.T) {
		queue := NewQueue(-1)
		assert.Equal(t, 3, queue.workers)
		assert.Equal(t, 3, cap(queue.workerPool))
	})
}

// TestBasicConstants tests package constants
func TestBasicConstants(t *testing.T) {
	assert.Equal(t, "job:", JobKeyPrefix)
	assert.Equal(t, "job_queue", JobQueueKey)
	assert.Equal(t, "job_processing", JobProcessingKey)
	assert.Equal(t, "job_stats", JobStatsKey)
	assert.Equal(t, 3, DefaultMaxRetries)
	assert.Equal(t, 24*time.Hour, JobTTL)
}

// TestPayloadFromMapErrors tests error handling in payload deserialization
func TestPayloadFromMapErrors(t *testing.T) {
	t.Run("ImageProcessingJobPayload invalid data", func(t *testing.T) {
		invalidData := map[string]interface{}{
			"invalid": make(chan int), // Channels can't be marshaled to JSON
		}

		payload, err := ImageProcessingJobPayloadFromMap(invalidData)
		assert.Error(t, err)
		assert.Nil(t, payload)
	})
}
