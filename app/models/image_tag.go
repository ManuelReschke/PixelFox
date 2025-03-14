package models

import "time"

type ImageTag struct {
	ImageID   uint      `gorm:"primaryKey;autoIncrement:false" json:"image_id"`
	TagID     uint      `gorm:"primaryKey;autoIncrement:false" json:"tag_id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}
