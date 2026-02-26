package storage

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2/log"
	"gorm.io/gorm"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/s3backup"
)

// StorageManager handles all storage operations across multiple pools
type StorageManager struct {
	db *gorm.DB
}

// PoolMetrics represents performance metrics for a storage pool
type PoolMetrics struct {
	PoolID        uint          `json:"pool_id"`
	PoolName      string        `json:"pool_name"`
	AvgWriteTime  time.Duration `json:"avg_write_time"`
	AvgReadTime   time.Duration `json:"avg_read_time"`
	ErrorCount    int64         `json:"error_count"`
	SuccessCount  int64         `json:"success_count"`
	LastOperation time.Time     `json:"last_operation"`
	HealthScore   float64       `json:"health_score"` // 0.0 - 1.0
}

// FileOperation represents a file operation result
type FileOperation struct {
	Success  bool          `json:"success"`
	FilePath string        `json:"file_path"`
	Duration time.Duration `json:"duration"`
	Error    error         `json:"error,omitempty"`
	PoolID   uint          `json:"pool_id"`
	PoolName string        `json:"pool_name"`
}

// NewStorageManager creates a new storage manager instance
func NewStorageManager() *StorageManager {
	return &StorageManager{
		db: database.GetDB(),
	}
}

// SelectPoolForFile selects the optimal storage pool for a file of given size
// DEPRECATED: Use SelectPoolForUpload for new uploads to ensure hot storage priority
func (sm *StorageManager) SelectPoolForFile(fileSize int64) (*models.StoragePool, error) {
	if sm.db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	pool, err := models.SelectOptimalPool(sm.db, fileSize)
	if err != nil {
		log.Errorf("[StorageManager] Failed to select optimal pool for file size %d: %v", fileSize, err)
		return nil, err
	}

	log.Debugf("[StorageManager] Selected pool '%s' (ID: %d) for file size %d bytes", pool.Name, pool.ID, fileSize)
	return pool, nil
}

// SelectPoolForUpload selects the optimal hot storage pool for new file uploads
// This ensures all new uploads go to hot storage first for best performance
func (sm *StorageManager) SelectPoolForUpload(fileSize int64) (*models.StoragePool, error) {
	if sm.db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	pool, err := models.SelectOptimalPoolForUpload(sm.db, fileSize)
	if err != nil {
		log.Errorf("[StorageManager] Failed to select hot storage pool for upload size %d: %v", fileSize, err)
		return nil, err
	}

	log.Infof("[StorageManager] Selected %s storage pool '%s' (ID: %d) for upload (%d bytes)",
		pool.StorageTier, pool.Name, pool.ID, fileSize)
	return pool, nil
}

// SelectPoolByTier selects a storage pool from a specific tier
func (sm *StorageManager) SelectPoolByTier(tier string, fileSize int64) (*models.StoragePool, error) {
	if sm.db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	pools, err := models.FindActiveStoragePoolsByTier(sm.db, tier)
	if err != nil {
		return nil, fmt.Errorf("failed to get %s storage pools: %w", tier, err)
	}

	if len(pools) == 0 {
		return nil, fmt.Errorf("no active %s storage pools available", tier)
	}

	// Find the best pool in this tier with enough space
	for _, pool := range pools {
		if pool.CanAcceptFile(fileSize) {
			log.Debugf("[StorageManager] Selected %s storage pool '%s' for file size %d bytes", tier, pool.Name, fileSize)
			return &pool, nil
		}
	}

	return nil, fmt.Errorf("no %s storage pools can accept file of size %d bytes", tier, fileSize)
}

