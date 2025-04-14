package models

import (
	"time"
)

// ImageVariant represents a processed variant of an original image
type ImageVariant struct {
	ID          uint      `gorm:"primaryKey"`
	ImageID     uint      `gorm:"not null"`
	VariantType string    `gorm:"size:32;not null"` // e.g., "webp", "thumb_small", "avif"
	Path        string    `gorm:"type:text;not null"`
	Width       *int      `gorm:"default:null"`
	Height      *int      `gorm:"default:null"`
	FileSize    *int64    `gorm:"default:null"`
	Quality     *int      `gorm:"default:null"`
	CreatedAt   time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt   time.Time `gorm:"default:CURRENT_TIMESTAMP;autoUpdateTime"`

	// Relations
	Image Image `gorm:"foreignKey:ImageID"`
}

// TableName overrides the table name
func (ImageVariant) TableName() string {
	return "image_variants"
}
