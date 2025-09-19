package entitlements

import (
	"strings"

	"github.com/ManuelReschke/PixelFox/app/models"
)

type Plan string

const (
	PlanFree       Plan = "free"
	PlanPremium    Plan = "premium"
	PlanPremiumMax Plan = "premium_max"
)

// AllowedThumbs returns which thumbnail/fullsize optimized formats are allowed for a given plan
func AllowedThumbs(plan Plan) (orig, webp, avif bool) {
	switch plan {
	case PlanPremiumMax:
		return true, true, true
	case PlanPremium:
		return true, true, false
	default:
		return true, false, false
	}
}

// EffectiveThumbs combines admin settings, user plan and user preferences
// to compute final booleans for generating Original/WebP/AVIF variants.
func EffectiveThumbs(us *models.UserSettings, app *models.AppSettings) (orig, webp, avif bool) {
	// Admin global toggles
	adminOrig := app != nil && app.IsThumbnailOriginalEnabled()
	adminWebp := app != nil && app.IsThumbnailWebPEnabled()
	adminAvif := app != nil && app.IsThumbnailAVIFEnabled()

	// Plan allowances
	p := Plan(strings.ToLower(us.Plan))
	allowOrig, allowWebp, allowAvif := AllowedThumbs(p)

	// User preferences
	prefOrig := us.PrefThumbOriginal
	prefWebp := us.PrefThumbWebP
	prefAvif := us.PrefThumbAVIF

	return adminOrig && allowOrig && prefOrig,
		adminWebp && allowWebp && prefWebp,
		adminAvif && allowAvif && prefAvif
}
