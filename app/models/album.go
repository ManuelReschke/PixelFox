package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/ManuelReschke/PixelFox/internal/pkg/shortener"
)

type Album struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	UserID       uint           `gorm:"index" json:"user_id"`
	User         User           `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Title        string         `gorm:"type:varchar(255);not null" json:"title" validate:"required,min=3,max=255"`
	Description  string         `gorm:"type:text" json:"description"`
	CoverImageID uint           `json:"cover_image_id"`
	IsPublic     bool           `gorm:"default:false" json:"is_public"`
	ShareLink    string         `gorm:"type:varchar(255) CHARACTER SET utf8 COLLATE utf8_bin;uniqueIndex" json:"share_link"`
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
	// Generiere einen eindeutigen ShareLink, falls nicht vorhanden
	if a.ShareLink == "" {
		// Temporärer ShareLink für den Insert
		a.ShareLink = "temp-" + uuid.New().String()[:8]
	}
	return nil
}

// AfterCreate wird nach dem Erstellen eines neuen Datensatzes aufgerufen
func (a *Album) AfterCreate(tx *gorm.DB) error {
	// Generiere den ShareLink basierend auf der ID
	if a.ShareLink != "" && (a.ShareLink[:5] == "temp-" || a.ShareLink == "") {
		a.ShareLink = shortener.EncodeID(a.ID)
		// Aktualisiere den ShareLink in der Datenbank
		return tx.Model(a).Update("share_link", a.ShareLink).Error
	}
	return nil
}
