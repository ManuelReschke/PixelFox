package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/ManuelReschke/PixelFox/internal/pkg/shortener"
)

// JSON ist ein Typ für die Speicherung von JSON-Daten in der Datenbank
type JSON json.RawMessage

// Value implementiert das driver.Valuer Interface
func (j JSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
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

type Image struct {
	ID                 uint   `gorm:"primaryKey" json:"id"`
	UUID               string `gorm:"type:char(36) CHARACTER SET utf8 COLLATE utf8_bin;uniqueIndex;not null" json:"uuid"`
	UserID             uint   `gorm:"index" json:"user_id"`
	User               User   `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Title              string `gorm:"type:varchar(255)" json:"title"`
	Description        string `gorm:"type:text" json:"description"`
	FilePath           string `gorm:"type:varchar(255);not null" json:"file_path"`
	FileName           string `gorm:"type:varchar(255);not null" json:"file_name"`
	FileSize           int64  `gorm:"type:bigint" json:"file_size"`
	FileType           string `gorm:"type:varchar(50)" json:"file_type"`
	Width              int    `gorm:"type:int" json:"width"`
	Height             int    `gorm:"type:int" json:"height"`
	ShareLink          string `gorm:"type:varchar(255) CHARACTER SET utf8 COLLATE utf8_bin;uniqueIndex" json:"share_link"`
	IsPublic           bool   `gorm:"default:false" json:"is_public"`
	ViewCount          int    `gorm:"default:0" json:"view_count"`
	DownloadCount      int    `gorm:"default:0" json:"download_count"`
	HasWebp            bool   `gorm:"default:false" json:"has_webp"`
	HasAVIF            bool   `gorm:"default:false" json:"has_avif"`
	HasThumbnailSmall  bool   `gorm:"default:false" json:"has_thumbnail_small"`
	HasThumbnailMedium bool   `gorm:"default:false" json:"has_thumbnail_medium"`
	// meta data
	CameraModel  *string    `gorm:"type:varchar(255)" json:"camera_model"`
	TakenAt      *time.Time `gorm:"type:datetime" json:"taken_at"`
	Latitude     *float64   `gorm:"type:decimal(10,8)" json:"latitude"`
	Longitude    *float64   `gorm:"type:decimal(11,8)" json:"longitude"`
	ExposureTime *string    `gorm:"type:varchar(50)" json:"exposure_time"`
	Aperture     *string    `gorm:"type:varchar(20)" json:"aperture"`
	ISO          *int       `gorm:"type:int" json:"iso"`
	FocalLength  *string    `gorm:"type:varchar(20)" json:"focal_length"`
	Metadata     *JSON      `gorm:"type:json" json:"metadata"`
	IPv4         string     `gorm:"type:varchar(15);default:null" json:"-"` // IPv4 address of the uploader
	IPv6         string     `gorm:"type:varchar(45);default:null" json:"-"` // IPv6 address of the uploader
	// relations
	Tags      []Tag          `gorm:"many2many:image_tags;" json:"tags,omitempty"`
	Comments  []Comment      `gorm:"foreignKey:ImageID" json:"comments,omitempty"`
	Likes     []Like         `gorm:"foreignKey:ImageID" json:"likes,omitempty"`
	Albums    []Album        `gorm:"many2many:album_images;" json:"albums,omitempty"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// BeforeCreate wird vor dem Erstellen eines neuen Datensatzes aufgerufen
func (i *Image) BeforeCreate(tx *gorm.DB) error {
	// Generiere UUID, falls nicht vorhanden
	if i.UUID == "" {
		i.UUID = uuid.New().String()
	}

	// Generiere einen eindeutigen ShareLink, falls nicht vorhanden
	if i.ShareLink == "" {
		// Wir müssen zuerst das Objekt speichern, um eine ID zu bekommen
		// Daher setzen wir einen temporären ShareLink
		i.ShareLink = "temp"
	}

	return nil
}

// AfterCreate wird nach dem Erstellen eines neuen Datensatzes aufgerufen
func (i *Image) AfterCreate(tx *gorm.DB) error {
	// Jetzt haben wir eine ID und können den ShareLink generieren
	if i.ShareLink == "temp" {
		// Generiere den ShareLink basierend auf der ID
		i.ShareLink = shortener.EncodeID(i.ID)

		// Aktualisiere den ShareLink in der Datenbank
		return tx.Model(i).Update("share_link", i.ShareLink).Error
	}

	return nil
}

// IncrementViewCount erhöht den Zähler für Aufrufe
func (i *Image) IncrementViewCount(db *gorm.DB) error {
	return db.Model(i).Update("view_count", i.ViewCount+1).Error
}

// IncrementDownloadCount erhöht den Zähler für Downloads
func (i *Image) IncrementDownloadCount(db *gorm.DB) error {
	return db.Model(i).Update("download_count", i.DownloadCount+1).Error
}

// TogglePublic ändert den öffentlichen Status des Bildes
func (i *Image) TogglePublic(db *gorm.DB) error {
	i.IsPublic = !i.IsPublic
	return db.Model(i).Update("is_public", i.IsPublic).Error
}

// FindByUUID findet ein Bild anhand seiner UUID
func FindImageByUUID(db *gorm.DB, uuid string) (*Image, error) {
	var image Image
	result := db.Where("uuid = ?", uuid).First(&image)
	return &image, result.Error
}

// FindByFilename findet ein Bild anhand seines Dateinamens
func FindImageByFilename(db *gorm.DB, filename string) (*Image, error) {
	var image Image
	result := db.Where("file_name = ?", filename).First(&image)
	return &image, result.Error
}
