package storage

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/cache"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
	"github.com/gofiber/fiber/v2/log"
)

// health monitor state
var (
	healthStopCh chan struct{}
)

// PoolHealth represents cached health data for a storage pool
type PoolHealth struct {
	PoolID             uint      `json:"pool_id"`
	Healthy            bool      `json:"healthy"`
	UploadAPIReachable bool      `json:"upload_api_reachable"`
	UsedSize           int64     `json:"used_size"`
	MaxSize            int64     `json:"max_size"`
	UsagePercent       float64   `json:"usage_percent"`
	AvailableSize      int64     `json:"available_size"`
	CheckedAt          time.Time `json:"checked_at"`
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
		// check upload api reachability (best-effort)
		reachable := false
		if u := strings.TrimSpace(pool.UploadAPIURL); u != "" && (strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://")) {
			client := &http.Client{Timeout: 2 * time.Second}
			// Try OPTIONS first (CORS preflight should be handled), then HEAD
			req, _ := http.NewRequest("OPTIONS", u, nil)
			if resp, err := client.Do(req); err == nil {
				if resp.StatusCode >= 200 && resp.StatusCode < 500 { // any response means reachable
					reachable = true
				}
			} else {
				// fallback to HEAD
				req2, _ := http.NewRequest("HEAD", u, nil)
				if resp2, err2 := client.Do(req2); err2 == nil {
					if resp2.StatusCode >= 200 && resp2.StatusCode < 500 {
						reachable = true
					}
				}
			}
		}
		// Dev fallback: if still not reachable and this is the local node, try hitting localhost:APP_PORT
		if !reachable && strings.EqualFold(strings.TrimSpace(pool.NodeID), "local") {
			// avoid marking remote pools as reachable by accident
			internalURL := fmt.Sprintf("http://localhost:%s/api/internal/upload", strings.TrimSpace(env.GetEnv("APP_PORT", "4000")))
			client := &http.Client{Timeout: 2 * time.Second}
			if req, _ := http.NewRequest("HEAD", internalURL, nil); req != nil {
				if resp, err := client.Do(req); err == nil && resp.StatusCode >= 200 && resp.StatusCode < 500 {
					reachable = true
				}
			}
		}
		ph := PoolHealth{
			PoolID:             pool.ID,
			Healthy:            healthy,
			UploadAPIReachable: reachable,
			UsedSize:           stats.UsedSize,
			MaxSize:            stats.MaxSize,
			UsagePercent:       stats.UsagePercentage,
			AvailableSize:      stats.AvailableSize,
			CheckedAt:          time.Now(),
		}

		b, _ := json.Marshal(ph)
		key := fmt.Sprintf("storage_health:%d", pool.ID)
		if err := cache.Set(key, string(b), 2*time.Minute); err != nil {
			log.Errorf("[StorageHealth] Cache set failed for pool %s: %v", pool.Name, err)
		}
	}
}
