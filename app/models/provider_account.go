package models

import "time"

// ProviderAccount stores external OAuth provider identities linked to a user
type ProviderAccount struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	UserID         uint       `gorm:"index" json:"user_id"`
	Provider       string     `gorm:"index:provider_uid,unique;type:varchar(50)" json:"provider"`
	ProviderUserID string     `gorm:"index:provider_uid,unique;type:varchar(191)" json:"provider_user_id"`
	AccessToken    string     `gorm:"type:text" json:"-"`
	RefreshToken   string     `gorm:"type:text" json:"-"`
	ExpiresAt      *time.Time `gorm:"type:timestamp;default:null" json:"expires_at,omitempty"`
	CreatedAt      time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}
