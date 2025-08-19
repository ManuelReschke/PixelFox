package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

// JSON ist ein Typ für die Speicherung von JSON-Daten in der Datenbank
type JSON json.RawMessage

// Value implementiert das driver.Valuer Interface
func (j JSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}

	// Validate JSON before returning
	var temp interface{}
	if err := json.Unmarshal(j, &temp); err != nil {
		// If JSON is invalid, return empty JSON object
		return "{}", nil
	}

	return string(j), nil
}

// Scan implementiert das sql.Scanner Interface
func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = JSON("{}")
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("invalid scan source")
	}
	*j = JSON(bytes)
	return nil
}

// MarshalJSON implementiert das json.Marshaler Interface
func (j JSON) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return j, nil
}

// UnmarshalJSON implementiert das json.Unmarshaler Interface
func (j *JSON) UnmarshalJSON(data []byte) error {
	*j = JSON(data)
	return nil
}

// ImageMetadata contains the metadata information of an image
type ImageMetadata struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	ImageID      uint           `gorm:"index;not null" json:"image_id"`
	CameraModel  *string        `gorm:"type:varchar(255)" json:"camera_model"`
	TakenAt      *time.Time     `gorm:"type:datetime" json:"taken_at"`
	Latitude     *float64       `gorm:"type:decimal(10,8)" json:"latitude"`
	Longitude    *float64       `gorm:"type:decimal(11,8)" json:"longitude"`
	ExposureTime *string        `gorm:"type:varchar(50)" json:"exposure_time"`
	Aperture     *string        `gorm:"type:varchar(20)" json:"aperture"`
	ISO          *int           `gorm:"type:int" json:"iso"`
	FocalLength  *string        `gorm:"type:varchar(20)" json:"focal_length"`
	Metadata     *JSON          `gorm:"type:json" json:"metadata"`
	CreatedAt    time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// FindMetadataByImageID finds metadata for an image by its ID
func FindMetadataByImageID(db *gorm.DB, imageID uint) (*ImageMetadata, error) {
	var metadata ImageMetadata
	result := db.Where("image_id = ?", imageID).First(&metadata)
	return &metadata, result.Error
}
