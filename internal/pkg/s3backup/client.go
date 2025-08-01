package s3backup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/gofiber/fiber/v2/log"
)

// Client wraps the S3 client with backup-specific functionality
type Client struct {
	s3Client *s3.Client
	config   *Config
}

// NewClient creates a new S3 backup client
func NewClient(cfg *Config) (*Client, error) {
	if !cfg.IsEnabled() {
		return nil, fmt.Errorf("S3 backup is disabled")
	}

	// Create AWS config
	awsConfig, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client
	s3Client := s3.NewFromConfig(awsConfig, func(o *s3.Options) {
		if cfg.EndpointURL != "" {
			o.BaseEndpoint = aws.String(cfg.EndpointURL)
			// Backblaze B2 specific settings
			o.UsePathStyle = true   // Force path-style URLs for B2
			o.UseAccelerate = false // Disable transfer acceleration
		}
	})

	client := &Client{
		s3Client: s3Client,
		config:   cfg,
	}

	// Test connection
	if err := client.testConnection(); err != nil {
		return nil, fmt.Errorf("failed to connect to S3: %w", err)
	}

	log.Infof("[S3Backup] Successfully initialized S3 client for bucket: %s", cfg.GetBucketName())
	return client, nil
}

// testConnection tests the S3 connection by checking if the bucket exists
func (c *Client) testConnection() error {
	ctx := context.Background()
	bucketName := c.config.GetBucketName()

	_, err := c.s3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	})

	if err != nil {
		// If bucket doesn't exist, try to create it (for development)
		if GetAppEnv() != "prod" {
			log.Warnf("[S3Backup] Bucket %s not found, attempting to create it", bucketName)
			return c.createBucket(bucketName)
		}
		return fmt.Errorf("bucket %s not accessible: %w", bucketName, err)
	}

	return nil
}

// createBucket creates a new S3 bucket (dev/staging only)
func (c *Client) createBucket(bucketName string) error {
	ctx := context.Background()

	input := &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	}

	// For AWS regions other than us-east-1, we need to specify the location constraint
	// For Backblaze B2, we don't set the LocationConstraint
	if c.config.EndpointURL == "" && c.config.Region != "us-east-1" {
		input.CreateBucketConfiguration = &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(c.config.Region),
		}
	}

	_, err := c.s3Client.CreateBucket(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to create bucket %s: %w", bucketName, err)
	}

	log.Infof("[S3Backup] Successfully created bucket: %s", bucketName)
	return nil
}

// UploadFile uploads a file to S3
func (c *Client) UploadFile(localFilePath, objectKey string) (*UploadResult, error) {
	ctx := context.Background()
	bucketName := c.config.GetBucketName()

	// Open the file
	file, err := os.Open(localFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", localFilePath, err)
	}
	defer file.Close()

	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info for %s: %w", localFilePath, err)
	}

	// Determine content type based on file extension
	contentType := getContentType(filepath.Ext(localFilePath))

	log.Infof("[S3Backup] Starting upload: %s -> s3://%s/%s (Size: %d bytes)",
		localFilePath, bucketName, objectKey, fileInfo.Size())

	// Reset file pointer to beginning
	if _, err := file.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to seek file: %w", err)
	}

	// Upload to S3
	_, err = c.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(bucketName),
		Key:           aws.String(objectKey),
		Body:          file,
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(fileInfo.Size()),
		Metadata: map[string]string{
			"original-path": localFilePath,
			"upload-source": "pixelfox-backup",
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to upload to S3: %w", err)
	}

	result := &UploadResult{
		BucketName:  bucketName,
		ObjectKey:   objectKey,
		Size:        fileInfo.Size(),
		ContentType: contentType,
	}

	log.Infof("[S3Backup] Successfully uploaded: s3://%s/%s", bucketName, objectKey)
	return result, nil
}

// DownloadFile downloads a file from S3 to local storage
func (c *Client) DownloadFile(objectKey, localFilePath string) error {
	ctx := context.Background()
	bucketName := c.config.GetBucketName()

	// Get object from S3
	result, err := c.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return fmt.Errorf("failed to get object from S3: %w", err)
	}
	defer result.Body.Close()

	// Create local directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(localFilePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create local file
	file, err := os.Create(localFilePath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer file.Close()

	// Copy data
	_, err = io.Copy(file, result.Body)
	if err != nil {
		return fmt.Errorf("failed to copy data: %w", err)
	}

	log.Infof("[S3Backup] Successfully downloaded: s3://%s/%s -> %s", bucketName, objectKey, localFilePath)
	return nil
}

// DeleteFile deletes a file from S3
func (c *Client) DeleteFile(objectKey string) error {
	ctx := context.Background()
	bucketName := c.config.GetBucketName()

	_, err := c.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	})

	if err != nil {
		return fmt.Errorf("failed to delete object from S3: %w", err)
	}

	log.Infof("[S3Backup] Successfully deleted: s3://%s/%s", bucketName, objectKey)
	return nil
}

// ObjectExists checks if an object exists in S3
func (c *Client) ObjectExists(objectKey string) (bool, error) {
	ctx := context.Background()
	bucketName := c.config.GetBucketName()

	_, err := c.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	})

	if err != nil {
		// Check if it's a "not found" error
		var notFound *types.NotFound
		if errors.As(err, &notFound) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check object existence: %w", err)
	}

	return true, nil
}

// UploadResult contains the result of a successful upload
type UploadResult struct {
	BucketName  string
	ObjectKey   string
	Size        int64
	ContentType string
}

// getContentType returns the MIME type based on file extension
func getContentType(ext string) string {
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".avif":
		return "image/avif"
	case ".bmp":
		return "image/bmp"
	case ".tiff", ".tif":
		return "image/tiff"
	default:
		return "application/octet-stream"
	}
}
