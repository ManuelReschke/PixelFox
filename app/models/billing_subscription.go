package models

import "time"

const (
	BillingIntervalMonth   = "month"
	BillingIntervalYear    = "year"
	BillingIntervalUnknown = "unknown"
)

const (
	BillingStatusActive     = "active"
	BillingStatusTrialing   = "trialing"
	BillingStatusPastDue    = "past_due"
	BillingStatusCanceled   = "canceled"
	BillingStatusIncomplete = "incomplete"
	BillingStatusExpired    = "expired"
	BillingStatusPaused     = "paused"
)

// BillingSubscription mirrors a provider subscription/member state and maps it
// to an internal plan used by entitlements.
type BillingSubscription struct {
	ID                     uint       `gorm:"primaryKey" json:"id"`
	UserID                 uint       `gorm:"not null;index" json:"user_id"`
	Provider               string     `gorm:"type:varchar(20);not null;index:idx_billing_subscriptions_provider_status,priority:1;index:ux_billing_subscriptions_provider_subid,unique,priority:1" json:"provider"`
	ProviderSubscriptionID string     `gorm:"type:varchar(191);not null;index:ux_billing_subscriptions_provider_subid,unique,priority:2" json:"provider_subscription_id"`
	ProviderPlanRef        string     `gorm:"type:varchar(191);not null;index" json:"provider_plan_ref"`
	InternalPlan           string     `gorm:"type:varchar(50);not null;default:'free';index" json:"internal_plan"`
	BillingInterval        string     `gorm:"type:varchar(16);not null;default:'unknown'" json:"billing_interval"`
	Status                 string     `gorm:"type:varchar(32);not null;default:'active';index:idx_billing_subscriptions_provider_status,priority:2" json:"status"`
	CurrentPeriodStart     *time.Time `gorm:"type:timestamp;default:null" json:"current_period_start,omitempty"`
	CurrentPeriodEnd       *time.Time `gorm:"type:timestamp;default:null" json:"current_period_end,omitempty"`
	CancelAtPeriodEnd      bool       `gorm:"default:false" json:"cancel_at_period_end"`
	RawPayloadJSON         string     `gorm:"type:longtext" json:"raw_payload_json"`
	CreatedAt              time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt              time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}
