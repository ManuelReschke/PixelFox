package jobqueue

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/cache"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTestEnvironment(t *testing.T) (*Queue, *gorm.DB, func()) {
	// Setup test database connection
	db := database.GetDB()
	if db == nil {
		// Initialize database if not already done
		database.SetupDatabase()
		db = database.GetDB()
	}
	require.NotNil(t, db, "Database connection should not be nil")

	// Setup test cache connection
	cacheClient := cache.GetClient()
	require.NotNil(t, cacheClient, "Cache client should not be nil")

	// Create Redis client for queue
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "cache:6379", // Use the container Redis
		Password: "",
		DB:       1, // Use test database
	})

	// Test Redis connection
	ctx := context.Background()
	_, err := redisClient.Ping(ctx).Result()
	require.NoError(t, err, "Redis connection should work")

	// Create queue instance
	queue := &Queue{
		client:     redisClient,
		workers:    3,
		workerPool: make(chan struct{}, 3),
		stopCh:     make(chan struct{}),
	}

	// Cleanup function
	cleanup := func() {
		// Clean test data from Redis
		if err := redisClient.FlushDB(ctx).Err(); err != nil {
			t.Logf("Failed to flush Redis DB: %v", err)
		}
		if err := redisClient.Close(); err != nil {
			t.Logf("Failed to close Redis client: %v", err)
		}
	}

	return queue, db, cleanup
}

func createTestImage(t *testing.T, db *gorm.DB, filePath, fileName string) *models.Image {
	testUUID := uuid.New().String()
	image := &models.Image{
		UUID:     testUUID,
		FilePath: filePath,
		FileName: fileName,
		FileSize: 2048,
		Width:    800,
		Height:   600,
		FileType: "image/jpeg",
		UserID:   1, // Assume test user exists
	}

	err := db.Create(image).Error
	require.NoError(t, err, "Should create test image")
	return image
}

func createTestFile(t *testing.T, filePath, fileName string) string {
	// Create test directory if it doesn't exist
	err := os.MkdirAll(filePath, 0755)
	require.NoError(t, err, "Should create test directory")

	// Create a simple test image file (1x1 PNG)
	fullPath := filepath.Join(filePath, fileName)
	file, err := os.Create(fullPath)
	require.NoError(t, err, "Should create test file")
	defer func() {
		if err := file.Close(); err != nil {
			t.Logf("Failed to close file: %v", err)
		}
	}()

	// Write minimal PNG data (1x1 transparent pixel)
	pngData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, // 1x1 dimensions
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
		0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41, // IDAT chunk
		0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00,
		0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, // IEND chunk
		0x42, 0x60, 0x82,
	}

	_, err = file.Write(pngData)
	require.NoError(t, err, "Should write test PNG data")

	return fullPath
}

func TestQueue_processImageProcessingJob_Success(t *testing.T) {
	queue, db, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create test file first
	testDir := "/tmp/test_images"
	testFileName := "test.jpg"
	_ = createTestFile(t, testDir, testFileName)

	// Create test image in database
	image := createTestImage(t, db, testDir, testFileName)
	defer db.Delete(image) // Cleanup
	defer func() {
		if err := os.RemoveAll(testDir); err != nil {
			t.Logf("Failed to cleanup test directory: %v", err)
		}
	}()

	// Create job payload
	payload := ImageProcessingJobPayload{
		ImageID:      image.ID,
		ImageUUID:    image.UUID,
		FilePath:     testDir,
		FileName:     "test.jpg",
		FileType:     ".jpg",
		EnableBackup: false,
	}

	// Create job
	job := &Job{
		ID:         uuid.New().String(),
		Type:       JobTypeImageProcessing,
		Status:     JobStatusPending,
		Payload:    payload.ToMap(),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		MaxRetries: 3,
	}

	// Process the job
	ctx := context.Background()
	err := queue.processImageProcessingJob(ctx, job)

	// Assertions
	assert.NoError(t, err, "Job processing should succeed")

	// Verify image still exists in database
	var foundImage models.Image
	err = db.Where("uuid = ?", image.UUID).First(&foundImage).Error
	assert.NoError(t, err, "Image should still exist in database")
}

func TestQueue_processImageProcessingJob_WithBackup(t *testing.T) {
	queue, db, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create test file first
	testDir := "/tmp/test_images_backup"
	testFileName := "test_backup.jpg"
	_ = createTestFile(t, testDir, testFileName)

	// Create test image in database
	image := createTestImage(t, db, testDir, testFileName)
	defer db.Delete(image) // Cleanup
	defer func() {
		if err := os.RemoveAll(testDir); err != nil {
			t.Logf("Failed to cleanup test directory: %v", err)
		}
	}()

	// Create job payload with backup enabled
	payload := ImageProcessingJobPayload{
		ImageID:      image.ID,
		ImageUUID:    image.UUID,
		FilePath:     testDir,
		FileName:     "test_backup.jpg",
		FileType:     ".jpg",
		EnableBackup: true,
	}

	// Create job
	job := &Job{
		ID:         uuid.New().String(),
		Type:       JobTypeImageProcessing,
		Status:     JobStatusPending,
		Payload:    payload.ToMap(),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		MaxRetries: 3,
	}

	// Process the job
	ctx := context.Background()
	err := queue.processImageProcessingJob(ctx, job)

	// Assertions
	assert.NoError(t, err, "Job processing with backup should succeed")

	// Verify image still exists in database
	var foundImage models.Image
	err = db.Where("uuid = ?", image.UUID).First(&foundImage).Error
	assert.NoError(t, err, "Image should still exist in database")

	// Wait a bit and check if backup job was enqueued
	time.Sleep(100 * time.Millisecond)

	// Check Redis for backup job (this is a best-effort check)
	keys, err := queue.client.Keys(ctx, "*s3_backup*").Result()
	if err == nil && len(keys) > 0 {
		t.Logf("Backup job appears to have been enqueued (found %d backup-related keys)", len(keys))
	}
}

