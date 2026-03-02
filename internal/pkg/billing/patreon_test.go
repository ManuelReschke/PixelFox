package billing

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/ManuelReschke/PixelFox/app/models"
)

func TestPatreonStatusToBillingStatus(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "active_patron", want: models.BillingStatusActive},
		{in: "declined_patron", want: models.BillingStatusPastDue},
		{in: "former_patron", want: models.BillingStatusCanceled},
		{in: "something_else", want: models.BillingStatusIncomplete},
	}

	for _, tt := range tests {
		if got := PatreonStatusToBillingStatus(tt.in); got != tt.want {
			t.Fatalf("PatreonStatusToBillingStatus(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestPatreonMembershipToBillingStatus_EmptyStatus(t *testing.T) {
	if got := PatreonMembershipToBillingStatus("", false); got != models.BillingStatusActive {
		t.Fatalf("expected empty status + non-follower to be active, got %q", got)
	}
	if got := PatreonMembershipToBillingStatus("", true); got != models.BillingStatusIncomplete {
		t.Fatalf("expected empty status + follower to be incomplete, got %q", got)
	}
}

func TestVerifyPatreonWebhookSignature(t *testing.T) {
	payload := []byte(`{"foo":"bar"}`)
	secret := "top-secret"

	mac := hmac.New(md5.New, []byte(secret))
	mac.Write(payload)
	validSig := hex.EncodeToString(mac.Sum(nil))

	if !VerifyPatreonWebhookSignature(payload, validSig, secret) {
		t.Fatalf("expected signature to validate")
	}

	macSHA256 := hmac.New(sha256.New, []byte(secret))
	macSHA256.Write(payload)
	validSHA256 := hex.EncodeToString(macSHA256.Sum(nil))
	if !VerifyPatreonWebhookSignature(payload, validSHA256, secret) {
		t.Fatalf("expected sha256 fallback signature to validate")
	}
	if VerifyPatreonWebhookSignature(payload, "deadbeef", secret) {
		t.Fatalf("expected invalid signature to fail")
	}
}

func TestParsePatreonWebhookMemberEvent(t *testing.T) {
	raw := []byte(`{
		"data": {
			"id": "m_123",
			"type": "member",
			"attributes": { "patron_status": "active_patron", "is_follower": false },
			"relationships": {
				"user": { "data": { "id": "u_456", "type": "user" } },
				"currently_entitled_tiers": {
					"data": [
						{ "id": "tier_a", "type": "tier" },
						{ "id": "tier_b", "type": "tier" }
					]
				}
			}
		}
	}`)

	ev, err := ParsePatreonWebhookMemberEvent(raw)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if ev.MemberID != "m_123" || ev.PatreonUserID != "u_456" {
		t.Fatalf("unexpected ids: member=%q user=%q", ev.MemberID, ev.PatreonUserID)
	}
	if ev.IsFollower {
		t.Fatalf("expected is_follower=false")
	}
	if len(ev.TierIDs) != 2 {
		t.Fatalf("expected 2 tiers, got %d", len(ev.TierIDs))
	}
}
