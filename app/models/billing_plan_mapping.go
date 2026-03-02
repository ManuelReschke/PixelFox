package models

import "time"

// BillingPlanMapping maps provider-specific plan references (tier/price IDs)
// to internal entitlement plans.
type BillingPlanMapping struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	Provider        string    `gorm:"type:varchar(20);not null;index:ux_billing_plan_mappings_ref,unique,priority:1;index" json:"provider"`
	ProviderPlanRef string    `gorm:"type:varchar(191);not null;index:ux_billing_plan_mappings_ref,unique,priority:2" json:"provider_plan_ref"`
	InternalPlan    string    `gorm:"type:varchar(50);not null;default:'free';index" json:"internal_plan"`
	BillingInterval string    `gorm:"type:varchar(16);not null;default:'unknown';index:ux_billing_plan_mappings_ref,unique,priority:3" json:"billing_interval"`
	IsActive        bool      `gorm:"default:true;index" json:"is_active"`
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}