// SaveFile saves a file to the specified storage pool
func (sm *StorageManager) SaveFile(data io.Reader, filename string, poolID uint) (*FileOperation, error) {
	startTime := time.Now()

	operation := &FileOperation{
		PoolID: poolID,
	}

	// Get pool information
	pool, err := models.FindStoragePoolByID(sm.db, poolID)
	if err != nil {
		operation.Error = fmt.Errorf("failed to find storage pool %d: %w", poolID, err)
		operation.Duration = time.Since(startTime)
		return operation, operation.Error
	}

	operation.PoolName = pool.Name

	// Validate pool health
	if !pool.IsHealthy() {
		operation.Error = fmt.Errorf("storage pool '%s' is not healthy", pool.Name)
		operation.Duration = time.Since(startTime)
		return operation, operation.Error
	}

	relativePath, err := cleanRelativeStoragePath(filename)
	if err != nil {
		operation.Error = fmt.Errorf("invalid file path %q: %w", filename, err)
		operation.Duration = time.Since(startTime)
		return operation, operation.Error
	}

	var bytesWritten int64
	if pool.IsS3Storage() {
		s3Client, err := s3backup.NewPoolClient(pool)
		if err != nil {
			operation.Error = fmt.Errorf("failed to initialize S3 client for pool '%s': %w", pool.Name, err)
			operation.Duration = time.Since(startTime)
			return operation, operation.Error
		}

		tmpFile, err := os.CreateTemp("", "pixelfox-storage-upload-*")
		if err != nil {
			operation.Error = fmt.Errorf("failed to create temporary upload file: %w", err)
			operation.Duration = time.Since(startTime)
			return operation, operation.Error
		}
		tmpPath := tmpFile.Name()
		defer os.Remove(tmpPath)

		bytesWritten, err = io.Copy(tmpFile, data)
		if closeErr := tmpFile.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
		if err != nil {
			operation.Error = fmt.Errorf("failed to buffer upload for S3: %w", err)
			operation.Duration = time.Since(startTime)
			return operation, operation.Error
		}

		s3Key := toS3ObjectKey(relativePath)
		if err := s3Client.UploadFile(tmpPath, s3Key); err != nil {
			operation.Error = fmt.Errorf("failed to upload file %s to S3 pool '%s': %w", s3Key, pool.Name, err)
			operation.Duration = time.Since(startTime)
			return operation, operation.Error
		}
		operation.FilePath = s3Key
	} else {
		fullPath := filepath.Join(pool.BasePath, filepath.FromSlash(relativePath))
		operation.FilePath = fullPath

		// Ensure directory exists
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			operation.Error = fmt.Errorf("failed to create directory %s: %w", dir, err)
			operation.Duration = time.Since(startTime)
			return operation, operation.Error
		}

		// Create and write file
		file, err := os.Create(fullPath)
		if err != nil {
			operation.Error = fmt.Errorf("failed to create file %s: %w", fullPath, err)
			operation.Duration = time.Since(startTime)
			return operation, operation.Error
		}
		defer file.Close()

		// Copy data to file and track size
		bytesWritten, err = io.Copy(file, data)
		if err != nil {
			operation.Error = fmt.Errorf("failed to write file %s: %w", fullPath, err)
			operation.Duration = time.Since(startTime)
			// Clean up partial file
			_ = os.Remove(fullPath)
			return operation, operation.Error
		}
	}

	// Update pool usage
	if err := pool.UpdateUsedSize(sm.db, bytesWritten); err != nil {
		log.Errorf("[StorageManager] Failed to update pool usage for %s: %v", pool.Name, err)
		// Don't fail the operation for this
	}

	operation.Success = true
	operation.Duration = time.Since(startTime)

	log.Infof("[StorageManager] Successfully saved file %s (%d bytes) to pool '%s' in %v",
		relativePath, bytesWritten, pool.Name, operation.Duration)

	return operation, nil
}

