package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/ManuelReschke/PixelFox/internal/pkg/shortener"
)

type Image struct {
	ID            uint   `gorm:"primaryKey" json:"id"`
	UUID          string `gorm:"type:char(36) CHARACTER SET utf8 COLLATE utf8_bin;uniqueIndex;not null" json:"uuid"`
	UserID        uint   `gorm:"index" json:"user_id"`
	User          User   `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Title         string `gorm:"type:varchar(255)" json:"title"`
	Description   string `gorm:"type:text" json:"description"`
	FilePath      string `gorm:"type:varchar(255);not null" json:"file_path"`
	FileName      string `gorm:"type:varchar(255);not null" json:"file_name"`
	FileSize      int64  `gorm:"type:bigint" json:"file_size"`
	FileType      string `gorm:"type:varchar(50)" json:"file_type"`
	Width         int    `gorm:"type:int" json:"width"`
	Height        int    `gorm:"type:int" json:"height"`
	ShareLink     string `gorm:"type:varchar(255) CHARACTER SET utf8 COLLATE utf8_bin;uniqueIndex" json:"share_link"`
	IsPublic      bool   `gorm:"default:false" json:"is_public"`
	ViewCount     int    `gorm:"default:0" json:"view_count"`
	DownloadCount int    `gorm:"default:0" json:"download_count"`
	IPv4          string `gorm:"type:varchar(15);default:null" json:"-"` // IPv4 address of the uploader
	IPv6          string `gorm:"type:varchar(45);default:null" json:"-"` // IPv6 address of the uploader
	// relations
	Metadata  *ImageMetadata `gorm:"foreignKey:ImageID" json:"metadata,omitempty"`
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
	result := db.Preload("Metadata").Where("uuid = ?", uuid).First(&image)
	return &image, result.Error
}

// FindByFilename findet ein Bild anhand seines Dateinamens
func FindImageByFilename(db *gorm.DB, filename string) (*Image, error) {
	var image Image
	result := db.Where("file_name = ?", filename).First(&image)
	return &image, result.Error
}
