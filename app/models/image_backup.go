package models

import (
	"time"

	"gorm.io/gorm"
)

// BackupProvider defines the supported backup providers
type BackupProvider string

const (
	BackupProviderS3    BackupProvider = "s3"
	BackupProviderGCS   BackupProvider = "gcs"
	BackupProviderAzure BackupProvider = "azure"
)

// BackupStatus defines the possible backup states
type BackupStatus string

const (
	BackupStatusPending   BackupStatus = "pending"
	BackupStatusUploading BackupStatus = "uploading"
	BackupStatusCompleted BackupStatus = "completed"
	BackupStatusFailed    BackupStatus = "failed"
	BackupStatusDeleted   BackupStatus = "deleted"
)

// ImageBackup represents a backup of an image to cloud storage
type ImageBackup struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	ImageID      uint           `gorm:"not null;index:idx_image_id" json:"image_id"`
	Image        Image          `gorm:"foreignKey:ImageID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"image,omitempty"`
	Provider     BackupProvider `gorm:"type:varchar(20);not null;default:'s3';index:idx_provider" json:"provider"`
	Status       BackupStatus   `gorm:"type:varchar(20);not null;default:'pending';index:idx_status" json:"status"`
	BucketName   string         `gorm:"type:varchar(100)" json:"bucket_name"`
	ObjectKey    string         `gorm:"type:varchar(500)" json:"object_key"`
	BackupSize   int64          `gorm:"type:bigint unsigned" json:"backup_size"`
	BackupDate   *time.Time     `json:"backup_date"`
	ErrorMessage string         `gorm:"type:text" json:"error_message"`
	RetryCount   int            `gorm:"type:int unsigned;default:0" json:"retry_count"`
	ClaimedBy    string         `gorm:"type:varchar(50);default:'';index:idx_claimed_by" json:"claimed_by"`
	ClaimedAt    *time.Time     `json:"claimed_at"`
	CreatedAt    time.Time      `gorm:"autoCreateTime;index:idx_created_at" json:"created_at"`
	UpdatedAt    time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName returns the table name for ImageBackup
func (ImageBackup) TableName() string {
	return "image_backups"
}

// BeforeCreate sets default values before creating a new backup record
func (ib *ImageBackup) BeforeCreate(tx *gorm.DB) error {
	if ib.Provider == "" {
		ib.Provider = BackupProviderS3
	}
	if ib.Status == "" {
		ib.Status = BackupStatusPending
	}
	return nil
}

// MarkAsUploading updates the backup status to uploading
func (ib *ImageBackup) MarkAsUploading(db *gorm.DB) error {
	ib.Status = BackupStatusUploading
	return db.Save(ib).Error
}

// ClaimForUploading attempts to atomically claim this backup for uploading by the given node.
// Returns true if the claim succeeded, false if not (another worker claimed or status not eligible).
func (ib *ImageBackup) ClaimForUploading(db *gorm.DB, nodeID string) (bool, error) {
	now := time.Now()
	// Atomic conditional update: only claim when status is pending or failed
	tx := db.Model(&ImageBackup{}).
		Where("id = ? AND status IN ?", ib.ID, []BackupStatus{BackupStatusPending, BackupStatusFailed}).
		Updates(map[string]interface{}{
			"status":     BackupStatusUploading,
			"claimed_by": nodeID,
			"claimed_at": now,
			"updated_at": now,
		})
	if tx.Error != nil {
		return false, tx.Error
	}
	if tx.RowsAffected == 0 {
		return false, nil
	}
	// Reflect changes on struct
	ib.Status = BackupStatusUploading
	ib.ClaimedBy = nodeID
	ib.ClaimedAt = &now
	return true, nil
}

// MarkAsCompleted updates the backup status to completed with metadata
func (ib *ImageBackup) MarkAsCompleted(db *gorm.DB, bucketName, objectKey string, size int64) error {
	now := time.Now()
	ib.Status = BackupStatusCompleted
	ib.BucketName = bucketName
	ib.ObjectKey = objectKey
	ib.BackupSize = size
	ib.BackupDate = &now
	ib.ErrorMessage = "" // Clear any previous error
	ib.ClaimedBy = ""
	ib.ClaimedAt = nil
	return db.Save(ib).Error
}

