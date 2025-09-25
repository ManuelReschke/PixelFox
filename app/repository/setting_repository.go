package repository

import (
	"github.com/ManuelReschke/PixelFox/app/models"
	"gorm.io/gorm"
)

// settingRepository implements the SettingRepository interface
type settingRepository struct {
	db *gorm.DB
}

// NewSettingRepository creates a new setting repository instance
func NewSettingRepository(db *gorm.DB) SettingRepository {
	return &settingRepository{db: db}
}

// Get retrieves the current application settings
func (r *settingRepository) Get() (*models.AppSettings, error) {
	return models.GetAppSettings(), nil
}

// Save saves the application settings to the database
func (r *settingRepository) Save(settings *models.AppSettings) error {
	return models.SaveSettings(r.db, settings)
}

// GetValue retrieves a specific setting value by key
func (r *settingRepository) GetValue(key string) (string, error) {
	var setting models.Setting
	// Correct column is `setting_key` (see gorm tag in models.Setting)
	err := r.db.Where("setting_key = ?", key).First(&setting).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", nil // Return empty string for non-existent settings
		}
		return "", err
	}
	return setting.Value, nil
}

// SetValue sets a specific setting value by key
func (r *settingRepository) SetValue(key, value string) error {
	var setting models.Setting
	// Correct column is `setting_key` (see gorm tag in models.Setting)
	err := r.db.Where("setting_key = ?", key).First(&setting).Error

	if err == gorm.ErrRecordNotFound {
		// Create new setting
		setting = models.Setting{
			Key:   key,
			Value: value,
		}
		return r.db.Create(&setting).Error
	} else if err != nil {
		return err
	}

	// Update existing setting
	setting.Value = value
	return r.db.Save(&setting).Error
}
