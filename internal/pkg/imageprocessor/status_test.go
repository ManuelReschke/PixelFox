package imageprocessor_test

import (
	"fmt"
	"testing"

	"github.com/ManuelReschke/PixelFox/internal/pkg/imageprocessor"
	"github.com/stretchr/testify/assert"
)

func TestIsImageProcessingFailed(t *testing.T) {
	originalGet := imageprocessor.GetCacheImplementation
	t.Cleanup(func() {
		imageprocessor.GetCacheImplementation = originalGet
	})

	t.Run("returns true for failed status", func(t *testing.T) {
		imageprocessor.GetCacheImplementation = func(key string) (string, error) {
			return imageprocessor.STATUS_FAILED, nil
		}

		assert.True(t, imageprocessor.IsImageProcessingFailed("img-123"))
	})

	t.Run("returns false for non-failed status", func(t *testing.T) {
		imageprocessor.GetCacheImplementation = func(key string) (string, error) {
			return imageprocessor.STATUS_COMPLETED, nil
		}

		assert.False(t, imageprocessor.IsImageProcessingFailed("img-123"))
	})

	t.Run("returns false on cache error", func(t *testing.T) {
		imageprocessor.GetCacheImplementation = func(key string) (string, error) {
			return "", fmt.Errorf("cache miss")
		}

		assert.False(t, imageprocessor.IsImageProcessingFailed("img-123"))
	})

	t.Run("returns false and skips cache for empty uuid", func(t *testing.T) {
		called := false
		imageprocessor.GetCacheImplementation = func(key string) (string, error) {
			called = true
			return imageprocessor.STATUS_FAILED, nil
		}

		assert.False(t, imageprocessor.IsImageProcessingFailed(""))
		assert.False(t, called)
	})
}
