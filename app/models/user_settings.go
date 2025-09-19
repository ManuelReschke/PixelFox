package models

import (
	"time"

	"gorm.io/gorm"
)

// UserSettings stores per-user preferences and plan info
type UserSettings struct {
	ID                uint           `gorm:"primaryKey" json:"id"`
	UserID            uint           `gorm:"uniqueIndex" json:"user_id"`
	Plan              string         `gorm:"type:varchar(50);default:'free'" json:"plan"`
	PrefThumbOriginal bool           `gorm:"default:true" json:"pref_thumb_original"`
	PrefThumbWebP     bool           `gorm:"default:false" json:"pref_thumb_webp"`
	PrefThumbAVIF     bool           `gorm:"default:false" json:"pref_thumb_avif"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`
}

// GetOrCreateUserSettings returns existing settings or creates defaults
func GetOrCreateUserSettings(db *gorm.DB, userID uint) (*UserSettings, error) {
	var us UserSettings
	if err := db.Where("user_id = ?", userID).First(&us).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			us = UserSettings{UserID: userID, Plan: "free", PrefThumbOriginal: true}
			if err := db.Create(&us).Error; err != nil {
				return nil, err
			}
			return &us, nil
		}
		return nil, err
	}
	return &us, nil
}
