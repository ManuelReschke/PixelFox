package jobqueue

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ManuelReschke/PixelFox/app/models"
)

func TestEnqueueImageProcessing_NilImage(t *testing.T) {
	// Execute the test with nil image
	err := EnqueueImageProcessing(nil)

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
	err := EnqueueImageProcessing(image)

	// Assertions
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot enqueue invalid image data")
}

func TestProcessImageUnified(t *testing.T) {
	host, port, password := resolveTestRedis(t)
	configureTestCache(host, port, password)
	globalManager = nil
	managerOnce = sync.Once{}

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

func TestProcessImageUnified_InvalidImage(t *testing.T) {
	// Execute the test with nil image
	err := ProcessImageUnified(nil)

	// Assertions
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot enqueue invalid image data")
}