// DeleteFile removes a file from the specified storage pool
func (sm *StorageManager) DeleteFile(relativePath string, poolID uint) (*FileOperation, error) {
	startTime := time.Now()

	operation := &FileOperation{
		PoolID: poolID,
	}

	// Get pool information
	pool, err := models.FindStoragePoolByID(sm.db, poolID)
	if err != nil {
		operation.Error = fmt.Errorf("failed to find storage pool %d: %w", poolID, err)
		operation.Duration = time.Since(startTime)
		return operation, operation.Error
	}

	operation.PoolName = pool.Name

	cleanRelPath, err := cleanRelativeStoragePath(relativePath)
	if err != nil {
		operation.Error = fmt.Errorf("invalid file path %q: %w", relativePath, err)
		operation.Duration = time.Since(startTime)
		return operation, operation.Error
	}

	fileSize := int64(0)
	if pool.IsS3Storage() {
		s3Client, err := s3backup.NewPoolClient(pool)
		if err != nil {
			operation.Error = fmt.Errorf("failed to initialize S3 client for pool '%s': %w", pool.Name, err)
			operation.Duration = time.Since(startTime)
			return operation, operation.Error
		}

		s3Key := toS3ObjectKey(cleanRelPath)
		operation.FilePath = s3Key
		if size, err := s3Client.GetFileSize(s3Key); err == nil {
			fileSize = size
		}
		if err := s3Client.DeleteFile(s3Key); err != nil {
			operation.Error = fmt.Errorf("failed to delete S3 object %s: %w", s3Key, err)
			operation.Duration = time.Since(startTime)
			return operation, operation.Error
		}
	} else {
		fullPath := filepath.Join(pool.BasePath, filepath.FromSlash(cleanRelPath))
		operation.FilePath = fullPath

		// Get file size before deletion for usage tracking
		fileInfo, err := os.Stat(fullPath)
		if err == nil {
			fileSize = fileInfo.Size()
		}

		// Delete the file
		if err := os.Remove(fullPath); err != nil {
			if !os.IsNotExist(err) {
				operation.Error = fmt.Errorf("failed to delete file %s: %w", fullPath, err)
				operation.Duration = time.Since(startTime)
				return operation, operation.Error
			}
			// File doesn't exist, consider it successful
		}
	}

	// Update pool usage (subtract file size)
	if fileSize > 0 {
		if err := pool.UpdateUsedSize(sm.db, -fileSize); err != nil {
			log.Errorf("[StorageManager] Failed to update pool usage for %s: %v", pool.Name, err)
			// Don't fail the operation for this
		}
	}

	operation.Success = true
	operation.Duration = time.Since(startTime)

	log.Infof("[StorageManager] Successfully deleted file %s (%d bytes) from pool '%s' in %v",
		cleanRelPath, fileSize, pool.Name, operation.Duration)

	return operation, nil
}

