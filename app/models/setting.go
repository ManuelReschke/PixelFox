package models

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

// Setting represents a system setting
type Setting struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Key       string    `gorm:"column:setting_key;size:255;not null;uniqueIndex" json:"key" validate:"required,min=1,max=255"`
	Value     string    `gorm:"type:text" json:"value"`
	Type      string    `gorm:"size:50;not null" json:"type" validate:"required"` // string, boolean, integer, float
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AppSettings represents the application settings structure
type AppSettings struct {
	SiteTitle                    string `json:"site_title" validate:"required,min=1,max=255"`
	SiteDescription              string `json:"site_description" validate:"max=500"`
	ImageUploadEnabled           bool   `json:"image_upload_enabled"`
	DirectUploadEnabled          bool   `json:"direct_upload_enabled"`
	UploadRateLimitPerMinute     int    `json:"upload_rate_limit_per_minute" validate:"min=0,max=100000"`
	UploadUserRateLimitPerMinute int    `json:"upload_user_rate_limit_per_minute" validate:"min=0,max=100000"`
	// Thumbnail format settings
	ThumbnailOriginalEnabled bool `json:"thumbnail_original_enabled"`
	ThumbnailWebPEnabled     bool `json:"thumbnail_webp_enabled"`
	ThumbnailAVIFEnabled     bool `json:"thumbnail_avif_enabled"`
	// S3 Backup settings
	S3BackupEnabled       bool `json:"s3_backup_enabled"`
	S3BackupDelayMinutes  int  `json:"s3_backup_delay_minutes" validate:"min=0,max=43200"` // Max 30 days (43200 minutes)
	S3BackupCheckInterval int  `json:"s3_backup_check_interval" validate:"min=1,max=60"`   // How often to check for delayed backups (1-60 minutes)
	S3RetryInterval       int  `json:"s3_retry_interval" validate:"min=1,max=60"`          // How often to retry failed backups (1-60 minutes)
	JobQueueWorkerCount   int  `json:"job_queue_worker_count" validate:"min=1,max=20"`     // Number of job queue workers (1-20)
	// Replication/Storage settings
	ReplicationRequireChecksum bool `json:"replication_require_checksum"`
	mu                         sync.RWMutex
}

// Global settings instance
var (
	appSettings *AppSettings
	settingsMu  sync.RWMutex
)

// GetAppSettings returns the current application settings
func GetAppSettings() *AppSettings {
	settingsMu.RLock()
	defer settingsMu.RUnlock()
	return appSettings
}

