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
	gorm.Model
	ID          uint           `gorm:"primaryKey" json:"id"`
	UUID        string         `gorm:"type:char(36) CHARACTER SET utf8 COLLATE utf8_bin;uniqueIndex;not null" json:"uuid"`
	UserID      uint           `gorm:"index" json:"user_id"`
	User        User           `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Filename    string         `gorm:"type:varchar(255);not null" json:"filename"`
	Title       string         `gorm:"type:varchar(255)" json:"title"`
	Description string         `gorm:"type:text" json:"description"`
	FilePath    string         `gorm:"column:file_path;type:varchar(255);not null" json:"file_path"`
	FileType    string         `gorm:"column:file_type;type:varchar(50)" json:"file_type"`
	Width       int            `gorm:"type:int" json:"width"`
	Height      int            `gorm:"type:int" json:"height"`
	Filesize    int64          `gorm:"type:bigint" json:"filesize"`
	ContentType string         `gorm:"type:varchar(100)" json:"content_type"`
	ShareLink   string         `gorm:"type:varchar(100) CHARACTER SET utf8 COLLATE utf8_bin;uniqueIndex" json:"share_link"`
	Downloads   int            `gorm:"type:int" json:"downloads"`
	Views       int            `gorm:"type:int" json:"views"`
	IsPublic    bool           `gorm:"default:false" json:"is_public"`
	CreatedAt   time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	// Metadata fields
	CameraModel  string     `gorm:"type:varchar(100)" json:"camera_model"`
	TakenAt      *time.Time `gorm:"type:datetime" json:"taken_at"`
	Latitude     *float64   `gorm:"type:decimal(10,8)" json:"latitude"`
	Longitude    *float64   `gorm:"type:decimal(11,8)" json:"longitude"`
	ExposureTime string     `gorm:"type:varchar(50)" json:"exposure_time"`
	Aperture     string     `gorm:"type:varchar(50)" json:"aperture"`
	ISO          *int       `gorm:"type:int" json:"iso"`
	FocalLength  string     `gorm:"type:varchar(50)" json:"focal_length"`
	Metadata     JSON       `gorm:"type:json" json:"metadata"`
	// Relations
	Variants   []ImageVariant `gorm:"foreignKey:ImageID" json:"variants,omitempty"`
	Tags       []Tag          `gorm:"many2many:image_tags;" json:"tags,omitempty"`
	Comments   []Comment      `gorm:"foreignKey:ImageID" json:"comments,omitempty"`
	Likes      []Like         `gorm:"foreignKey:ImageID" json:"likes,omitempty"`
	Albums     []Album        `gorm:"many2many:album_images;" json:"albums,omitempty"`
	PreviewURL string         `gorm:"-"`
	// Fields for compatibility with older code
	HasWebPField            bool   `gorm:"-"`
	HasAVIFField            bool   `gorm:"-"`
	HasThumbnailSmallField  bool   `gorm:"-"`
	HasThumbnailMediumField bool   `gorm:"-"`
	MimeType                string `gorm:"-"`
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
	return db.Model(i).Update("views", i.Views+1).Error
}

// IncrementDownloadCount erhöht den Zähler für Downloads
func (i *Image) IncrementDownloadCount(db *gorm.DB) error {
	return db.Model(i).Update("downloads", i.Downloads+1).Error
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
	result := db.Where("filename = ?", filename).First(&image)
	return &image, result.Error
}

// HasWebP prüft, ob eine WebP-Variante vorhanden ist
func (i *Image) HasWebP() bool {
	for _, variant := range i.Variants {
		if variant.VariantType == "webp" {
			return true
		}
	}
	return false
}

// HasAVIF prüft, ob eine AVIF-Variante vorhanden ist
func (i *Image) HasAVIF() bool {
	for _, variant := range i.Variants {
		if variant.VariantType == "avif" {
			return true
		}
	}
	return false
}

// HasThumbnailSmall prüft, ob eine kleine Thumbnail-Variante vorhanden ist
func (i *Image) HasThumbnailSmall() bool {
	for _, variant := range i.Variants {
		if variant.VariantType == "thumbnail_small" {
			return true
		}
	}
	return false
}

// HasThumbnailMedium prüft, ob eine mittlere Thumbnail-Variante vorhanden ist
func (i *Image) HasThumbnailMedium() bool {
	for _, variant := range i.Variants {
		if variant.VariantType == "thumbnail_medium" {
			return true
		}
	}
	return false
}

// HasVariant prüft, ob eine bestimmte Variante vorhanden ist
func (i *Image) HasVariant(variantType string) bool {
	for _, variant := range i.Variants {
		if variant.VariantType == variantType {
			return true
		}
	}
	return false
}
