package jobqueue

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2/log"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/imageprocessor"
)

// runTieringSweepOnce scans hot pools and enqueues move jobs for inactive images based on admin settings.
func (m *Manager) runTieringSweepOnce() error {
	settings := getAppSettings()
	if settings == nil || !settings.IsTieringEnabled() {
		return nil
	}
	db := database.GetDB()
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}

	hotPools, err := models.FindHotStoragePools(db)
	if err != nil {
		return fmt.Errorf("failed to list hot pools: %w", err)
	}
	if len(hotPools) == 0 {
		return nil
	}

	// Load warm pools once for target selection
	warmPools, _ := models.FindActiveStoragePoolsByTier(db, models.StorageTierWarm)
	// fallback: if no warm, try cold
	coldPools, _ := models.FindActiveStoragePoolsByTier(db, models.StorageTierCold)

	maxBatch := settings.GetMaxTieringCandidatesPerSweep()
	if maxBatch <= 0 {
		maxBatch = 200
	}
	demoted := 0

	keepDays := settings.GetHotKeepDaysAfterUpload()
	noViewsDays := settings.GetDemoteIfNoViewsDays()
	high := settings.GetHotWatermarkHigh()

	now := time.Now()

	for _, pool := range hotPools {
		if demoted >= maxBatch {
			break
		}

		// Refresh stats to get up-to-date usage
		stats, serr := models.GetStoragePoolStats(db, pool.ID)
		if serr != nil {
			log.Errorf("[Tiering] stats error for pool %s: %v", pool.Name, serr)
			continue
		}
		usage := int(stats.UsagePercentage + 0.5)
		capacityPressure := usage >= high

		// Build candidate query for this pool
		// Criteria: (now - COALESCE(last_viewed_at, created_at)) >= noViewsDays AND (now - created_at) >= keepDays
		// Order by oldest last activity first
		type simpleImage struct {
			ID            uint
			UUID          string
			FileSize      int64
			StoragePoolID uint
		}
		var imgs []simpleImage
		limit := maxBatch - demoted
		q := db.
			Table("images").
			Select("id, uuid, file_size, storage_pool_id").
			Where("storage_pool_id = ?", pool.ID).
			Where("deleted_at IS NULL").
			Where("TIMESTAMPDIFF(DAY, COALESCE(last_viewed_at, created_at), ?) >= ?", now, noViewsDays).
			Where("TIMESTAMPDIFF(DAY, created_at, ?) >= ?", now, keepDays).
			Order("COALESCE(last_viewed_at, created_at) ASC, id ASC").
			Limit(limit)
		if err := q.Scan(&imgs).Error; err != nil {
			log.Errorf("[Tiering] candidate scan error for pool %s: %v", pool.Name, err)
			continue
		}
		if len(imgs) == 0 {
			// No inactivity candidates; if capacity pressure, we could demote by age only (optional).
			continue
		}

		for _, si := range imgs {
			if demoted >= maxBatch {
				break
			}

			// Skip images that are still processing (race safety against moving mid‑processing)
			if si.UUID == "" || !imageprocessor.IsImageProcessingComplete(si.UUID) {
				continue
			}

			// pick target pool: warm first, else cold
			var target *models.StoragePool
			for i := range warmPools {
				if warmPools[i].CanAcceptFile(si.FileSize) {
					target = &warmPools[i]
					break
				}
			}
			if target == nil && len(coldPools) > 0 {
				for i := range coldPools {
					if coldPools[i].CanAcceptFile(si.FileSize) {
						target = &coldPools[i]
						break
					}
				}
			}
			if target == nil {
				// no capacity; skip
				continue
			}

			payload := MoveImageJobPayload{ImageID: si.ID, SourcePoolID: pool.ID, TargetPoolID: target.ID}
			if _, err := m.queue.EnqueueJob(JobTypeMoveImage, payload.ToMap()); err != nil {
				log.Errorf("[Tiering] enqueue move failed: img=%d pool=%s->%s err=%v", si.ID, pool.Name, target.Name, err)
				continue
			}
			demoted++

			// Optional: if we are under capacity pressure, keep going; else we can stop early after a few
			if !capacityPressure && demoted >= maxBatch {
				break
			}
		}

		// Optional hysteresis handling: if usage < low, we could stop early; here we rely on next sweep
	}

	if demoted > 0 {
		log.Infof("[Tiering] Demoted %d images in this sweep", demoted)
	}
	return nil
}
