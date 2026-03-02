package billing

import "time"

// NormalizedSubscription is the provider-agnostic shape used by the billing
// service when syncing external subscription state into local tables.
type NormalizedSubscription struct {
	UserID                 uint
	Provider               string
	ProviderSubscriptionID string
	ProviderPlanRef        string
	BillingInterval        string
	Status                 string
	CurrentPeriodStart     *time.Time
	CurrentPeriodEnd       *time.Time
	CancelAtPeriodEnd      bool
	RawPayloadJSON         string
}

// WebhookEventInput is the normalized input for webhook event persistence.
type WebhookEventInput struct {
	Provider        string
	ProviderEventID string
	EventType       string
	PayloadJSON     string
	SignatureValid  bool
}
