package models

import "time"

// Billing provider constants used across billing-related models.
const (
	BillingProviderPatreon = "patreon"
	BillingProviderStripe  = "stripe"
)

// BillingAccount stores a user's linked billing identity per provider.
type BillingAccount struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	UserID            uint       `gorm:"not null;index:ux_billing_accounts_user_provider,unique" json:"user_id"`
	Provider          string     `gorm:"type:varchar(20);not null;index:ux_billing_accounts_user_provider,unique;index:ux_billing_accounts_provider_account,unique,priority:1" json:"provider"`
	ProviderAccountID string     `gorm:"type:varchar(191);not null;index:ux_billing_accounts_provider_account,unique,priority:2" json:"provider_account_id"`
	Email             string     `gorm:"type:varchar(200);default:''" json:"email"`
	AccessTokenEnc    string     `gorm:"type:text" json:"-"`
	RefreshTokenEnc   string     `gorm:"type:text" json:"-"`
	TokenExpiresAt    *time.Time `gorm:"type:timestamp;default:null" json:"token_expires_at,omitempty"`
	CreatedAt         time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}
