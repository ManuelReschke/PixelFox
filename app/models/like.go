package models

import (
	"time"

	"gorm.io/gorm"
)

type Like struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	UserID    uint           `gorm:"index" json:"user_id"`
	User      User           `gorm:"foreignKey:UserID" json:"user,omitempty"`
	ImageID   uint           `gorm:"index" json:"image_id"`
	Image     Image          `gorm:"foreignKey:ImageID" json:"image,omitempty"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// ToggleLike erstellt oder entfernt einen Like
func ToggleLike(db *gorm.DB, userID, imageID uint) error {
	var like Like
	result := db.Where("user_id = ? AND image_id = ?", userID, imageID).First(&like)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			// Like existiert nicht, erstelle ihn
			newLike := Like{
				UserID:  userID,
				ImageID: imageID,
			}
			return db.Create(&newLike).Error
		}
		return result.Error
	}

	// Like existiert, entferne ihn
	return db.Delete(&like).Error
}
