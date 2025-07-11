package models

import (
	"encoding/json"
	"fmt"
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
	SiteTitle          string `json:"site_title" validate:"required,min=1,max=255"`
	SiteDescription    string `json:"site_description" validate:"max=500"`
	ImageUploadEnabled bool   `json:"image_upload_enabled"`
	mu                 sync.RWMutex
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
		SiteTitle:          "PixelFox",
		SiteDescription:    "Image sharing platform",
		ImageUploadEnabled: true,
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
		"site_title":           settings.SiteTitle,
		"site_description":     settings.SiteDescription,
		"image_upload_enabled": fmt.Sprintf("%t", settings.ImageUploadEnabled),
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
	case "image_upload_enabled":
		return "boolean"
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
