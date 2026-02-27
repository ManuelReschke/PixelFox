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
	JobQueueWorkerCount      int  `json:"job_queue_worker_count" validate:"min=1,max=20"` // Number of job queue workers (1-20)
	// API rate limiting
	APIRateLimitPerMinute int `json:"api_rate_limit_per_minute" validate:"min=0,max=100000"` // Global API limiter for /api routes (0 = unlimited)
	// Replication/Storage settings
	ReplicationRequireChecksum bool `json:"replication_require_checksum"`
	// Tiering (Phase A)
	TieringEnabled               bool `json:"tiering_enabled"`
	HotKeepDaysAfterUpload       int  `json:"hot_keep_days_after_upload" validate:"min=0,max=3650"`
	DemoteIfNoViewsDays          int  `json:"demote_if_no_views_days" validate:"min=1,max=3650"`
	MinDwellDaysPerTier          int  `json:"min_dwell_days_per_tier" validate:"min=0,max=3650"`
	HotWatermarkHigh             int  `json:"hot_watermark_high" validate:"min=1,max=100"`
	HotWatermarkLow              int  `json:"hot_watermark_low" validate:"min=0,max=100"`
	MaxTieringCandidatesPerSweep int  `json:"max_tiering_candidates_per_sweep" validate:"min=1,max=100000"`
	TieringSweepIntervalMinutes  int  `json:"tiering_sweep_interval_minutes" validate:"min=1,max=1440"`
	mu                           sync.RWMutex
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
		JobQueueWorkerCount:          5,    // Default: 5 workers
		APIRateLimitPerMinute:        120,  // Default: 120 requests / minute for /api routes
		ReplicationRequireChecksum:   true, // Default: enforce checksum for replication
		// Tiering defaults (Phase A)
		TieringEnabled:               true,
		HotKeepDaysAfterUpload:       7,
		DemoteIfNoViewsDays:          30,
		MinDwellDaysPerTier:          7,
		HotWatermarkHigh:             80,
		HotWatermarkLow:              65,
		MaxTieringCandidatesPerSweep: 200,
		TieringSweepIntervalMinutes:  15,
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
		case "job_queue_worker_count":
			if count, err := strconv.Atoi(setting.Value); err == nil {
				appSettings.JobQueueWorkerCount = count
			}
		case "api_rate_limit_per_minute":
			if v, err := strconv.Atoi(setting.Value); err == nil {
				appSettings.APIRateLimitPerMinute = v
			}
		case "replication_require_checksum":
			appSettings.ReplicationRequireChecksum = setting.Value == "true"
		case "tiering_enabled":
			appSettings.TieringEnabled = setting.Value == "true"
		case "hot_keep_days_after_upload":
			if v, err := strconv.Atoi(setting.Value); err == nil {
				appSettings.HotKeepDaysAfterUpload = v
			}
		case "demote_if_no_views_days":
			if v, err := strconv.Atoi(setting.Value); err == nil {
				appSettings.DemoteIfNoViewsDays = v
			}
		case "min_dwell_days_per_tier":
			if v, err := strconv.Atoi(setting.Value); err == nil {
				appSettings.MinDwellDaysPerTier = v
			}
		case "hot_watermark_high":
			if v, err := strconv.Atoi(setting.Value); err == nil {
				appSettings.HotWatermarkHigh = v
			}
		case "hot_watermark_low":
			if v, err := strconv.Atoi(setting.Value); err == nil {
				appSettings.HotWatermarkLow = v
			}
		case "max_tiering_candidates_per_sweep":
			if v, err := strconv.Atoi(setting.Value); err == nil {
				appSettings.MaxTieringCandidatesPerSweep = v
			}
		case "tiering_sweep_interval_minutes":
			if v, err := strconv.Atoi(setting.Value); err == nil {
				appSettings.TieringSweepIntervalMinutes = v
			}
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
		"job_queue_worker_count":            fmt.Sprintf("%d", settings.JobQueueWorkerCount),
		"api_rate_limit_per_minute":         fmt.Sprintf("%d", settings.APIRateLimitPerMinute),
		"replication_require_checksum":      fmt.Sprintf("%t", settings.ReplicationRequireChecksum),
		// Tiering
		"tiering_enabled":                  fmt.Sprintf("%t", settings.TieringEnabled),
		"hot_keep_days_after_upload":       fmt.Sprintf("%d", settings.HotKeepDaysAfterUpload),
		"demote_if_no_views_days":          fmt.Sprintf("%d", settings.DemoteIfNoViewsDays),
		"min_dwell_days_per_tier":          fmt.Sprintf("%d", settings.MinDwellDaysPerTier),
		"hot_watermark_high":               fmt.Sprintf("%d", settings.HotWatermarkHigh),
		"hot_watermark_low":                fmt.Sprintf("%d", settings.HotWatermarkLow),
		"max_tiering_candidates_per_sweep": fmt.Sprintf("%d", settings.MaxTieringCandidatesPerSweep),
		"tiering_sweep_interval_minutes":   fmt.Sprintf("%d", settings.TieringSweepIntervalMinutes),
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
	case "image_upload_enabled", "direct_upload_enabled", "thumbnail_original_enabled", "thumbnail_webp_enabled", "thumbnail_avif_enabled", "replication_require_checksum", "tiering_enabled":
		return "boolean"
	case "job_queue_worker_count", "upload_rate_limit_per_minute", "upload_user_rate_limit_per_minute", "hot_keep_days_after_upload", "demote_if_no_views_days", "min_dwell_days_per_tier", "hot_watermark_high", "hot_watermark_low", "max_tiering_candidates_per_sweep", "tiering_sweep_interval_minutes", "api_rate_limit_per_minute":
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

// GetAPIRateLimitPerMinute returns the API limiter value for /api routes (0 = unlimited)
func (s *AppSettings) GetAPIRateLimitPerMinute() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.APIRateLimitPerMinute
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

// Tiering getters
func (s *AppSettings) IsTieringEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.TieringEnabled
}

func (s *AppSettings) GetHotKeepDaysAfterUpload() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.HotKeepDaysAfterUpload
}

func (s *AppSettings) GetDemoteIfNoViewsDays() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.DemoteIfNoViewsDays
}

func (s *AppSettings) GetMinDwellDaysPerTier() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.MinDwellDaysPerTier
}

func (s *AppSettings) GetHotWatermarkHigh() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.HotWatermarkHigh
}

func (s *AppSettings) GetHotWatermarkLow() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.HotWatermarkLow
}

func (s *AppSettings) GetMaxTieringCandidatesPerSweep() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.MaxTieringCandidatesPerSweep
}

func (s *AppSettings) GetTieringSweepIntervalMinutes() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.TieringSweepIntervalMinutes
}