// MarkAsFailed updates the backup status to failed with error message
func (ib *ImageBackup) MarkAsFailed(db *gorm.DB, errorMsg string) error {
	ib.Status = BackupStatusFailed
	ib.ErrorMessage = errorMsg
	ib.RetryCount++
	ib.ClaimedBy = ""
	ib.ClaimedAt = nil
	return db.Save(ib).Error
}

// MarkAsDeleted updates the backup status to deleted
func (ib *ImageBackup) MarkAsDeleted(db *gorm.DB, message string) error {
	ib.Status = BackupStatusDeleted
	ib.ErrorMessage = message
	ib.ClaimedBy = ""
	ib.ClaimedAt = nil
	return db.Save(ib).Error
}

// IsRetryable checks if the backup can be retried (max 3 retries)
func (ib *ImageBackup) IsRetryable() bool {
	return ib.Status == BackupStatusFailed && ib.RetryCount < 3
}

// FindBackupByImageAndProvider finds a backup record by image ID and provider
func FindBackupByImageAndProvider(db *gorm.DB, imageID uint, provider BackupProvider) (*ImageBackup, error) {
	var backup ImageBackup
	err := db.Where("image_id = ? AND provider = ?", imageID, provider).First(&backup).Error
	return &backup, err
}

// FindBackupsByStatus finds all backup records by status
func FindBackupsByStatus(db *gorm.DB, status BackupStatus) ([]ImageBackup, error) {
	var backups []ImageBackup
	err := db.Preload("Image").Where("status = ?", status).Find(&backups).Error
	return backups, err
}

// FindPendingBackups finds all pending backup records
func FindPendingBackups(db *gorm.DB) ([]ImageBackup, error) {
	return FindBackupsByStatus(db, BackupStatusPending)
}

// FindFailedRetryableBackups finds all failed backups that can be retried
func FindFailedRetryableBackups(db *gorm.DB) ([]ImageBackup, error) {
	var backups []ImageBackup
	err := db.Preload("Image").Where("status = ? AND retry_count < ?", BackupStatusFailed, 3).Find(&backups).Error
	return backups, err
}

// FindStuckUploadingBackups returns backups that have been in 'uploading' longer than the given duration
func FindStuckUploadingBackups(db *gorm.DB, olderThan time.Duration) ([]ImageBackup, error) {
	var backups []ImageBackup
	cutoff := time.Now().Add(-olderThan)
	err := db.Preload("Image").Where("status = ? AND claimed_at IS NOT NULL AND claimed_at <= ?", BackupStatusUploading, cutoff).Find(&backups).Error
	return backups, err
}

// CountBackupsByStatus returns the count of backups by status
func CountBackupsByStatus(db *gorm.DB, status BackupStatus) (int64, error) {
	var count int64
	err := db.Model(&ImageBackup{}).Where("status = ?", status).Count(&count).Error
	return count, err
}

// GetBackupStats returns statistics about backup status
func GetBackupStats(db *gorm.DB) (map[BackupStatus]int64, error) {
	stats := make(map[BackupStatus]int64)

	statuses := []BackupStatus{
		BackupStatusPending,
		BackupStatusUploading,
		BackupStatusCompleted,
		BackupStatusFailed,
		BackupStatusDeleted,
	}

	for _, status := range statuses {
		count, err := CountBackupsByStatus(db, status)
		if err != nil {
			return nil, err
		}
		stats[status] = count
	}

	return stats, nil
}

// CreateBackupRecord creates a new backup record for an image
func CreateBackupRecord(db *gorm.DB, imageID uint, provider BackupProvider) (*ImageBackup, error) {
	backup := &ImageBackup{
		ImageID:  imageID,
		Provider: provider,
		Status:   BackupStatusPending,
	}

	err := db.Create(backup).Error
	return backup, err
}

// FindCompletedBackupsByImageID finds all completed backup records for an image
func FindCompletedBackupsByImageID(db *gorm.DB, imageID uint) ([]ImageBackup, error) {
	var backups []ImageBackup
	err := db.Where("image_id = ? AND status = ?", imageID, BackupStatusCompleted).Find(&backups).Error
	return backups, err
}
