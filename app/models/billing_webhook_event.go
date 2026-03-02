package models

import "time"

// BillingWebhookEvent stores provider webhook payloads with deduplication
// metadata for idempotent processing.
type BillingWebhookEvent struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	Provider        string     `gorm:"type:varchar(20);not null;index:ux_billing_webhook_events_provider_event,unique,priority:1;index" json:"provider"`
	ProviderEventID string     `gorm:"type:varchar(191);not null;default:'';index:ux_billing_webhook_events_provider_event,unique,priority:2" json:"provider_event_id"`
	EventType       string     `gorm:"type:varchar(100);not null;index" json:"event_type"`
	PayloadJSON     string     `gorm:"type:longtext;not null" json:"payload_json"`
	SignatureValid  bool       `gorm:"default:false;index" json:"signature_valid"`
	ProcessedAt     *time.Time `gorm:"type:timestamp;default:null" json:"processed_at,omitempty"`
	ProcessingError string     `gorm:"type:text" json:"processing_error"`
	CreatedAt       time.Time  `gorm:"autoCreateTime;index" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}
