package repository

import (
	"github.com/ManuelReschke/PixelFox/app/models"
	"gorm.io/gorm"
)

// storagePoolRepository implements the StoragePoolRepository interface
type storagePoolRepository struct {
	db *gorm.DB
}

// NewStoragePoolRepository creates a new storage pool repository instance
func NewStoragePoolRepository(db *gorm.DB) StoragePoolRepository {
	return &storagePoolRepository{db: db}
}

// Create creates a new storage pool in the database
func (r *storagePoolRepository) Create(pool *models.StoragePool) error {
	return r.db.Create(pool).Error
}

// GetByID retrieves a storage pool by its ID
func (r *storagePoolRepository) GetByID(id uint) (*models.StoragePool, error) {
	var pool models.StoragePool
	err := r.db.First(&pool, id).Error
	if err != nil {
		return nil, err
	}
	return &pool, nil
}

// GetAll retrieves all storage pools
func (r *storagePoolRepository) GetAll() ([]models.StoragePool, error) {
	var pools []models.StoragePool
	err := r.db.Order("priority ASC, created_at ASC").Find(&pools).Error
	return pools, err
}

// GetActive retrieves all active storage pools
func (r *storagePoolRepository) GetActive() ([]models.StoragePool, error) {
	var pools []models.StoragePool
	err := r.db.Where("is_active = ?", true).
		Order("priority ASC, created_at ASC").Find(&pools).Error
	return pools, err
}

// GetByTier retrieves storage pools by tier (hot, warm, cold, archive)
func (r *storagePoolRepository) GetByTier(tier string) ([]models.StoragePool, error) {
	var pools []models.StoragePool
	err := r.db.Where("storage_tier = ? AND is_active = ?", tier, true).
		Order("priority ASC, created_at ASC").Find(&pools).Error
	return pools, err
}

// GetOptimalForUpload finds the optimal storage pool for new uploads (prioritizes hot storage)
func (r *storagePoolRepository) GetOptimalForUpload(fileSize int64) (*models.StoragePool, error) {
	// First try to find a hot storage pool with enough space
	hotPools, err := r.GetByTier("hot")
	if err != nil {
		return nil, err
	}

	for _, pool := range hotPools {
		if pool.CanAcceptFile(fileSize) {
			return &pool, nil
		}
	}

	// If no hot storage available, fall back to any available pool
	return r.GetOptimalForFile(fileSize)
}

// GetOptimalForFile finds the optimal storage pool for a file based on available space
func (r *storagePoolRepository) GetOptimalForFile(fileSize int64) (*models.StoragePool, error) {
	var pool models.StoragePool

	// Find active pools that can accept the file, ordered by priority and available space
	err := r.db.Where("is_active = ? AND (capacity_bytes = 0 OR (capacity_bytes - used_bytes) >= ?)",
		true, fileSize).
		Order("priority ASC, (capacity_bytes - used_bytes) DESC").
		First(&pool).Error

	if err != nil {
		return nil, err
	}

	return &pool, nil
}

// Update updates an existing storage pool in the database
func (r *storagePoolRepository) Update(pool *models.StoragePool) error {
	return r.db.Save(pool).Error
}

// Delete soft deletes a storage pool by its ID
func (r *storagePoolRepository) Delete(id uint) error {
	return r.db.Delete(&models.StoragePool{}, id).Error
}

// UpdateUsage updates the used size of a storage pool
func (r *storagePoolRepository) UpdateUsage(id uint, sizeChange int64) error {
	if sizeChange == 0 {
		return nil
	}

	// Update the used_bytes field atomically
	return r.db.Model(&models.StoragePool{}).Where("id = ?", id).
		UpdateColumn("used_bytes", gorm.Expr("used_bytes + ?", sizeChange)).Error
}

// GetStats retrieves statistics for a specific storage pool
func (r *storagePoolRepository) GetStats(id uint) (*models.StoragePoolStats, error) {
	return models.GetStoragePoolStats(r.db, id)
}

// GetAllStats retrieves statistics for all storage pools
func (r *storagePoolRepository) GetAllStats() ([]models.StoragePoolStats, error) {
	return models.GetAllStoragePoolStats(r.db)
}