// LoadSettings loads settings from database into memory
func LoadSettings(db *gorm.DB) error {
	settingsMu.Lock()
	defer settingsMu.Unlock()

	// Initialize with defaults
	appSettings = &AppSettings{
		SiteTitle:                    "PixelFox",
		SiteDescription:              "Image sharing platform",
		ImageUploadEnabled:           true,
		DirectUploadEnabled:          false,
		UploadRateLimitPerMinute:     60,
		UploadUserRateLimitPerMinute: 60,
		ThumbnailOriginalEnabled:     true,
		ThumbnailWebPEnabled:         true,
		ThumbnailAVIFEnabled:         true,
		S3BackupEnabled:              true,
		S3BackupDelayMinutes:         5,    // Default: immediate backup (0 minutes delay)
		S3BackupCheckInterval:        5,    // Default: check every 5 minutes
		S3RetryInterval:              2,    // Default: retry every 2 minutes
		JobQueueWorkerCount:          5,    // Default: 5 workers
		ReplicationRequireChecksum:   true, // Default: enforce checksum for replication
	}

	// Load settings from database
	var settings []Setting
	if err := db.Find(&settings).Error; err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	// Apply loaded settings
	for _, setting := range settings {
		switch setting.Key {
		case "site_title":
			appSettings.SiteTitle = setting.Value
		case "site_description":
			appSettings.SiteDescription = setting.Value
		case "image_upload_enabled":
			appSettings.ImageUploadEnabled = setting.Value == "true"
		case "direct_upload_enabled":
			appSettings.DirectUploadEnabled = setting.Value == "true"
		case "upload_rate_limit_per_minute":
			if v, err := strconv.Atoi(setting.Value); err == nil {
				appSettings.UploadRateLimitPerMinute = v
			}
		case "upload_user_rate_limit_per_minute":
			if v, err := strconv.Atoi(setting.Value); err == nil {
				appSettings.UploadUserRateLimitPerMinute = v
			}
		case "thumbnail_original_enabled":
			appSettings.ThumbnailOriginalEnabled = setting.Value == "true"
		case "thumbnail_webp_enabled":
			appSettings.ThumbnailWebPEnabled = setting.Value == "true"
		case "thumbnail_avif_enabled":
			appSettings.ThumbnailAVIFEnabled = setting.Value == "true"
		case "s3_backup_enabled":
			appSettings.S3BackupEnabled = setting.Value == "true"
		case "s3_backup_delay_minutes":
			if minutes, err := strconv.Atoi(setting.Value); err == nil {
				appSettings.S3BackupDelayMinutes = minutes
			}
		case "s3_backup_check_interval":
			if interval, err := strconv.Atoi(setting.Value); err == nil {
				appSettings.S3BackupCheckInterval = interval
			}
		case "s3_retry_interval":
			if interval, err := strconv.Atoi(setting.Value); err == nil {
				appSettings.S3RetryInterval = interval
			}
		case "job_queue_worker_count":
			if count, err := strconv.Atoi(setting.Value); err == nil {
				appSettings.JobQueueWorkerCount = count
			}
		case "replication_require_checksum":
			appSettings.ReplicationRequireChecksum = setting.Value == "true"
		}
	}

	return nil
}

// SaveSettings saves current settings to database
func SaveSettings(db *gorm.DB, settings *AppSettings) error {
	settingsMu.Lock()
	defer settingsMu.Unlock()

	// Validate settings
	if err := settings.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Convert settings to database format
	settingsMap := map[string]interface{}{
		"site_title":                        settings.SiteTitle,
		"site_description":                  settings.SiteDescription,
		"image_upload_enabled":              fmt.Sprintf("%t", settings.ImageUploadEnabled),
		"direct_upload_enabled":             fmt.Sprintf("%t", settings.DirectUploadEnabled),
		"upload_rate_limit_per_minute":      fmt.Sprintf("%d", settings.UploadRateLimitPerMinute),
		"upload_user_rate_limit_per_minute": fmt.Sprintf("%d", settings.UploadUserRateLimitPerMinute),
		"thumbnail_original_enabled":        fmt.Sprintf("%t", settings.ThumbnailOriginalEnabled),
		"thumbnail_webp_enabled":            fmt.Sprintf("%t", settings.ThumbnailWebPEnabled),
		"thumbnail_avif_enabled":            fmt.Sprintf("%t", settings.ThumbnailAVIFEnabled),
		"s3_backup_enabled":                 fmt.Sprintf("%t", settings.S3BackupEnabled),
		"s3_backup_delay_minutes":           fmt.Sprintf("%d", settings.S3BackupDelayMinutes),
		"s3_backup_check_interval":          fmt.Sprintf("%d", settings.S3BackupCheckInterval),
		"s3_retry_interval":                 fmt.Sprintf("%d", settings.S3RetryInterval),
		"job_queue_worker_count":            fmt.Sprintf("%d", settings.JobQueueWorkerCount),
		"replication_require_checksum":      fmt.Sprintf("%t", settings.ReplicationRequireChecksum),
	}

	// Save each setting
	for key, value := range settingsMap {
		var setting Setting
		result := db.Where("setting_key = ?", key).First(&setting)

		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				// Create new setting
				setting = Setting{
					Key:   key,
					Value: fmt.Sprintf("%v", value),
					Type:  getSettingType(key),
				}
				if err := db.Create(&setting).Error; err != nil {
					return fmt.Errorf("failed to create setting %s: %w", key, err)
				}
			} else {
				return fmt.Errorf("failed to query setting %s: %w", key, result.Error)
			}
		} else {
			// Update existing setting
			setting.Value = fmt.Sprintf("%v", value)
			if err := db.Save(&setting).Error; err != nil {
				return fmt.Errorf("failed to update setting %s: %w", key, err)
			}
		}
	}

	// Update global settings
	appSettings = settings
	return nil
}