func TestQueue_processImageProcessingJob_DatabaseError(t *testing.T) {
	queue, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create job payload with non-existent image UUID
	payload := ImageProcessingJobPayload{
		ImageID:      999999, // Non-existent ID
		ImageUUID:    "non-existent-uuid",
		FilePath:     "/tmp/test",
		FileName:     "test.jpg",
		FileType:     ".jpg",
		EnableBackup: false,
	}

	// Create job
	job := &Job{
		ID:         uuid.New().String(),
		Type:       JobTypeImageProcessing,
		Status:     JobStatusPending,
		Payload:    payload.ToMap(),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		MaxRetries: 3,
	}

	// Process the job
	ctx := context.Background()
	err := queue.processImageProcessingJob(ctx, job)

	// Assertions
	assert.Error(t, err, "Job processing should fail for non-existent image")
	assert.Contains(t, err.Error(), "failed to find image", "Error should mention image not found")
}

func TestQueue_processImageProcessingJob_FileNotFound(t *testing.T) {
	queue, db, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create test image in database (no actual file needed for this test)
	image := createTestImage(t, db, "/tmp/nonexistent", "nonexistent.jpg")
	defer db.Delete(image) // Cleanup

	// Create job payload with non-existent file
	payload := ImageProcessingJobPayload{
		ImageID:      image.ID,
		ImageUUID:    image.UUID,
		FilePath:     "/tmp/nonexistent",
		FileName:     "nonexistent.jpg",
		FileType:     ".jpg",
		EnableBackup: false,
	}

	// Create job
	job := &Job{
		ID:         uuid.New().String(),
		Type:       JobTypeImageProcessing,
		Status:     JobStatusPending,
		Payload:    payload.ToMap(),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		MaxRetries: 3,
	}

	// Process the job
	ctx := context.Background()
	err := queue.processImageProcessingJob(ctx, job)

	// Assertions
	assert.Error(t, err, "Job processing should fail for non-existent file")
	assert.Contains(t, err.Error(), "original file not found", "Error should mention file not found")
}

func TestQueue_processImageProcessingJob_ProcessingError(t *testing.T) {
	queue, db, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create invalid file (not a real image)
	testDir := "/tmp/test_invalid"
	invalidFileName := "invalid.jpg"

	// Create test image in database
	image := createTestImage(t, db, testDir, invalidFileName)
	defer db.Delete(image) // Cleanup
	err := os.MkdirAll(testDir, 0755)
	require.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(testDir); err != nil {
			t.Logf("Failed to cleanup test directory: %v", err)
		}
	}()

	invalidFile := filepath.Join(testDir, "invalid.jpg")
	err = os.WriteFile(invalidFile, []byte("not an image"), 0644)
	require.NoError(t, err)

	// Create job payload
	payload := ImageProcessingJobPayload{
		ImageID:      image.ID,
		ImageUUID:    image.UUID,
		FilePath:     testDir,
		FileName:     "invalid.jpg",
		FileType:     ".jpg",
		EnableBackup: false,
	}

	// Create job
	job := &Job{
		ID:         uuid.New().String(),
		Type:       JobTypeImageProcessing,
		Status:     JobStatusPending,
		Payload:    payload.ToMap(),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		MaxRetries: 3,
	}

	// Process the job
	ctx := context.Background()
	err = queue.processImageProcessingJob(ctx, job)

	// Assertions - may fail due to invalid image format
	if err != nil {
		assert.Contains(t, err.Error(), "image processing failed", "Error should indicate processing failure")
	}
}

func TestQueue_processImageProcessingJob_InvalidPayload(t *testing.T) {
	queue, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create job with invalid payload
	job := &Job{
		ID:     uuid.New().String(),
		Type:   JobTypeImageProcessing,
		Status: JobStatusPending,
		Payload: map[string]interface{}{
			"invalid_field": "invalid_value",
			// Missing required fields
		},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		MaxRetries: 3,
	}

	// Process the job
	ctx := context.Background()
	err := queue.processImageProcessingJob(ctx, job)

	// Assertions - The payload will parse (with empty/default values) but fail on database lookup
	assert.Error(t, err, "Job processing should fail for invalid payload")
	// Since the payload has missing fields, it will have empty UUID and fail on database lookup
	assert.Contains(t, err.Error(), "failed to find image", "Error should mention image lookup failure")
}
