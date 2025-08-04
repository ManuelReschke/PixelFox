package jobqueue

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ManuelReschke/PixelFox/app/models"
)

func TestEnqueueImageProcessing_NilImage(t *testing.T) {
	// Execute the test with nil image
	err := EnqueueImageProcessing(nil, false)

	// Assertions
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot enqueue invalid image data")
}

func TestEnqueueImageProcessing_EmptyUUID(t *testing.T) {
	// Create test image with empty UUID
	image := &models.Image{
		ID:       123,
		UUID:     "", // Empty UUID
		FilePath: "/test/path",
		FileName: "test.jpg",
		FileType: ".jpg",
	}

	// Execute the test
	err := EnqueueImageProcessing(image, false)

	// Assertions
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot enqueue invalid image data")
}

func TestProcessImageUnified_BackupEnabled(t *testing.T) {
	// Set environment variable to enable backup
	originalEnv := os.Getenv("S3_BACKUP_ENABLED")
	defer func() {
		if originalEnv != "" {
			os.Setenv("S3_BACKUP_ENABLED", originalEnv)
		} else {
			os.Unsetenv("S3_BACKUP_ENABLED")
		}
	}()
	os.Setenv("S3_BACKUP_ENABLED", "true")

	// Create test image
	image := &models.Image{
		ID:       789,
		UUID:     "unified-test-uuid",
		FilePath: "/unified/test/path",
		FileName: "unified.jpg",
		FileType: ".jpg",
	}

	// Execute the test - this should succeed because it only enqueues a job
	err := ProcessImageUnified(image)

	// The function should succeed as it only enqueues a job to Redis queue
	// The actual processing happens asynchronously in workers
	assert.NoError(t, err, "ProcessImageUnified should successfully enqueue job")
}

func TestProcessImageUnified_BackupDisabled(t *testing.T) {
	// Set environment variable to disable backup
	originalEnv := os.Getenv("S3_BACKUP_ENABLED")
	defer func() {
		if originalEnv != "" {
			os.Setenv("S3_BACKUP_ENABLED", originalEnv)
		} else {
			os.Unsetenv("S3_BACKUP_ENABLED")
		}
	}()
	os.Setenv("S3_BACKUP_ENABLED", "false")

	// Create test image
	image := &models.Image{
		ID:       789,
		UUID:     "unified-test-uuid-no-backup",
		FilePath: "/unified/test/path",
		FileName: "unified.jpg",
		FileType: ".jpg",
	}

	// Execute the test
	err := ProcessImageUnified(image)

	// The function should succeed as it only enqueues a job to Redis queue
	// The actual processing happens asynchronously in workers
	assert.NoError(t, err, "ProcessImageUnified should successfully enqueue job")
}

func TestProcessImageUnified_InvalidImage(t *testing.T) {
	// Execute the test with nil image
	err := ProcessImageUnified(nil)

	// Assertions
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot enqueue invalid image data")
}
