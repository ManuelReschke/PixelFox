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
