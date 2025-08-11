package models

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofiber/fiber/v2/log"
	"gorm.io/gorm"
)

// Storage tier constants
const (
	StorageTierHot     = "hot"     // High-performance storage (SSD, fast access)
	StorageTierWarm    = "warm"    // Medium performance storage
	StorageTierCold    = "cold"    // Archive storage (HDD, slower access)
	StorageTierArchive = "archive" // Long-term archive (tape, very slow)
)

// StoragePool represents a storage location for images and variants
type StoragePool struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"type:varchar(100);not null;uniqueIndex" json:"name"`
	BasePath    string    `gorm:"type:varchar(500);not null" json:"base_path"`
	MaxSize     int64     `gorm:"type:bigint;not null" json:"max_size"`                                                                                             // Maximum size in bytes
	UsedSize    int64     `gorm:"type:bigint;default:0" json:"used_size"`                                                                                           // Currently used size in bytes
	IsActive    bool      `gorm:"default:true" json:"is_active"`                                                                                                    // Whether this pool is available for new files
	IsDefault   bool      `gorm:"default:false" json:"is_default"`                                                                                                  // Whether this is the default fallback pool
	Priority    int       `gorm:"default:100" json:"priority"`                                                                                                      // Lower number = higher priority
	StorageType string    `gorm:"type:varchar(50);default:'local'" json:"storage_type"`                                                                             // local, nfs, s3, etc.
	StorageTier string    `gorm:"type:varchar(20);default:'hot';index:idx_storage_tier;index:idx_tier_active,composite:storage_tier,is_active" json:"storage_tier"` // hot, warm, cold, archive
	Description string    `gorm:"type:text" json:"description"`                                                                                                     // Optional description
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// StoragePoolStats represents statistics for a storage pool
type StoragePoolStats struct {
	ID              uint      `json:"id"`
	Name            string    `json:"name"`
	UsedSize        int64     `json:"used_size"`
	MaxSize         int64     `json:"max_size"`
	AvailableSize   int64     `json:"available_size"`
	UsagePercentage float64   `json:"usage_percentage"`
	ImageCount      int64     `json:"image_count"`
	VariantCount    int64     `json:"variant_count"`
	IsHealthy       bool      `json:"is_healthy"`
	LastHealthCheck time.Time `json:"last_health_check"`
}

// BeforeCreate validates the storage pool before creation
func (sp *StoragePool) BeforeCreate(tx *gorm.DB) error {
	// Validate base path
	if err := sp.ValidateBasePath(); err != nil {
		return err
	}

	// Ensure only one default pool exists
	if sp.IsDefault {
		var count int64
		if err := tx.Model(&StoragePool{}).Where("is_default = ? AND id != ?", true, sp.ID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New("only one default storage pool is allowed")
		}
	}

	return nil
}

// BeforeUpdate validates the storage pool before update
func (sp *StoragePool) BeforeUpdate(tx *gorm.DB) error {
	// Validate base path if changed
	if err := sp.ValidateBasePath(); err != nil {
		return err
	}

	// Ensure only one default pool exists
	if sp.IsDefault {
		var count int64
		if err := tx.Model(&StoragePool{}).Where("is_default = ? AND id != ?", true, sp.ID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New("only one default storage pool is allowed")
		}
	}

	return nil
}

// ValidateBasePath checks if the base path is valid and writable
func (sp *StoragePool) ValidateBasePath() error {
	if sp.BasePath == "" {
		return errors.New("base path cannot be empty")
	}

	// Check if path is absolute
	if !filepath.IsAbs(sp.BasePath) {
		return errors.New("base path must be absolute")
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(sp.BasePath, 0755); err != nil {
		return fmt.Errorf("failed to create base path directory: %w", err)
	}

	// Test if directory is writable
	testFile := filepath.Join(sp.BasePath, ".pixelfox_write_test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return fmt.Errorf("base path is not writable: %w", err)
	}
	os.Remove(testFile) // Clean up test file

	return nil
}

// GetAvailableSize returns the available space in this pool
func (sp *StoragePool) GetAvailableSize() int64 {
	return sp.MaxSize - sp.UsedSize
}

// GetUsagePercentage returns the usage percentage of this pool
func (sp *StoragePool) GetUsagePercentage() float64 {
	if sp.MaxSize == 0 {
		return 0
	}
	return (float64(sp.UsedSize) / float64(sp.MaxSize)) * 100
}

// IsHealthy checks if the storage pool is healthy
func (sp *StoragePool) IsHealthy() bool {
	// Check if path exists and is writable
	if err := sp.ValidateBasePath(); err != nil {
		log.Errorf("[StoragePool] Health check failed for pool %s: %v", sp.Name, err)
		return false
	}

	// Check if pool is not over capacity
	if sp.UsedSize > sp.MaxSize {
		log.Warnf("[StoragePool] Pool %s is over capacity: %d/%d bytes", sp.Name, sp.UsedSize, sp.MaxSize)
		return false
	}

	return true
}

// CanAcceptFile checks if this pool can accept a file of given size
func (sp *StoragePool) CanAcceptFile(size int64) bool {
	if !sp.IsActive {
		return false
	}

	return sp.GetAvailableSize() >= size
}

// UpdateUsedSize updates the used size of the pool
func (sp *StoragePool) UpdateUsedSize(db *gorm.DB, sizeDelta int64) error {
	return db.Model(sp).UpdateColumn("used_size", gorm.Expr("used_size + ?", sizeDelta)).Error
}

// --- Static Functions ---

// FindStoragePoolByID finds a storage pool by ID
func FindStoragePoolByID(db *gorm.DB, id uint) (*StoragePool, error) {
	var pool StoragePool
	result := db.Where("id = ?", id).First(&pool)
	return &pool, result.Error
}

// FindStoragePoolByName finds a storage pool by name
func FindStoragePoolByName(db *gorm.DB, name string) (*StoragePool, error) {
	var pool StoragePool
	result := db.Where("name = ?", name).First(&pool)
	return &pool, result.Error
}

// FindDefaultStoragePool finds the default storage pool
func FindDefaultStoragePool(db *gorm.DB) (*StoragePool, error) {
	var pool StoragePool
	result := db.Where("is_default = ?", true).First(&pool)
	return &pool, result.Error
}

// FindActiveStoragePools returns all active storage pools ordered by priority
func FindActiveStoragePools(db *gorm.DB) ([]StoragePool, error) {
	var pools []StoragePool
	result := db.Where("is_active = ?", true).Order("priority ASC, id ASC").Find(&pools)
	return pools, result.Error
}

// FindAllStoragePools returns all storage pools
func FindAllStoragePools(db *gorm.DB) ([]StoragePool, error) {
	var pools []StoragePool
	result := db.Order("priority ASC, id ASC").Find(&pools)
	return pools, result.Error
}

// SelectOptimalPool selects the best storage pool for a file of given size
// DEPRECATED: Use SelectOptimalPoolForUpload for new uploads with tier-aware selection
func SelectOptimalPool(db *gorm.DB, fileSize int64) (*StoragePool, error) {
	// Get all active pools ordered by priority
	pools, err := FindActiveStoragePools(db)
	if err != nil {
		return nil, fmt.Errorf("failed to get active storage pools: %w", err)
	}

	if len(pools) == 0 {
		return nil, errors.New("no active storage pools available")
	}

	// Find the best pool based on priority and available space
	for _, pool := range pools {
		if pool.CanAcceptFile(fileSize) {
			log.Debugf("[StoragePool] Selected pool %s for file size %d bytes", pool.Name, fileSize)
			return &pool, nil
		}
	}

	// If no pool has enough space, try the default pool as fallback
	defaultPool, err := FindDefaultStoragePool(db)
	if err != nil {
		return nil, fmt.Errorf("no pools can accept file of size %d bytes and no default pool found: %w", fileSize, err)
	}

	if defaultPool.IsActive {
		log.Warnf("[StoragePool] Using default pool %s for oversized file (%d bytes)", defaultPool.Name, fileSize)
		return defaultPool, nil
	}

	return nil, fmt.Errorf("no storage pools can accept file of size %d bytes", fileSize)
}

// SelectOptimalPoolForUpload selects the best hot storage pool for new uploads
// All new uploads should go to hot storage first for optimal performance
func SelectOptimalPoolForUpload(db *gorm.DB, fileSize int64) (*StoragePool, error) {
	// First, try to find hot storage pools
	hotPools, err := FindActiveStoragePoolsByTier(db, StorageTierHot)
	if err != nil {
		log.Errorf("[StoragePool] Failed to get hot storage pools: %v", err)
	} else {
		// Find the best hot pool with enough space
		for _, pool := range hotPools {
			if pool.CanAcceptFile(fileSize) {
				log.Infof("[StoragePool] Selected HOT storage pool %s for upload (%d bytes)", pool.Name, fileSize)
				return &pool, nil
			}
		}
		log.Warnf("[StoragePool] No hot storage pools have sufficient space for file size %d bytes", fileSize)
	}

	// Fallback: try warm storage
	warmPools, err := FindActiveStoragePoolsByTier(db, StorageTierWarm)
	if err != nil {
		log.Errorf("[StoragePool] Failed to get warm storage pools: %v", err)
	} else {
		for _, pool := range warmPools {
			if pool.CanAcceptFile(fileSize) {
				log.Warnf("[StoragePool] Using WARM storage pool %s for upload (hot storage full) (%d bytes)", pool.Name, fileSize)
				return &pool, nil
			}
		}
	}

	// Final fallback: use any available storage
	return SelectOptimalPool(db, fileSize)
}

// FindActiveStoragePoolsByTier returns active storage pools filtered by tier
func FindActiveStoragePoolsByTier(db *gorm.DB, tier string) ([]StoragePool, error) {
	var pools []StoragePool
	result := db.Where("is_active = ? AND storage_tier = ?", true, tier).
		Order("priority ASC, id ASC").
		Find(&pools)
	return pools, result.Error
}

// FindHotStoragePools returns all active hot storage pools
func FindHotStoragePools(db *gorm.DB) ([]StoragePool, error) {
	return FindActiveStoragePoolsByTier(db, StorageTierHot)
}

// FindColdStoragePools returns all active cold storage pools
func FindColdStoragePools(db *gorm.DB) ([]StoragePool, error) {
	return FindActiveStoragePoolsByTier(db, StorageTierCold)
}

// IsHotStorage checks if this pool is hot storage
func (sp *StoragePool) IsHotStorage() bool {
	return sp.StorageTier == StorageTierHot
}

// IsColdStorage checks if this pool is cold storage
func (sp *StoragePool) IsColdStorage() bool {
	return sp.StorageTier == StorageTierCold || sp.StorageTier == StorageTierArchive
}

// GetStoragePoolStats returns statistics for a storage pool
func GetStoragePoolStats(db *gorm.DB, poolID uint) (*StoragePoolStats, error) {
	pool, err := FindStoragePoolByID(db, poolID)
	if err != nil {
		return nil, err
	}

	// Count images in this pool
	var imageCount int64
	db.Model(&Image{}).Where("storage_pool_id = ?", poolID).Count(&imageCount)

	// Count variants in this pool
	var variantCount int64
	db.Model(&ImageVariant{}).Where("storage_pool_id = ?", poolID).Count(&variantCount)

	stats := &StoragePoolStats{
		ID:              pool.ID,
		Name:            pool.Name,
		UsedSize:        pool.UsedSize,
		MaxSize:         pool.MaxSize,
		AvailableSize:   pool.GetAvailableSize(),
		UsagePercentage: pool.GetUsagePercentage(),
		ImageCount:      imageCount,
		VariantCount:    variantCount,
		IsHealthy:       pool.IsHealthy(),
		LastHealthCheck: time.Now(),
	}

	return stats, nil
}

// GetAllStoragePoolStats returns statistics for all storage pools
func GetAllStoragePoolStats(db *gorm.DB) ([]StoragePoolStats, error) {
	pools, err := FindAllStoragePools(db)
	if err != nil {
		return nil, err
	}

	stats := make([]StoragePoolStats, len(pools))
	for i, pool := range pools {
		poolStats, err := GetStoragePoolStats(db, pool.ID)
		if err != nil {
			log.Errorf("[StoragePool] Failed to get stats for pool %s: %v", pool.Name, err)
			continue
		}
		stats[i] = *poolStats
	}

	return stats, nil
}
