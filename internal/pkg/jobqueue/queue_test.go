package jobqueue

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestNewQueue tests the queue constructor
func TestNewQueue(t *testing.T) {
	tests := []struct {
		name            string
		workers         int
		expectedWorkers int
	}{
		{"Valid worker count", 5, 5},
		{"Zero workers", 0, 3},
		{"Negative workers", -1, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queue := NewQueue(tt.workers)

			assert.NotNil(t, queue)
			assert.Equal(t, tt.expectedWorkers, queue.workers)
			assert.NotNil(t, queue.workerPool)
			assert.Equal(t, tt.expectedWorkers, cap(queue.workerPool))
			assert.NotNil(t, queue.stopCh)
			assert.False(t, queue.running)
		})
	}
}

func TestQueue_EnqueueJob(t *testing.T) {
	// Skip this test - requires Redis connection
	t.Skip("Skipping integration test that requires Redis connection")
}

func TestQueue_EnqueueJob_PipelineError(t *testing.T) {
	// Skip this test - requires Redis connection
	t.Skip("Skipping integration test that requires Redis connection")
}

func TestQueue_GetJob(t *testing.T) {
	// Skip this test - requires Redis connection
	t.Skip("Skipping integration test that requires Redis connection")
}

func TestQueue_GetJob_NotFound(t *testing.T) {
	// Skip this test - requires Redis connection
	t.Skip("Skipping integration test that requires Redis connection")
}

func TestQueue_GetJobStats(t *testing.T) {
	// Skip this test - requires Redis connection
	t.Skip("Skipping integration test that requires Redis connection")
}

func TestQueue_GetQueueSize(t *testing.T) {
	// Skip this test - requires Redis connection
	t.Skip("Skipping integration test that requires Redis connection")
}

func TestQueue_GetProcessingSize(t *testing.T) {
	// Skip this test - requires Redis connection
	t.Skip("Skipping integration test that requires Redis connection")
}

func TestQueue_StartStop(t *testing.T) {
	queue := NewQueue(2)

	// Initial state
	assert.False(t, queue.running)

	// Starting/stopping without Redis should be safe for basic state checks
	// We skip the actual start/stop as it would try to connect to Redis
	t.Skip("Skipping Redis-dependent start/stop test")
}

func TestQueue_updateJob(t *testing.T) {
	// Skip this test - requires Redis connection
	t.Skip("Skipping integration test that requires Redis connection")
}

func TestQueue_removeFromProcessing(t *testing.T) {
	// Skip this test - requires Redis connection
	t.Skip("Skipping integration test that requires Redis connection")
}

func TestQueue_removeCompletedJob(t *testing.T) {
	// Skip this test - requires Redis connection
	t.Skip("Skipping integration test that requires Redis connection")
}

func TestQueue_updateJobStats(t *testing.T) {
	// Skip this test - requires Redis connection
	t.Skip("Skipping integration test that requires Redis connection")
}

func TestConstants(t *testing.T) {
	// Test Redis key constants
	assert.Equal(t, "job:", JobKeyPrefix)
	assert.Equal(t, "job_queue", JobQueueKey)
	assert.Equal(t, "job_processing", JobProcessingKey)
	assert.Equal(t, "job_stats", JobStatsKey)

	// Test job settings constants
	assert.Equal(t, 3, DefaultMaxRetries)
	assert.Equal(t, 24*time.Hour, JobTTL)
}