// getSettingType returns the type of a setting based on its key
func getSettingType(key string) string {
	switch key {
	case "site_title", "site_description":
		return "string"
	case "image_upload_enabled", "direct_upload_enabled", "thumbnail_original_enabled", "thumbnail_webp_enabled", "thumbnail_avif_enabled", "replication_require_checksum", "s3_backup_enabled":
		return "boolean"
	case "s3_backup_delay_minutes", "s3_backup_check_interval", "s3_retry_interval", "job_queue_worker_count", "upload_rate_limit_per_minute", "upload_user_rate_limit_per_minute":
		return "integer"
	default:
		return "string"
	}
}

// Validate validates the settings
func (s *AppSettings) Validate() error {
	validate := validator.New()
	return validate.Struct(s)
}

// ToJSON converts settings to JSON
func (s *AppSettings) ToJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(s)
}

// FromJSON loads settings from JSON
func (s *AppSettings) FromJSON(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return json.Unmarshal(data, s)
}

// GetSiteTitle returns the site title
func (s *AppSettings) GetSiteTitle() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.SiteTitle
}

// GetSiteDescription returns the site description
func (s *AppSettings) GetSiteDescription() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.SiteDescription
}

// IsImageUploadEnabled returns whether image upload is enabled
func (s *AppSettings) IsImageUploadEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ImageUploadEnabled
}

// IsDirectUploadEnabled returns whether direct-to-storage upload is enabled
func (s *AppSettings) IsDirectUploadEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.DirectUploadEnabled
}

// GetUploadRateLimitPerMinute returns API rate limit for uploads (0 = unlimited)
func (s *AppSettings) GetUploadRateLimitPerMinute() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.UploadRateLimitPerMinute
}

// GetUploadUserRateLimitPerMinute returns per-user upload rate limit per minute (0 = unlimited)
func (s *AppSettings) GetUploadUserRateLimitPerMinute() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.UploadUserRateLimitPerMinute
}

// IsThumbnailOriginalEnabled returns whether original format thumbnails are enabled
func (s *AppSettings) IsThumbnailOriginalEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ThumbnailOriginalEnabled
}

// IsThumbnailWebPEnabled returns whether WebP format thumbnails are enabled
func (s *AppSettings) IsThumbnailWebPEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ThumbnailWebPEnabled
}

// IsThumbnailAVIFEnabled returns whether AVIF format thumbnails are enabled
func (s *AppSettings) IsThumbnailAVIFEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ThumbnailAVIFEnabled
}

// GetS3BackupDelayMinutes returns the S3 backup delay in minutes
func (s *AppSettings) GetS3BackupDelayMinutes() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.S3BackupDelayMinutes
}

// GetS3BackupCheckInterval returns the S3 backup check interval in minutes
func (s *AppSettings) GetS3BackupCheckInterval() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.S3BackupCheckInterval
}

// GetS3RetryInterval returns the S3 retry interval in minutes
func (s *AppSettings) GetS3RetryInterval() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.S3RetryInterval
}

// GetJobQueueWorkerCount returns the job queue worker count
func (s *AppSettings) GetJobQueueWorkerCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.JobQueueWorkerCount
}

// IsReplicationChecksumRequired returns whether replication checksum validation is required
func (s *AppSettings) IsReplicationChecksumRequired() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ReplicationRequireChecksum
}

// IsS3BackupEnabled returns whether S3 backups are enabled via admin settings
func (s *AppSettings) IsS3BackupEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.S3BackupEnabled
}
