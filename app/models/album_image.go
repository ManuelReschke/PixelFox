package models

import "time"

type AlbumImage struct {
	AlbumID   uint      `gorm:"primaryKey;autoIncrement:false" json:"album_id"`
	ImageID   uint      `gorm:"primaryKey;autoIncrement:false" json:"image_id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}
