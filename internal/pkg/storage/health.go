package storage

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/cache"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/gofiber/fiber/v2/log"
)

// health monitor state
var (
	healthStopCh chan struct{}
)

// PoolHealth represents cached health data for a storage pool
type PoolHealth struct {
	PoolID        uint      `json:"pool_id"`
	Healthy       bool      `json:"healthy"`
	UsedSize      int64     `json:"used_size"`
	MaxSize       int64     `json:"max_size"`
	UsagePercent  float64   `json:"usage_percent"`
	AvailableSize int64     `json:"available_size"`
	CheckedAt     time.Time `json:"checked_at"`
}

// StartHealthMonitor starts a lightweight heartbeat that caches pool health in Redis
func StartHealthMonitor() {
	if healthStopCh != nil {
		return
	}
	healthStopCh = make(chan struct{})
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		log.Info("[StorageHealth] Monitor started (interval: 60s)")

		// run once immediately
		runHealthCheckOnce()

		for {
			select {
			case <-healthStopCh:
				log.Info("[StorageHealth] Monitor stopped")
				return
			case <-ticker.C:
				runHealthCheckOnce()
			}
		}
	}()
}

// StopHealthMonitor stops the heartbeat
func StopHealthMonitor() {
	if healthStopCh != nil {
		close(healthStopCh)
		healthStopCh = nil
	}
}

func runHealthCheckOnce() {
	db := database.GetDB()
	if db == nil {
		return
	}
	pools, err := models.FindAllStoragePools(db)
	if err != nil {
		log.Errorf("[StorageHealth] Failed to load storage pools: %v", err)
		return
	}

	for _, pool := range pools {
		// Use existing stats helper to refresh usage figures
		stats, err := models.GetStoragePoolStats(db, pool.ID)
		if err != nil {
			log.Errorf("[StorageHealth] Stats error for pool %s: %v", pool.Name, err)
			continue
		}

		healthy := pool.IsHealthy()
		ph := PoolHealth{
			PoolID:        pool.ID,
			Healthy:       healthy,
			UsedSize:      stats.UsedSize,
			MaxSize:       stats.MaxSize,
			UsagePercent:  stats.UsagePercentage,
			AvailableSize: stats.AvailableSize,
			CheckedAt:     time.Now(),
		}

		b, _ := json.Marshal(ph)
		key := fmt.Sprintf("storage_health:%d", pool.ID)
		if err := cache.Set(key, string(b), 2*time.Minute); err != nil {
			log.Errorf("[StorageHealth] Cache set failed for pool %s: %v", pool.Name, err)
		}
	}
}