// MigrateFile moves a file from one storage pool to another
func (sm *StorageManager) MigrateFile(relativePath string, sourcePoolID, targetPoolID uint) (*FileOperation, error) {
	startTime := time.Now()

	operation := &FileOperation{}

	// Get source and target pools
	sourcePool, err := models.FindStoragePoolByID(sm.db, sourcePoolID)
	if err != nil {
		operation.Error = fmt.Errorf("failed to find source storage pool %d: %w", sourcePoolID, err)
		operation.Duration = time.Since(startTime)
		return operation, operation.Error
	}

	targetPool, err := models.FindStoragePoolByID(sm.db, targetPoolID)
	if err != nil {
		operation.Error = fmt.Errorf("failed to find target storage pool %d: %w", targetPoolID, err)
		operation.Duration = time.Since(startTime)
		return operation, operation.Error
	}

	operation.PoolID = targetPoolID
	operation.PoolName = targetPool.Name

	relPath, err := cleanRelativeStoragePath(relativePath)
	if err != nil {
		operation.Error = fmt.Errorf("invalid file path %q: %w", relativePath, err)
		operation.Duration = time.Since(startTime)
		return operation, operation.Error
	}

	// Check target pool health and capacity
	if !targetPool.IsHealthy() {
		operation.Error = fmt.Errorf("target storage pool '%s' is not healthy", targetPool.Name)
		operation.Duration = time.Since(startTime)
		return operation, operation.Error
	}

	exists, fileSize, err := sm.FileExists(relPath, sourcePoolID)
	if err != nil {
		operation.Error = fmt.Errorf("failed to inspect source file %s: %w", relPath, err)
		operation.Duration = time.Since(startTime)
		return operation, operation.Error
	}
	if !exists {
		operation.Error = fmt.Errorf("source file not found: %s", relPath)
		operation.Duration = time.Since(startTime)
		return operation, operation.Error
	}

	// Check if target pool can accept the file
	if !targetPool.CanAcceptFile(fileSize) {
		operation.Error = fmt.Errorf("target pool '%s' cannot accept file of size %d bytes", targetPool.Name, fileSize)
		operation.Duration = time.Since(startTime)
		return operation, operation.Error
	}

	// Local no-op if source and destination are identical
	if !sourcePool.IsS3Storage() && !targetPool.IsS3Storage() {
		sourcePath := filepath.Clean(filepath.Join(sourcePool.BasePath, filepath.FromSlash(relPath)))
		targetPath := filepath.Clean(filepath.Join(targetPool.BasePath, filepath.FromSlash(relPath)))
		if strings.EqualFold(sourcePath, targetPath) {
			operation.Success = true
			operation.FilePath = targetPath
			operation.Duration = time.Since(startTime)
			log.Infof("[StorageManager] Source and target are identical (%s), migration skipped", sourcePath)
			return operation, nil
		}
	}

	// Open source stream depending on pool type
	var (
		sourceReader io.ReadCloser
		tempFilePath string
	)
	if sourcePool.IsS3Storage() {
		s3Client, err := s3backup.NewPoolClient(sourcePool)
		if err != nil {
			operation.Error = fmt.Errorf("failed to initialize S3 client for source pool '%s': %w", sourcePool.Name, err)
			operation.Duration = time.Since(startTime)
			return operation, operation.Error
		}
		tmpFile, err := os.CreateTemp("", "pixelfox-storage-migrate-*")
		if err != nil {
			operation.Error = fmt.Errorf("failed to create temp file for migration: %w", err)
			operation.Duration = time.Since(startTime)
			return operation, operation.Error
		}
		tempFilePath = tmpFile.Name()
		_ = tmpFile.Close()
		defer os.Remove(tempFilePath)

		s3Key := toS3ObjectKey(relPath)
		if err := s3Client.DownloadFile(s3Key, tempFilePath); err != nil {
			operation.Error = fmt.Errorf("failed to download source object %s from pool '%s': %w", s3Key, sourcePool.Name, err)
			operation.Duration = time.Since(startTime)
			return operation, operation.Error
		}
		sourceReader, err = os.Open(tempFilePath)
		if err != nil {
			operation.Error = fmt.Errorf("failed to open temporary source file: %w", err)
			operation.Duration = time.Since(startTime)
			return operation, operation.Error
		}
	} else {
		sourcePath := filepath.Join(sourcePool.BasePath, filepath.FromSlash(relPath))
		f, err := os.Open(sourcePath)
		if err != nil {
			operation.Error = fmt.Errorf("failed to open source file %s: %w", sourcePath, err)
			operation.Duration = time.Since(startTime)
			return operation, operation.Error
		}
		sourceReader = f
	}
	defer sourceReader.Close()

	saveOp, err := sm.SaveFile(sourceReader, relPath, targetPoolID)
	if err != nil || !saveOp.Success {
		if err == nil && saveOp != nil && saveOp.Error != nil {
			err = saveOp.Error
		}
		operation.Error = fmt.Errorf("failed to save file to target pool '%s': %w", targetPool.Name, err)
		operation.Duration = time.Since(startTime)
		return operation, operation.Error
	}

	if _, err := sm.DeleteFile(relPath, sourcePoolID); err != nil {
		operation.Error = fmt.Errorf("file copied to target but failed deleting source: %w", err)
		operation.Duration = time.Since(startTime)
		return operation, operation.Error
	}

	operation.FilePath = saveOp.FilePath

	operation.Success = true
	operation.Duration = time.Since(startTime)

	log.Infof("[StorageManager] Successfully migrated file %s (%d bytes) from pool '%s' to pool '%s' in %v",
		relPath, fileSize, sourcePool.Name, targetPool.Name, operation.Duration)

	return operation, nil
}

