package models

import (
	"time"

	"gorm.io/gorm"
)

type Image struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	UserID        uint           `gorm:"index" json:"user_id"`
	User          User           `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Title         string         `gorm:"type:varchar(255)" json:"title"`
	Description   string         `gorm:"type:text" json:"description"`
	FilePath      string         `gorm:"type:varchar(255);not null" json:"file_path"`
	FileName      string         `gorm:"type:varchar(255);not null" json:"file_name"`
	FileSize      int64          `gorm:"type:bigint" json:"file_size"`
	FileType      string         `gorm:"type:varchar(50)" json:"file_type"`
	Width         int            `gorm:"type:int" json:"width"`
	Height        int            `gorm:"type:int" json:"height"`
	ShareLink     string         `gorm:"type:varchar(255);uniqueIndex" json:"share_link"`
	IsPublic      bool           `gorm:"default:false" json:"is_public"`
	ViewCount     int            `gorm:"default:0" json:"view_count"`
	DownloadCount int            `gorm:"default:0" json:"download_count"`
	Tags          []Tag          `gorm:"many2many:image_tags;" json:"tags,omitempty"`
	Comments      []Comment      `gorm:"foreignKey:ImageID" json:"comments,omitempty"`
	Likes         []Like         `gorm:"foreignKey:ImageID" json:"likes,omitempty"`
	Albums        []Album        `gorm:"many2many:album_images;" json:"albums,omitempty"`
	CreatedAt     time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
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
