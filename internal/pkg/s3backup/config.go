package s3backup

import (
	"errors"
	"fmt"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
	"gorm.io/gorm"
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
func (c *Config) GetObjectKey(imageUUID, fileExtension string, year, month, day int) string {
	// Format: images/YYYY/MM/DD/UUID.ext (with day folder as requested)
	return fmt.Sprintf("images/%04d/%02d/%02d/%s%s", year, month, day, imageUUID, fileExtension)
}

// GetAppEnv returns the current application environment
func GetAppEnv() string {
	return env.GetEnv("APP_ENV", "dev")
}

// GetBucketName returns the bucket name as configured (no automatic prefixing)
func (c *Config) GetBucketName() string {
	return c.BucketName
}

// LoadConfigFromStoragePool loads S3 configuration from the highest priority S3 storage pool
func LoadConfigFromStoragePool(db *gorm.DB) (*Config, error) {
	// Find the highest priority S3 storage pool
	s3Pool, err := models.FindHighestPriorityS3Pool(db)
	if err != nil {
		return nil, fmt.Errorf("failed to find S3 storage pool: %w", err)
	}

	if s3Pool == nil {
		return &Config{Enabled: false}, nil // No S3 pools configured, but not an error
	}

	// Validate that all required S3 fields are set
	if s3Pool.S3AccessKeyID == nil || *s3Pool.S3AccessKeyID == "" {
		return nil, fmt.Errorf("S3 storage pool '%s' is missing Access Key ID", s3Pool.Name)
	}
	if s3Pool.S3SecretAccessKey == nil || *s3Pool.S3SecretAccessKey == "" {
		return nil, fmt.Errorf("S3 storage pool '%s' is missing Secret Access Key", s3Pool.Name)
	}
	if s3Pool.S3BucketName == nil || *s3Pool.S3BucketName == "" {
		return nil, fmt.Errorf("S3 storage pool '%s' is missing Bucket Name", s3Pool.Name)
	}
	if s3Pool.S3Region == nil || *s3Pool.S3Region == "" {
		return nil, fmt.Errorf("S3 storage pool '%s' is missing Region", s3Pool.Name)
	}

	config := &Config{
		AccessKeyID:     *s3Pool.S3AccessKeyID,
		SecretAccessKey: *s3Pool.S3SecretAccessKey,
		Region:          *s3Pool.S3Region,
		BucketName:      *s3Pool.S3BucketName,
		Enabled:         true,
	}

	// Set endpoint URL if provided
	if s3Pool.S3EndpointURL != nil && *s3Pool.S3EndpointURL != "" {
		config.EndpointURL = *s3Pool.S3EndpointURL
	}

	return config, nil
}
