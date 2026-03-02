package billing

import (
	"strings"

	"github.com/ManuelReschke/PixelFox/internal/pkg/entitlements"
)

func normalizePlan(plan string) string {
	switch strings.ToLower(strings.TrimSpace(plan)) {
	case string(entitlements.PlanPremium):
		return string(entitlements.PlanPremium)
	case string(entitlements.PlanPremiumMax):
		return string(entitlements.PlanPremiumMax)
	default:
		return string(entitlements.PlanFree)
	}
}

func planRank(plan string) int {
	switch normalizePlan(plan) {
	case string(entitlements.PlanPremiumMax):
		return 2
	case string(entitlements.PlanPremium):
		return 1
	default:
		return 0
	}
}

func normalizeInterval(interval string) string {
	i := strings.ToLower(strings.TrimSpace(interval))
	switch i {
	case "month", "year":
		return i
	default:
		return "unknown"
	}
}

func isEntitlingStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "active", "trialing", "past_due":
		return true
	default:
		return false
	}
}
