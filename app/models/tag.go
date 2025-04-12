package models

import (
	"time"

	"gorm.io/gorm"
)

type Tag struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Name      string         `gorm:"type:varchar(100) CHARACTER SET utf8 COLLATE utf8_bin;uniqueIndex" json:"name" validate:"required,min=2,max=100"`
	Images    []Image        `gorm:"many2many:image_tags;" json:"images,omitempty"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// FindOrCreate findet einen Tag anhand des Namens oder erstellt ihn, wenn er nicht existiert
func (t *Tag) FindOrCreate(db *gorm.DB) error {
	result := db.Where("name = ?", t.Name).First(t)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return db.Create(t).Error
		}
		return result.Error
	}
	return nil
}
