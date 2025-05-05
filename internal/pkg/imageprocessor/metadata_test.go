package imageprocessor_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/imageprocessor"
	"github.com/stretchr/testify/assert"
)

func TestExtractMetadata(t *testing.T) {
	// Create a test image model
	image := &models.Image{}

	// Get the absolute path to the test image
	testImagePath := filepath.Join("testdata", "image-with-meta-data.jpg")

	// Call the function we want to test
	err := imageprocessor.ExtractMetadata(image, testImagePath)

	// Verify no error occurred
	assert.NoError(t, err, "ExtractMetadata should not return an error")

	// Test that all metadata fields are not empty
	assert.NotNil(t, image.CameraModel, "CameraModel should not be nil")
	assert.NotEmpty(t, *image.CameraModel, "CameraModel should not be empty")
	assert.Equal(t, "ONEPLUS A3003", *image.CameraModel, "CameraModel should be 'ONEPLUS A3003'")

	// expectedTime := time.Date(2019, 4, 8, 17, 12, 37, 0, time.UTC)
	expectedTime := time.Date(2019, 4, 8, 17, 12, 37, 0, time.Local)
	assert.NotNil(t, image.TakenAt, "TakenAt should not be nil")
	assert.Equal(t, &expectedTime, image.TakenAt)

	assert.NotNil(t, image.Latitude, "Latitude should not be nil")
	assert.NotNil(t, image.Longitude, "Longitude should not be nil")
	assert.InDelta(t, 39.703317, *image.Latitude, 0.000001, "Latitude should be approximately 39.703317")
	assert.InDelta(t, 21.649384, *image.Longitude, 0.000001, "Longitude should be approximately 21.649384")

	assert.NotNil(t, image.ExposureTime, "ExposureTime should not be nil")
	assert.NotEmpty(t, *image.ExposureTime, "ExposureTime should not be empty")
	assert.Equal(t, "1/1301", *image.ExposureTime, "ExposureTime should be '1/1301'")

	assert.NotNil(t, image.Aperture, "Aperture should not be nil")
	assert.NotEmpty(t, *image.Aperture, "Aperture should not be empty")
	assert.Equal(t, "200/100", *image.Aperture, "Aperture should be '200/100'")

	assert.NotNil(t, image.ISO, "ISO should not be nil")
	assert.NotZero(t, *image.ISO, "ISO should not be zero")
	assert.Equal(t, 100, *image.ISO, "ISO should be 100")

	assert.NotNil(t, image.FocalLength, "FocalLength should not be nil")
	assert.NotEmpty(t, *image.FocalLength, "FocalLength should not be empty")
	assert.Equal(t, "426/100", *image.FocalLength, "FocalLength should be '426/100'")

	assert.NotNil(t, image.Metadata, "Metadata should not be nil")
}
