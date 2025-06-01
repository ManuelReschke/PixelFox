package models

import (
	"time"

	"gorm.io/gorm"
)

// News represents a news article in the system
type News struct {
	ID        uint64         `gorm:"primaryKey" json:"id"`
	Title     string         `gorm:"type:varchar(255)" json:"title" validate:"required,min=3,max=255"`
	Content   string         `gorm:"type:text" json:"content" validate:"required"`
	Slug      string         `gorm:"uniqueIndex;type:varchar(255)" json:"slug" validate:"required,min=3,max=255"`
	Published bool           `gorm:"type:tinyint(1);default:0" json:"published"`
	UserID    uint64         `gorm:"index" json:"user_id"`
	User      User           `gorm:"foreignKey:UserID" json:"user"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name for the News model
func (News) TableName() string {
	return "news"
}
