package models

import (
	"time"

	"gorm.io/gorm"
)

type Notification struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	UserID      uint           `gorm:"index" json:"user_id"`
	User        User           `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Type        string         `gorm:"type:varchar(50)" json:"type" validate:"oneof=like comment follow system"`
	Content     string         `gorm:"type:text" json:"content"`
	IsRead      bool           `gorm:"default:false" json:"is_read"`
	ReferenceID uint           `json:"reference_id"` // ID des Objekts, auf das sich die Benachrichtigung bezieht
	CreatedAt   time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// MarkAsRead markiert eine Benachrichtigung als gelesen
func (n *Notification) MarkAsRead(db *gorm.DB) error {
	n.IsRead = true
	return db.Model(n).Update("is_read", true).Error
}

// CreateNotification erstellt eine neue Benachrichtigung
func CreateNotification(db *gorm.DB, userID uint, notificationType string, content string, referenceID uint) error {
	notification := Notification{
		UserID:      userID,
		Type:        notificationType,
		Content:     content,
		ReferenceID: referenceID,
		IsRead:      false,
	}

	return db.Create(&notification).Error
}