// copyFile copies a file from source to destination
func (sm *StorageManager) copyFile(source, destination string) error {
	sourceFile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		os.Remove(destination) // Clean up on error
		return err
	}

	return destFile.Sync()
}

// GetPoolStats returns statistics for a specific storage pool
func (sm *StorageManager) GetPoolStats(poolID uint) (*models.StoragePoolStats, error) {
	return models.GetStoragePoolStats(sm.db, poolID)
}

// GetAllPoolStats returns statistics for all storage pools
func (sm *StorageManager) GetAllPoolStats() ([]models.StoragePoolStats, error) {
	return models.GetAllStoragePoolStats(sm.db)
}

// HealthCheck performs health checks on all storage pools
func (sm *StorageManager) HealthCheck() (map[uint]bool, error) {
	pools, err := models.FindAllStoragePools(sm.db)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage pools: %w", err)
	}

	healthStatus := make(map[uint]bool)
	for _, pool := range pools {
		healthStatus[pool.ID] = pool.IsHealthy()
		if !healthStatus[pool.ID] {
			log.Warnf("[StorageManager] Pool '%s' (ID: %d) failed health check", pool.Name, pool.ID)
		}
	}

	return healthStatus, nil
}

// GetFilePath returns the full file path for a file in the specified pool
func (sm *StorageManager) GetFilePath(relativePath string, poolID uint) (string, error) {
	pool, err := models.FindStoragePoolByID(sm.db, poolID)
	if err != nil {
		return "", fmt.Errorf("failed to find storage pool %d: %w", poolID, err)
	}

	cleanRelPath, err := cleanRelativeStoragePath(relativePath)
	if err != nil {
		return "", err
	}
	if pool.IsS3Storage() {
		return toS3ObjectKey(cleanRelPath), nil
	}

	return filepath.Join(pool.BasePath, filepath.FromSlash(cleanRelPath)), nil
}

// FileExists checks whether a file exists in the specified pool and returns its size.
func (sm *StorageManager) FileExists(relativePath string, poolID uint) (bool, int64, error) {
	pool, err := models.FindStoragePoolByID(sm.db, poolID)
	if err != nil {
		return false, 0, fmt.Errorf("failed to find storage pool %d: %w", poolID, err)
	}

	cleanRelPath, err := cleanRelativeStoragePath(relativePath)
	if err != nil {
		return false, 0, err
	}

	if pool.IsS3Storage() {
		s3Client, err := s3backup.NewPoolClient(pool)
		if err != nil {
			return false, 0, fmt.Errorf("failed to initialize S3 client for pool '%s': %w", pool.Name, err)
		}
		return s3Client.FileInfo(toS3ObjectKey(cleanRelPath))
	}

	fullPath := filepath.Join(pool.BasePath, filepath.FromSlash(cleanRelPath))
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, 0, nil
		}
		return false, 0, fmt.Errorf("failed to stat %s: %w", fullPath, err)
	}
	return true, info.Size(), nil
}

// UpdatePoolUsage updates the used size of a storage pool
func (sm *StorageManager) UpdatePoolUsage(poolID uint, sizeChange int64) error {
	pool, err := models.FindStoragePoolByID(sm.db, poolID)
	if err != nil {
		return fmt.Errorf("failed to find storage pool %d: %w", poolID, err)
	}

	return pool.UpdateUsedSize(sm.db, sizeChange)
}

func cleanRelativeStoragePath(raw string) (string, error) {
	candidate := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if candidate == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	clean := strings.TrimPrefix(path.Clean("/"+candidate), "/")
	if clean == "" || clean == "." {
		return "", fmt.Errorf("path cannot be empty")
	}
	if clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return "", fmt.Errorf("path traversal is not allowed")
	}
	return clean, nil
}

func toS3ObjectKey(relativePath string) string {
	clean := strings.TrimPrefix(path.Clean("/"+strings.ReplaceAll(relativePath, "\\", "/")), "/")
	if clean == "" || clean == "." {
		return "uploads"
	}
	if strings.HasPrefix(clean, "uploads/") {
		return clean
	}
	return path.Join("uploads", clean)
}
