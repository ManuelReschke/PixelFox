package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Album struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	UserID       uint           `gorm:"index" json:"user_id"`
	User         User           `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Title        string         `gorm:"type:varchar(255);not null" json:"title" validate:"required,min=3,max=255"`
	Description  string         `gorm:"type:text" json:"description"`
	CoverImageID uint           `json:"cover_image_id"`
	IsPublic     bool           `gorm:"default:false" json:"is_public"`
	ShareLink    string         `gorm:"type:char(36) CHARACTER SET utf8 COLLATE utf8_bin;uniqueIndex;not null" json:"share_link"`
	ViewCount    int            `gorm:"default:0" json:"view_count"`
	Images       []Image        `gorm:"many2many:album_images;" json:"images,omitempty"`
	CreatedAt    time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// IncrementViewCount erhöht den Zähler für Aufrufe
func (a *Album) IncrementViewCount(db *gorm.DB) error {
	return db.Model(a).Update("view_count", a.ViewCount+1).Error
}

// TogglePublic ändert den öffentlichen Status des Albums
func (a *Album) TogglePublic(db *gorm.DB) error {
	a.IsPublic = !a.IsPublic
	return db.Model(a).Update("is_public", a.IsPublic).Error
}

// AddImage fügt ein Bild zum Album hinzu
func (a *Album) AddImage(db *gorm.DB, imageID uint) error {
	return db.Exec("INSERT INTO album_images (album_id, image_id) VALUES (?, ?)", a.ID, imageID).Error
}

// RemoveImage entfernt ein Bild aus dem Album
func (a *Album) RemoveImage(db *gorm.DB, imageID uint) error {
	return db.Exec("DELETE FROM album_images WHERE album_id = ? AND image_id = ?", a.ID, imageID).Error
}

// BeforeCreate wird vor dem Erstellen eines neuen Datensatzes aufgerufen
func (a *Album) BeforeCreate(tx *gorm.DB) error {
	// Generiere einen eindeutigen ShareLink als UUIDv4
	if a.ShareLink == "" {
		a.ShareLink = uuid.New().String()
	}
	return nil
}

// AfterCreate wird nicht benötigt, da der ShareLink bereits als UUID gesetzt wird
