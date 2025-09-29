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

// AlbumLimit returns the allowed number of albums for a given plan.
// -1 means unlimited.
func AlbumLimit(plan Plan) int {
	switch plan {
	case PlanPremiumMax:
		return -1
	case PlanPremium:
		return 50
	default:
		return 5
	}
}

// CanCreateAlbum decides if a user with given plan and current album count may create another album.
func CanCreateAlbum(plan Plan, currentCount int) bool {
	limit := AlbumLimit(plan)
	if limit < 0 {
		return true
	}
	return currentCount < limit
}

// MaxUploadBytes returns the maximum allowed upload size in bytes for a plan.
// Free: 5 MiB, Premium: 50 MiB, Premium Max: 100 MiB
func MaxUploadBytes(plan Plan) int64 {
	const MiB = 1024 * 1024
	switch plan {
	case PlanPremiumMax:
		return 100 * MiB
	case PlanPremium:
		return 50 * MiB
	default:
		return 5 * MiB
	}
}

// StorageQuotaBytes returns the total storage quota in bytes per plan.
// -1 means unlimited.
func StorageQuotaBytes(plan Plan) int64 {
	const GiB = int64(1024 * 1024 * 1024)
	switch plan {
	case PlanPremiumMax:
		return 2000 * GiB // 2 TB
	case PlanPremium:
		return 500 * GiB // 500 GB
	default:
		return 25 * GiB
	}
}

// CanMultiUpload returns whether a plan supports selecting and uploading
// multiple files in one batch from the UI.
func CanMultiUpload(plan Plan) bool {
	switch plan {
	case PlanPremium, PlanPremiumMax:
		return true
	default:
		return false
	}
}

// MaxFilesPerBatch returns the maximum number of files allowed per batch
// upload for a given plan. -1 means unlimited (not recommended in UI).
func MaxFilesPerBatch(plan Plan) int {
	switch plan {
	case PlanPremiumMax:
		return 50
	case PlanPremium:
		return 25
	default:
		return 1
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
