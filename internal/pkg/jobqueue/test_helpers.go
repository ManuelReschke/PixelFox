//go:build test
// +build test

package jobqueue

import (
	"time"
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
				ImageID:   123,
				ImageUUID: "test-image-uuid",
				FilePath:  "/test/path",
				FileName:  "test.jpg",
				FileType:  ".jpg",
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
