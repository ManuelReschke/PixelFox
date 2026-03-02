package billing

import "testing"

func TestNormalizePlan(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "free", want: "free"},
		{in: "premium", want: "premium"},
		{in: "premium_max", want: "premium_max"},
		{in: "PREMIUM_MAX", want: "premium_max"},
		{in: "invalid", want: "free"},
	}

	for _, tt := range tests {
		if got := normalizePlan(tt.in); got != tt.want {
			t.Fatalf("normalizePlan(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestPlanRank(t *testing.T) {
	if planRank("free") >= planRank("premium") {
		t.Fatalf("expected premium to outrank free")
	}
	if planRank("premium") >= planRank("premium_max") {
		t.Fatalf("expected premium_max to outrank premium")
	}
}

func TestIsEntitlingStatus(t *testing.T) {
	for _, status := range []string{"active", "trialing", "past_due"} {
		if !isEntitlingStatus(status) {
			t.Fatalf("expected status %q to be entitling", status)
		}
	}
	for _, status := range []string{"canceled", "incomplete", "expired", "paused"} {
		if isEntitlingStatus(status) {
			t.Fatalf("expected status %q to be non-entitling", status)
		}
	}
}
