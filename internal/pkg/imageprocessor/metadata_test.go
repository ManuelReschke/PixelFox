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

	// Verify that the metadata object was created and attached to the image
	assert.NotNil(t, image.Metadata, "Image.Metadata should not be nil")

	// Access the metadata through the relationship
	metadata := image.Metadata

	// Test that all metadata fields are not empty
	assert.NotNil(t, metadata.CameraModel, "CameraModel should not be nil")
	assert.NotEmpty(t, *metadata.CameraModel, "CameraModel should not be empty")
	assert.Equal(t, "ONEPLUS A3003", *metadata.CameraModel, "CameraModel should be 'ONEPLUS A3003'")

	// expectedTime := time.Date(2019, 4, 8, 17, 12, 37, 0, time.UTC)
	expectedTime := time.Date(2019, 4, 8, 17, 12, 37, 0, time.Local)
	assert.NotNil(t, metadata.TakenAt, "TakenAt should not be nil")
	assert.Equal(t, &expectedTime, metadata.TakenAt)

	assert.NotNil(t, metadata.Latitude, "Latitude should not be nil")
	assert.NotNil(t, metadata.Longitude, "Longitude should not be nil")
	assert.InDelta(t, 39.703317, *metadata.Latitude, 0.000001, "Latitude should be approximately 39.703317")
	assert.InDelta(t, 21.649384, *metadata.Longitude, 0.000001, "Longitude should be approximately 21.649384")

	assert.NotNil(t, metadata.ExposureTime, "ExposureTime should not be nil")
	assert.NotEmpty(t, *metadata.ExposureTime, "ExposureTime should not be empty")
	assert.Equal(t, "1/1301", *metadata.ExposureTime, "ExposureTime should be '1/1301'")

	assert.NotNil(t, metadata.Aperture, "Aperture should not be nil")
	assert.NotEmpty(t, *metadata.Aperture, "Aperture should not be empty")
	assert.Equal(t, "200/100", *metadata.Aperture, "Aperture should be '200/100'")

	assert.NotNil(t, metadata.ISO, "ISO should not be nil")
	assert.NotZero(t, *metadata.ISO, "ISO should not be zero")
	assert.Equal(t, 100, *metadata.ISO, "ISO should be 100")

	assert.NotNil(t, metadata.FocalLength, "FocalLength should not be nil")
	assert.NotEmpty(t, *metadata.FocalLength, "FocalLength should not be empty")
	assert.Equal(t, "426/100", *metadata.FocalLength, "FocalLength should be '426/100'")

	assert.NotNil(t, metadata.Metadata, "Metadata should not be nil")
}
