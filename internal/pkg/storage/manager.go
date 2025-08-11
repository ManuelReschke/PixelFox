package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/gofiber/fiber/v2/log"
	"gorm.io/gorm"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
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

	// Create full file path
	fullPath := filepath.Join(pool.BasePath, filename)
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
	bytesWritten, err := io.Copy(file, data)
	if err != nil {
		operation.Error = fmt.Errorf("failed to write file %s: %w", fullPath, err)
		operation.Duration = time.Since(startTime)
		// Clean up partial file
		os.Remove(fullPath)
		return operation, operation.Error
	}

	// Update pool usage
	if err := pool.UpdateUsedSize(sm.db, bytesWritten); err != nil {
		log.Errorf("[StorageManager] Failed to update pool usage for %s: %v", pool.Name, err)
		// Don't fail the operation for this
	}

	operation.Success = true
	operation.Duration = time.Since(startTime)

	log.Infof("[StorageManager] Successfully saved file %s (%d bytes) to pool '%s' in %v",
		filename, bytesWritten, pool.Name, operation.Duration)

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

	// Create full file path
	fullPath := filepath.Join(pool.BasePath, relativePath)
	operation.FilePath = fullPath

	// Get file size before deletion for usage tracking
	fileInfo, err := os.Stat(fullPath)
	fileSize := int64(0)
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
		relativePath, fileSize, pool.Name, operation.Duration)

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

	// Check target pool health and capacity
	if !targetPool.IsHealthy() {
		operation.Error = fmt.Errorf("target storage pool '%s' is not healthy", targetPool.Name)
		operation.Duration = time.Since(startTime)
		return operation, operation.Error
	}

	// Create file paths
	sourcePath := filepath.Join(sourcePool.BasePath, relativePath)
	targetPath := filepath.Join(targetPool.BasePath, relativePath)
	operation.FilePath = targetPath

	// Get file size for capacity check
	fileInfo, err := os.Stat(sourcePath)
	if err != nil {
		operation.Error = fmt.Errorf("failed to stat source file %s: %w", sourcePath, err)
		operation.Duration = time.Since(startTime)
		return operation, operation.Error
	}

	fileSize := fileInfo.Size()

	// Check if target pool can accept the file
	if !targetPool.CanAcceptFile(fileSize) {
		operation.Error = fmt.Errorf("target pool '%s' cannot accept file of size %d bytes", targetPool.Name, fileSize)
		operation.Duration = time.Since(startTime)
		return operation, operation.Error
	}

	// Create target directory
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		operation.Error = fmt.Errorf("failed to create target directory %s: %w", targetDir, err)
		operation.Duration = time.Since(startTime)
		return operation, operation.Error
	}

	// Copy file (safer than move for cross-filesystem operations)
	if err := sm.copyFile(sourcePath, targetPath); err != nil {
		operation.Error = fmt.Errorf("failed to copy file from %s to %s: %w", sourcePath, targetPath, err)
		operation.Duration = time.Since(startTime)
		return operation, operation.Error
	}

	// Update pool usage
	if err := sourcePool.UpdateUsedSize(sm.db, -fileSize); err != nil {
		log.Errorf("[StorageManager] Failed to update source pool usage: %v", err)
	}

	if err := targetPool.UpdateUsedSize(sm.db, fileSize); err != nil {
		log.Errorf("[StorageManager] Failed to update target pool usage: %v", err)
	}

	// Delete source file after successful copy
	if err := os.Remove(sourcePath); err != nil {
		log.Errorf("[StorageManager] Failed to remove source file %s after migration: %v", sourcePath, err)
		// Don't fail the operation for this
	}

	operation.Success = true
	operation.Duration = time.Since(startTime)

	log.Infof("[StorageManager] Successfully migrated file %s (%d bytes) from pool '%s' to pool '%s' in %v",
		relativePath, fileSize, sourcePool.Name, targetPool.Name, operation.Duration)

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

	return filepath.Join(pool.BasePath, relativePath), nil
}

// UpdatePoolUsage updates the used size of a storage pool
func (sm *StorageManager) UpdatePoolUsage(poolID uint, sizeChange int64) error {
	pool, err := models.FindStoragePoolByID(sm.db, poolID)
	if err != nil {
		return fmt.Errorf("failed to find storage pool %d: %w", poolID, err)
	}

	return pool.UpdateUsedSize(sm.db, sizeChange)
}
