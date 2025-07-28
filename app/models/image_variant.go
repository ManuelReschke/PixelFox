package models

import (
	"time"

	"gorm.io/gorm"
)

// Image variant type constants
const (
	VariantTypeWebP                = "webp"
	VariantTypeAVIF                = "avif"
	VariantTypeThumbnailSmallWebP  = "thumbnail_small_webp"
	VariantTypeThumbnailSmallAVIF  = "thumbnail_small_avif"
	VariantTypeThumbnailSmallOrig  = "thumbnail_small_original"
	VariantTypeThumbnailMediumWebP = "thumbnail_medium_webp"
	VariantTypeThumbnailMediumAVIF = "thumbnail_medium_avif"
	VariantTypeThumbnailMediumOrig = "thumbnail_medium_original"
	VariantTypeOriginal            = "original"
)

type ImageVariant struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	ImageID     uint           `gorm:"index;not null" json:"image_id"`
	Image       Image          `gorm:"foreignKey:ImageID" json:"image,omitempty"`
	VariantType string         `gorm:"type:varchar(50);not null" json:"variant_type"` // thumbnail_small_webp, thumbnail_medium_webp, thumbnail_small_avif, thumbnail_medium_avif, webp, avif
	FilePath    string         `gorm:"type:varchar(255);not null" json:"file_path"`
	FileName    string         `gorm:"type:varchar(255);not null" json:"file_name"`
	FileType    string         `gorm:"type:varchar(50);not null" json:"file_type"`
	FileSize    int64          `gorm:"type:bigint;not null" json:"file_size"`
	Width       int            `gorm:"type:int" json:"width"`
	Height      int            `gorm:"type:int" json:"height"`
	Quality     int            `gorm:"type:int" json:"quality"` // Compression quality for formats that support it
	CreatedAt   time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName returns the table name for the ImageVariant model
func (ImageVariant) TableName() string {
	return "image_variants"
}

// BeforeCreate is called before creating a new record
func (iv *ImageVariant) BeforeCreate(tx *gorm.DB) error {
	// Validate variant type - "original" is NO LONGER valid, original data is stored in images table
	validTypes := []string{VariantTypeThumbnailSmallWebP, VariantTypeThumbnailSmallAVIF, VariantTypeThumbnailSmallOrig, VariantTypeThumbnailMediumWebP, VariantTypeThumbnailMediumAVIF, VariantTypeThumbnailMediumOrig, VariantTypeWebP, VariantTypeAVIF}
	isValid := false
	for _, validType := range validTypes {
		if iv.VariantType == validType {
			isValid = true
			break
		}
	}
	if !isValid {
		return gorm.ErrInvalidValue
	}

	return nil
}

// FindVariantsByImageID finds all variants for a specific image
func FindVariantsByImageID(db *gorm.DB, imageID uint) ([]ImageVariant, error) {
	var variants []ImageVariant
	result := db.Where("image_id = ?", imageID).Find(&variants)
	return variants, result.Error
}

// FindVariantByImageIDAndType finds a specific variant for an image
func FindVariantByImageIDAndType(db *gorm.DB, imageID uint, variantType string) (*ImageVariant, error) {
	var variant ImageVariant
	result := db.Where("image_id = ? AND variant_type = ?", imageID, variantType).First(&variant)
	return &variant, result.Error
}

// HasVariant checks if a specific variant exists for an image
func HasVariant(db *gorm.DB, imageID uint, variantType string) bool {
	var count int64
	db.Model(&ImageVariant{}).Where("image_id = ? AND variant_type = ?", imageID, variantType).Count(&count)
	return count > 0
}

// GetVariantTypes returns all variant types for an image
func GetVariantTypes(db *gorm.DB, imageID uint) ([]string, error) {
	var types []string
	result := db.Model(&ImageVariant{}).Where("image_id = ?", imageID).Pluck("variant_type", &types)
	return types, result.Error
}

// DeleteVariantsByImageID deletes all variants for a specific image
func DeleteVariantsByImageID(db *gorm.DB, imageID uint) error {
	return db.Where("image_id = ?", imageID).Delete(&ImageVariant{}).Error
}
