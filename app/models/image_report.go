package models

import (
	"time"

	"gorm.io/gorm"
)

const (
	ReportStatusOpen      = "open"
	ReportStatusResolved  = "resolved"
	ReportStatusDismissed = "dismissed"
)

type ImageReport struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	ImageID      uint           `gorm:"index;not null" json:"image_id"`
	Image        *Image         `gorm:"foreignKey:ImageID" json:"image,omitempty"`
	ReporterID   *uint          `gorm:"index" json:"reporter_id,omitempty"`
	Reporter     *User          `gorm:"foreignKey:ReporterID" json:"reporter,omitempty"`
	Reason       string         `gorm:"type:varchar(50);not null" json:"reason"`
	Details      string         `gorm:"type:text" json:"details"`
	Status       string         `gorm:"type:varchar(20);default:'open'" json:"status"`
	ReporterIPv4 string         `gorm:"type:varchar(15);default:null" json:"-"`
	ReporterIPv6 string         `gorm:"type:varchar(45);default:null" json:"-"`
	ResolvedByID *uint          `gorm:"index" json:"resolved_by_id,omitempty"`
	ResolvedBy   *User          `gorm:"foreignKey:ResolvedByID" json:"resolved_by,omitempty"`
	ResolvedAt   *time.Time     `json:"resolved_at,omitempty"`
	CreatedAt    time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}
