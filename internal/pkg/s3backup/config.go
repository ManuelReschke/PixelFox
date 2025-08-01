package s3backup

import (
	"errors"
	"fmt"

	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
)

// Config holds S3 backup configuration
type Config struct {
	AccessKeyID     string
	SecretAccessKey string
	Region          string
	BucketName      string
	EndpointURL     string // Optional for S3-compatible services
	Enabled         bool
}

// LoadConfig loads S3 configuration from environment variables
func LoadConfig() (*Config, error) {
	config := &Config{
		AccessKeyID:     env.GetEnv("S3_ACCESS_KEY_ID", ""),
		SecretAccessKey: env.GetEnv("S3_SECRET_ACCESS_KEY", ""),
		Region:          env.GetEnv("S3_REGION", "us-west-001"),
		BucketName:      env.GetEnv("S3_BUCKET_NAME", ""),
		EndpointURL:     env.GetEnv("S3_ENDPOINT_URL", ""),
		Enabled:         env.GetEnv("S3_BACKUP_ENABLED", "false") == "true",
	}

	// Validate required fields if S3 backup is enabled
	if config.Enabled {
		if config.AccessKeyID == "" {
			return nil, errors.New("S3_ACCESS_KEY_ID is required when S3 backup is enabled")
		}
		if config.SecretAccessKey == "" {
			return nil, errors.New("S3_SECRET_ACCESS_KEY is required when S3 backup is enabled")
		}
		if config.BucketName == "" {
			return nil, errors.New("S3_BUCKET_NAME is required when S3 backup is enabled")
		}
	}

	return config, nil
}

// IsEnabled returns true if S3 backup is enabled
func (c *Config) IsEnabled() bool {
	return c.Enabled
}

// GetObjectKey generates a standardized S3 object key for an image
func (c *Config) GetObjectKey(imageUUID, fileExtension string, year, month int) string {
	// Format: images/YYYY/MM/UUID.ext
	return fmt.Sprintf("images/%04d/%02d/%s%s", year, month, imageUUID, fileExtension)
}

// GetAppEnv returns the current application environment
func GetAppEnv() string {
	return env.GetEnv("APP_ENV", "dev")
}

// GetBucketName returns the bucket name as configured (no automatic prefixing)
func (c *Config) GetBucketName() string {
	return c.BucketName
}
