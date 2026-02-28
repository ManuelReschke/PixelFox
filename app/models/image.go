package models

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/ManuelReschke/PixelFox/internal/pkg/shortener"
)

type Image struct {
	ID             uint         `gorm:"primaryKey" json:"id"`
	UUID           string       `gorm:"type:char(36) CHARACTER SET utf8 COLLATE utf8_bin;uniqueIndex;not null" json:"uuid"`
	UserID         uint         `gorm:"index;index:idx_user_file_hash,priority:1;uniqueIndex:ux_images_user_active_file_hash,priority:1" json:"user_id"`
	User           User         `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Title          string       `gorm:"type:varchar(255)" json:"title"`
	Description    string       `gorm:"type:text" json:"description"`
	FilePath       string       `gorm:"type:varchar(255);not null" json:"file_path"`
	FileName       string       `gorm:"type:varchar(255);not null" json:"file_name"`
	FileSize       int64        `gorm:"type:bigint" json:"file_size"`
	FileType       string       `gorm:"type:varchar(50)" json:"file_type"`
	Width          int          `gorm:"type:int" json:"width"`
	Height         int          `gorm:"type:int" json:"height"`
	ShareLink      string       `gorm:"type:varchar(16) CHARACTER SET utf8 COLLATE utf8_bin;uniqueIndex" json:"share_link"`
	IsPublic       bool         `gorm:"default:false" json:"is_public"`
	ViewCount      int          `gorm:"default:0" json:"view_count"`
	DownloadCount  int          `gorm:"default:0" json:"download_count"`
	LastViewedAt   *time.Time   `gorm:"index" json:"last_viewed_at,omitempty"`
	IPv4           string       `gorm:"type:varchar(15);default:null" json:"-"`                                                    // IPv4 address of the uploader
	IPv6           string       `gorm:"type:varchar(45);default:null" json:"-"`                                                    // IPv6 address of the uploader
	FileHash       string       `gorm:"type:varchar(64);not null;default:'';index:idx_user_file_hash,priority:2" json:"file_hash"` // SHA-256 hash for duplicate detection
	ActiveFileHash string       `gorm:"->;type:varchar(64) GENERATED ALWAYS AS (CASE WHEN deleted_at IS NULL THEN file_hash ELSE NULL END) STORED;default:(-);uniqueIndex:ux_images_user_active_file_hash,priority:2" json:"-"`
	StoragePoolID  uint         `gorm:"index;default:null" json:"storage_pool_id"` // Reference to storage pool
	StoragePool    *StoragePool `gorm:"foreignKey:StoragePoolID" json:"storage_pool,omitempty"`
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

const (
	imageShareLinkLength           = 10
	imageShareLinkGenerateMaxTries = 5
)

// BeforeCreate wird vor dem Erstellen eines neuen Datensatzes aufgerufen
func (i *Image) BeforeCreate(tx *gorm.DB) error {
	// Generiere UUID, falls nicht vorhanden
	if i.UUID == "" {
		i.UUID = uuid.New().String()
	}

	// Generiere einen kryptografisch sicheren ShareLink, falls nicht vorhanden
	if i.ShareLink == "" {
		shareLink, err := generateUniqueImageShareLink(tx)
		if err != nil {
			return err
		}
		i.ShareLink = shareLink
	}

	return nil
}

func generateUniqueImageShareLink(tx *gorm.DB) (string, error) {
	for attempt := 0; attempt < imageShareLinkGenerateMaxTries; attempt++ {
		candidate, err := shortener.GenerateSecureSlug(imageShareLinkLength)
		if err != nil {
			return "", fmt.Errorf("failed to generate secure image share link: %w", err)
		}

		var count int64
		if err := tx.Model(&Image{}).Where("share_link = ?", candidate).Limit(1).Count(&count).Error; err != nil {
			return "", fmt.Errorf("failed to check image share link uniqueness: %w", err)
		}
		if count == 0 {
			return candidate, nil
		}
	}

	return "", errors.New("failed to generate a unique image share link")
}

// IncrementViewCount erhöht den Zähler für Aufrufe
func (i *Image) IncrementViewCount(db *gorm.DB) error {
	// Use Model with only the ID to avoid updating related fields
	return db.Model(&Image{}).Where("id = ?", i.ID).UpdateColumn("view_count", gorm.Expr("view_count + ?", 1)).Error
}

// IncrementDownloadCount erhöht den Zähler für Downloads
func (i *Image) IncrementDownloadCount(db *gorm.DB) error {
	// Use Model with only the ID to avoid updating related fields
	return db.Model(&Image{}).Where("id = ?", i.ID).UpdateColumn("download_count", gorm.Expr("download_count + ?", 1)).Error
}

// TogglePublic ändert den öffentlichen Status des Bildes
func (i *Image) TogglePublic(db *gorm.DB) error {
	i.IsPublic = !i.IsPublic
	return db.Model(i).Update("is_public", i.IsPublic).Error
}

// FindByUUID findet ein Bild anhand seiner UUID
func FindImageByUUID(db *gorm.DB, uuid string) (*Image, error) {
	var image Image
	result := db.Preload("Metadata").Preload("StoragePool").Where("uuid = ?", uuid).First(&image)
	return &image, result.Error
}

// FindByFilename findet ein Bild anhand seines Dateinamens
func FindImageByFilename(db *gorm.DB, filename string) (*Image, error) {
	var image Image
	result := db.Where("file_name = ?", filename).First(&image)
	return &image, result.Error
}
