package s3backup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/gofiber/fiber/v2/log"

	"github.com/ManuelReschke/PixelFox/app/models"
)

// PoolClient is an S3 client that works with Storage Pool configurations
type PoolClient struct {
	s3Client *s3.Client
	pool     *models.StoragePool
}

// NewPoolClient creates a new S3 client from a Storage Pool configuration
func NewPoolClient(pool *models.StoragePool) (*PoolClient, error) {
	if pool.StorageType != models.StorageTypeS3 {
		return nil, fmt.Errorf("storage pool %s is not an S3 storage type", pool.Name)
	}

	// Validate S3 configuration
	if err := pool.ValidateS3Configuration(); err != nil {
		return nil, fmt.Errorf("invalid S3 configuration for pool %s: %w", pool.Name, err)
	}

	// Create AWS config using pool settings
	awsConfig, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(*pool.S3Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			*pool.S3AccessKeyID,
			*pool.S3SecretAccessKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config for pool %s: %w", pool.Name, err)
	}

	// Create S3 client with pool-specific settings
	s3Client := s3.NewFromConfig(awsConfig, func(o *s3.Options) {
		if pool.S3EndpointURL != nil && *pool.S3EndpointURL != "" {
			o.BaseEndpoint = aws.String(*pool.S3EndpointURL)
			// S3-compatible services (like Backblaze B2) often need path-style URLs
			o.UsePathStyle = true
			o.UseAccelerate = false
		}
	})

	return &PoolClient{
		s3Client: s3Client,
		pool:     pool,
	}, nil
}

// UploadFile uploads a file to the S3 storage pool
func (pc *PoolClient) UploadFile(localFilePath, s3Key string) error {
	// Open the file
	file, err := os.Open(localFilePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", localFilePath, err)
	}
	defer file.Close()

	// Add path prefix if configured
	fullKey := s3Key
	if pc.pool.S3PathPrefix != nil && *pc.pool.S3PathPrefix != "" {
		fullKey = fmt.Sprintf("%s/%s", *pc.pool.S3PathPrefix, s3Key)
	}

	// Upload to S3
	_, err = pc.s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(*pc.pool.S3BucketName),
		Key:    aws.String(fullKey),
		Body:   file,
	})

	if err != nil {
		return fmt.Errorf("failed to upload %s to S3 pool %s: %w", localFilePath, pc.pool.Name, err)
	}

	log.Infof("[S3PoolClient] Successfully uploaded %s to S3 pool %s as %s", localFilePath, pc.pool.Name, fullKey)
	return nil
}

// DownloadFile downloads a file from the S3 storage pool
func (pc *PoolClient) DownloadFile(s3Key, localFilePath string) error {
	// Add path prefix if configured
	fullKey := s3Key
	if pc.pool.S3PathPrefix != nil && *pc.pool.S3PathPrefix != "" {
		fullKey = fmt.Sprintf("%s/%s", *pc.pool.S3PathPrefix, s3Key)
	}

	// Download from S3
	result, err := pc.s3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(*pc.pool.S3BucketName),
		Key:    aws.String(fullKey),
	})
	if err != nil {
		return fmt.Errorf("failed to download %s from S3 pool %s: %w", fullKey, pc.pool.Name, err)
	}
	defer result.Body.Close()

	// Create local file
	file, err := os.Create(localFilePath)
	if err != nil {
		return fmt.Errorf("failed to create local file %s: %w", localFilePath, err)
	}
	defer file.Close()

	// Copy content
	_, err = io.Copy(file, result.Body)
	if err != nil {
		return fmt.Errorf("failed to copy content to %s: %w", localFilePath, err)
	}

	log.Infof("[S3PoolClient] Successfully downloaded %s from S3 pool %s to %s", fullKey, pc.pool.Name, localFilePath)
	return nil
}

// DeleteFile deletes a file from the S3 storage pool
func (pc *PoolClient) DeleteFile(s3Key string) error {
	// Add path prefix if configured
	fullKey := s3Key
	if pc.pool.S3PathPrefix != nil && *pc.pool.S3PathPrefix != "" {
		fullKey = fmt.Sprintf("%s/%s", *pc.pool.S3PathPrefix, s3Key)
	}

	// Delete from S3
	_, err := pc.s3Client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(*pc.pool.S3BucketName),
		Key:    aws.String(fullKey),
	})

	if err != nil {
		return fmt.Errorf("failed to delete %s from S3 pool %s: %w", fullKey, pc.pool.Name, err)
	}

	log.Infof("[S3PoolClient] Successfully deleted %s from S3 pool %s", fullKey, pc.pool.Name)
	return nil
}

// FileExists checks if a file exists in the S3 storage pool
func (pc *PoolClient) FileExists(s3Key string) (bool, error) {
	// Add path prefix if configured
	fullKey := s3Key
	if pc.pool.S3PathPrefix != nil && *pc.pool.S3PathPrefix != "" {
		fullKey = fmt.Sprintf("%s/%s", *pc.pool.S3PathPrefix, s3Key)
	}

	// Check if object exists
	_, err := pc.s3Client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: aws.String(*pc.pool.S3BucketName),
		Key:    aws.String(fullKey),
	})

	if err != nil {
		// Check if it's a "not found" error
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check if %s exists in S3 pool %s: %w", fullKey, pc.pool.Name, err)
	}

	return true, nil
}

// GetBucketName returns the bucket name for this storage pool
func (pc *PoolClient) GetBucketName() string {
	if pc.pool.S3BucketName == nil {
		return ""
	}
	return *pc.pool.S3BucketName
}

// GetPathPrefix returns the path prefix for this storage pool
func (pc *PoolClient) GetPathPrefix() string {
	if pc.pool.S3PathPrefix == nil {
		return ""
	}
	return *pc.pool.S3PathPrefix
}

// GetPoolInfo returns information about the storage pool
func (pc *PoolClient) GetPoolInfo() (name string, tier string, priority int) {
	return pc.pool.Name, pc.pool.StorageTier, pc.pool.Priority
}
