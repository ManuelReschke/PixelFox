package models

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// Storage type constants
const (
	StorageTypeLocal = "local" // Local filesystem storage
	StorageTypeNFS   = "nfs"   // Network File System storage
	StorageTypeS3    = "s3"    // S3-compatible storage (AWS S3, Backblaze B2, MinIO, etc.)
)

// StoragePool represents a storage location for images and variants
type StoragePool struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	Name        string `gorm:"type:varchar(100);not null;uniqueIndex" json:"name"`
	BasePath    string `gorm:"type:varchar(500);not null" json:"base_path"`
	MaxSize     int64  `gorm:"type:bigint;not null" json:"max_size"`                                                                                             // Maximum size in bytes
	UsedSize    int64  `gorm:"type:bigint;default:0" json:"used_size"`                                                                                           // Currently used size in bytes
	IsActive    bool   `gorm:"default:true" json:"is_active"`                                                                                                    // Whether this pool is available for new files
	IsDefault   bool   `gorm:"default:false" json:"is_default"`                                                                                                  // Whether this is the default fallback pool
	Priority    int    `gorm:"default:100" json:"priority"`                                                                                                      // Lower number = higher priority
	StorageType string `gorm:"type:varchar(50);default:'local'" json:"storage_type"`                                                                             // local, nfs, s3, etc.
	StorageTier string `gorm:"type:varchar(20);default:'hot';index:idx_storage_tier;index:idx_tier_active,composite:storage_tier,is_active" json:"storage_tier"` // hot, warm, cold, archive
	Description string `gorm:"type:text" json:"description"`                                                                                                     // Optional description

	// S3-specific configuration fields (only used when StorageType = 's3')
	// Note: These credentials should be encrypted at rest in production
	S3AccessKeyID     *string `gorm:"type:varchar(255)" json:"s3_access_key_id,omitempty"`          // S3 Access Key ID (nullable for security)
	S3SecretAccessKey *string `gorm:"type:varchar(500)" json:"-"`                                   // S3 Secret Key (excluded from JSON for security)
	S3Region          *string `gorm:"type:varchar(100)" json:"s3_region,omitempty"`                 // S3 Region (e.g., us-west-2, us-west-001 for Backblaze B2)
	S3BucketName      *string `gorm:"type:varchar(255)" json:"s3_bucket_name,omitempty"`            // S3 Bucket name
	S3EndpointURL     *string `gorm:"type:varchar(500)" json:"s3_endpoint_url,omitempty"`           // S3 Endpoint URL (for S3-compatible services like Backblaze B2, MinIO)
	S3PathPrefix      *string `gorm:"type:varchar(500);default:''" json:"s3_path_prefix,omitempty"` // Optional path prefix within bucket for organizing files

	// Node-aware multi-VPS fields
	PublicBaseURL string `gorm:"type:varchar(500);default:''" json:"public_base_url,omitempty"` // Public base URL for serving files, e.g. https://s01.pixelfox.cc
	UploadAPIURL  string `gorm:"type:varchar(500);default:''" json:"upload_api_url,omitempty"`  // Internal/public upload API endpoint for direct-to-storage
	NodeID        string `gorm:"type:varchar(100);default:'';index" json:"node_id,omitempty"`   // Logical node identifier, e.g. s01

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
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
	// Validate storage type
	if err := sp.ValidateStorageType(); err != nil {
		return err
	}

	// Validate storage-specific configuration
	if err := sp.ValidateStorageConfiguration(); err != nil {
		return err
	}

	// Validate base path for local/nfs storage types
	if sp.StorageType == StorageTypeLocal || sp.StorageType == StorageTypeNFS {
		if err := sp.ValidateBasePath(); err != nil {
			return err
		}
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
	// Validate storage type
	if err := sp.ValidateStorageType(); err != nil {
		return err
	}

	// Validate storage-specific configuration
	if err := sp.ValidateStorageConfiguration(); err != nil {
		return err
	}

	// Validate base path for local/nfs storage types
	if sp.StorageType == StorageTypeLocal || sp.StorageType == StorageTypeNFS {
		if err := sp.ValidateBasePath(); err != nil {
			return err
		}
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

// ValidateStorageType validates that the storage type is supported
func (sp *StoragePool) ValidateStorageType() error {
	switch sp.StorageType {
	case StorageTypeLocal, StorageTypeNFS, StorageTypeS3:
		return nil
	default:
		return fmt.Errorf("unsupported storage type: %s. Supported types: %s, %s, %s",
			sp.StorageType, StorageTypeLocal, StorageTypeNFS, StorageTypeS3)
	}
}

// ValidateStorageConfiguration validates storage-specific configuration
func (sp *StoragePool) ValidateStorageConfiguration() error {
	switch sp.StorageType {
	case StorageTypeS3:
		return sp.ValidateS3Configuration()
	case StorageTypeLocal, StorageTypeNFS:
		// For local/nfs, base path validation is handled by ValidateBasePath
		if sp.BasePath == "" {
			return errors.New("base path is required for local and NFS storage types")
		}
		return nil
	default:
		return fmt.Errorf("unknown storage type: %s", sp.StorageType)
	}
}

// ValidateS3Configuration validates S3-specific configuration
func (sp *StoragePool) ValidateS3Configuration() error {
	if sp.S3AccessKeyID == nil || strings.TrimSpace(*sp.S3AccessKeyID) == "" {
		return errors.New("S3 Access Key ID is required for S3 storage type")
	}

	if sp.S3SecretAccessKey == nil || strings.TrimSpace(*sp.S3SecretAccessKey) == "" {
		return errors.New("S3 Secret Access Key is required for S3 storage type")
	}

	if sp.S3BucketName == nil || strings.TrimSpace(*sp.S3BucketName) == "" {
		return errors.New("S3 Bucket Name is required for S3 storage type")
	}

	if sp.S3Region == nil || strings.TrimSpace(*sp.S3Region) == "" {
		return errors.New("S3 Region is required for S3 storage type")
	}

	// Validate bucket name format (basic validation)
	bucketName := strings.TrimSpace(*sp.S3BucketName)
	if len(bucketName) < 3 || len(bucketName) > 63 {
		return errors.New("S3 bucket name must be between 3 and 63 characters")
	}

	// Validate region format (basic validation)
	region := strings.TrimSpace(*sp.S3Region)
	if len(region) < 2 {
		return errors.New("S3 region must be at least 2 characters")
	}

	return nil
}

// IsS3Storage checks if this pool uses S3 storage
func (sp *StoragePool) IsS3Storage() bool {
	return sp.StorageType == StorageTypeS3
}

// GetS3AccessKeyID safely returns the S3 Access Key ID
func (sp *StoragePool) GetS3AccessKeyID() string {
	if sp.S3AccessKeyID != nil {
		return *sp.S3AccessKeyID
	}
	return ""
}

// GetS3SecretAccessKey safely returns the S3 Secret Access Key
func (sp *StoragePool) GetS3SecretAccessKey() string {
	if sp.S3SecretAccessKey != nil {
		return *sp.S3SecretAccessKey
	}
	return ""
}

// GetS3Region safely returns the S3 region
func (sp *StoragePool) GetS3Region() string {
	if sp.S3Region != nil {
		return *sp.S3Region
	}
	return ""
}

// GetS3BucketName safely returns the S3 bucket name
func (sp *StoragePool) GetS3BucketName() string {
	if sp.S3BucketName != nil {
		return *sp.S3BucketName
	}
	return ""
}

// GetS3EndpointURL safely returns the S3 endpoint URL
func (sp *StoragePool) GetS3EndpointURL() string {
	if sp.S3EndpointURL != nil {
		return *sp.S3EndpointURL
	}
	return ""
}

// GetS3PathPrefix safely returns the S3 path prefix
func (sp *StoragePool) GetS3PathPrefix() string {
	if sp.S3PathPrefix != nil {
		return *sp.S3PathPrefix
	}
	return ""
}

// SetS3Credentials sets S3 credentials (helper method for safe credential handling)
func (sp *StoragePool) SetS3Credentials(accessKeyID, secretAccessKey string) {
	accessKey := strings.TrimSpace(accessKeyID)
	secretKey := strings.TrimSpace(secretAccessKey)

	if accessKey != "" {
		sp.S3AccessKeyID = &accessKey
	}
	if secretKey != "" {
		sp.S3SecretAccessKey = &secretKey
	}
}

// SetS3Configuration sets S3 configuration (helper method)
func (sp *StoragePool) SetS3Configuration(region, bucketName, endpointURL, pathPrefix string) {
	regionTrimmed := strings.TrimSpace(region)
	bucketTrimmed := strings.TrimSpace(bucketName)
	endpointTrimmed := strings.TrimSpace(endpointURL)
	pathTrimmed := strings.TrimSpace(pathPrefix)

	if regionTrimmed != "" {
		sp.S3Region = &regionTrimmed
	}
	if bucketTrimmed != "" {
		sp.S3BucketName = &bucketTrimmed
	}
	if endpointTrimmed != "" {
		sp.S3EndpointURL = &endpointTrimmed
	}
	if pathTrimmed != "" {
		sp.S3PathPrefix = &pathTrimmed
	}
}

// HasS3Credentials checks if S3 credentials are configured
func (sp *StoragePool) HasS3Credentials() bool {
	return sp.S3AccessKeyID != nil && sp.S3SecretAccessKey != nil &&
		strings.TrimSpace(*sp.S3AccessKeyID) != "" &&
		strings.TrimSpace(*sp.S3SecretAccessKey) != ""
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
	// Validate storage type
	if err := sp.ValidateStorageType(); err != nil {
		log.Errorf("[StoragePool] Health check failed for pool %s: invalid storage type: %v", sp.Name, err)
		return false
	}

	// Validate storage-specific configuration
	if err := sp.ValidateStorageConfiguration(); err != nil {
		log.Errorf("[StoragePool] Health check failed for pool %s: invalid configuration: %v", sp.Name, err)
		return false
	}

	// For local/nfs storage, check if path exists and is writable
	if sp.StorageType == StorageTypeLocal || sp.StorageType == StorageTypeNFS {
		if err := sp.ValidateBasePath(); err != nil {
			log.Errorf("[StoragePool] Health check failed for pool %s: %v", sp.Name, err)
			return false
		}
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

// FindActiveStoragePoolsByType returns active storage pools filtered by storage type
func FindActiveStoragePoolsByType(db *gorm.DB, storageType string) ([]StoragePool, error) {
	var pools []StoragePool
	result := db.Where("is_active = ? AND storage_type = ?", true, storageType).
		Order("priority ASC, id ASC").
		Find(&pools)
	return pools, result.Error
}

// FindS3StoragePools returns all active S3 storage pools
func FindS3StoragePools(db *gorm.DB) ([]StoragePool, error) {
	return FindActiveStoragePoolsByType(db, StorageTypeS3)
}

// FindHighestPriorityS3Pool returns the S3 storage pool with the highest priority (lowest number)
// Returns nil if no active S3 storage pools are found
func FindHighestPriorityS3Pool(db *gorm.DB) (*StoragePool, error) {
	var pool StoragePool
	err := db.Where("storage_type = ? AND is_active = ?", StorageTypeS3, true).
		Order("priority ASC"). // Lower number = higher priority
		First(&pool).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // No S3 pools found, but not an error
		}
		return nil, fmt.Errorf("failed to find S3 storage pool: %w", err)
	}

	return &pool, nil
}

// FindLocalStoragePools returns all active local storage pools
func FindLocalStoragePools(db *gorm.DB) ([]StoragePool, error) {
	return FindActiveStoragePoolsByType(db, StorageTypeLocal)
}

// FindNFSStoragePools returns all active NFS storage pools
func FindNFSStoragePools(db *gorm.DB) ([]StoragePool, error) {
	return FindActiveStoragePoolsByType(db, StorageTypeNFS)
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

	var imageCount int64
	var variantCount int64
	var usedSize int64

	if pool.StorageType == StorageTypeS3 {
		// For S3 pools we count objects stored directly via pool assignment.
		var directImageCount int64
		db.Model(&Image{}).Where("storage_pool_id = ?", poolID).Count(&directImageCount)
		imageCount = directImageCount

		var directVariantCount int64
		db.Model(&ImageVariant{}).Where("storage_pool_id = ?", poolID).Count(&directVariantCount)
		variantCount = directVariantCount

		var directUsedSize int64
		db.Model(&Image{}).Where("storage_pool_id = ?", poolID).Select("COALESCE(SUM(file_size), 0)").Scan(&directUsedSize)
		usedSize = directUsedSize

		var variantUsedSize int64
		db.Model(&ImageVariant{}).Where("storage_pool_id = ?", poolID).Select("COALESCE(SUM(file_size), 0)").Scan(&variantUsedSize)
		usedSize += variantUsedSize

	} else {
		// For local/NFS storage pools, count images and variants normally
		db.Model(&Image{}).Where("storage_pool_id = ?", poolID).Count(&imageCount)
		db.Model(&ImageVariant{}).Where("storage_pool_id = ?", poolID).Count(&variantCount)

		// Calculate used size from actual files (COALESCE to avoid NULL → scan errors)
		db.Model(&Image{}).Where("storage_pool_id = ?", poolID).Select("COALESCE(SUM(file_size), 0)").Scan(&usedSize)
		var variantUsedSize int64
		db.Model(&ImageVariant{}).Where("storage_pool_id = ?", poolID).Select("COALESCE(SUM(file_size), 0)").Scan(&variantUsedSize)
		usedSize += variantUsedSize
	}

	// Update only the used_size to avoid overwriting other fields with stale values
	pool.UsedSize = usedSize
	db.Model(&StoragePool{}).Where("id = ?", pool.ID).UpdateColumn("used_size", usedSize)

	stats := &StoragePoolStats{
		ID:              pool.ID,
		Name:            pool.Name,
		UsedSize:        usedSize,
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
