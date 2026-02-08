package jobqueue

import (
	"sync"
	"time"

	"github.com/ManuelReschke/PixelFox/app/models"
	metrics "github.com/ManuelReschke/PixelFox/internal/pkg/metrics/counter"
	"github.com/gofiber/fiber/v2/log"
)

// Manager manages the global job queue and background tasks
type Manager struct {
	queue               *Queue
	retryTicker         *time.Ticker
	delayedBackupTicker *time.Ticker
	counterFlushTicker  *time.Ticker
	tieringTicker       *time.Ticker
	stopCh              chan struct{}
	wg                  sync.WaitGroup
	mu                  sync.Mutex
	running             bool
}

var (
	globalManager *Manager
	managerOnce   sync.Once
)

// GetManager returns the global job queue manager (singleton)
func GetManager() *Manager {
	managerOnce.Do(func() {
		// Get worker count from settings, fallback to 5 if not available
		workerCount := 5
		if settings := getAppSettings(); settings != nil {
			workerCount = settings.GetJobQueueWorkerCount()
		}

		globalManager = &Manager{
			queue:  NewQueue(workerCount), // Configurable workers for image processing + backup jobs (unified)
			stopCh: make(chan struct{}),
		}
	})
	return globalManager
}

// GetQueue returns the managed job queue
func (m *Manager) GetQueue() *Queue {
	return m.queue
}

// Start starts the job queue and background tasks
func (m *Manager) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return
	}

	// Recreate stop channel for each start cycle so manager can be restarted safely.
	m.stopCh = make(chan struct{})
	m.running = true
	log.Info("[JobQueue Manager] Starting job queue and background tasks")

	// Start the job queue
	m.queue.Start()

	// Get intervals from settings
	retryInterval := 2 * time.Minute // Default fallback
	checkInterval := 5 * time.Minute // Default fallback
	if settings := getAppSettings(); settings != nil {
		retryInterval = time.Duration(settings.GetS3RetryInterval()) * time.Minute
		checkInterval = time.Duration(settings.GetS3BackupCheckInterval()) * time.Minute
	}

	// Start retry mechanism - configurable interval
	m.retryTicker = time.NewTicker(retryInterval)
	m.wg.Add(1)
	go m.retryWorker()

	// Start delayed backup processing - configurable interval
	m.delayedBackupTicker = time.NewTicker(checkInterval)
	m.wg.Add(1)
	go m.delayedBackupWorker()

	// Start counter flush worker (Redis -> DB) every 5 seconds
	m.counterFlushTicker = time.NewTicker(5 * time.Second)
	m.wg.Add(1)
	go m.counterFlushWorker()

	// Tiering sweeper (Phase A)
	tieringInterval := 15 * time.Minute
	if settings := getAppSettings(); settings != nil {
		if v := settings.GetTieringSweepIntervalMinutes(); v > 0 {
			tieringInterval = time.Duration(v) * time.Minute
		}
	}
	m.tieringTicker = time.NewTicker(tieringInterval)
	m.wg.Add(1)
	go m.tieringWorker()

	log.Info("[JobQueue Manager] Started successfully")
}

// Stop stops the job queue and background tasks
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	log.Info("[JobQueue Manager] Stopping job queue and background tasks...")

	// Stop retry ticker
	if m.retryTicker != nil {
		m.retryTicker.Stop()
	}

	// Stop delayed backup ticker
	if m.delayedBackupTicker != nil {
		m.delayedBackupTicker.Stop()
	}

	if m.counterFlushTicker != nil {
		m.counterFlushTicker.Stop()
	}
	if m.tieringTicker != nil {
		m.tieringTicker.Stop()
	}

	// Signal workers to stop
	close(m.stopCh)
	m.stopCh = nil
	m.running = false

	// Wait for background workers to finish
	m.wg.Wait()

	// Stop the job queue
	m.queue.Stop()

	log.Info("[JobQueue Manager] Stopped successfully")
}

// retryWorker runs periodically to retry failed S3 backups
func (m *Manager) retryWorker() {
	defer m.wg.Done()
	interval := 2 // Default fallback
	if settings := getAppSettings(); settings != nil {
		interval = settings.GetS3RetryInterval()
	}
	log.Infof("[JobQueue Manager] Started retry worker (interval: %d minutes)", interval)

	for {
		select {
		case <-m.stopCh:
			log.Info("[JobQueue Manager] Retry worker stopping")
			return
		case <-m.retryTicker.C:
			log.Debug("[JobQueue Manager] Running retry check for failed S3 backups")
			if err := m.queue.RetryFailedS3Backups(); err != nil {
				log.Errorf("[JobQueue Manager] Error retrying failed S3 backups: %v", err)
			}
		}
	}
}

// delayedBackupWorker runs periodically to process delayed S3 backups
func (m *Manager) delayedBackupWorker() {
	defer m.wg.Done()
	interval := 5 // Default fallback
	if settings := getAppSettings(); settings != nil {
		interval = settings.GetS3BackupCheckInterval()
	}
	log.Infof("[JobQueue Manager] Started delayed backup worker (interval: %d minutes)", interval)

	for {
		select {
		case <-m.stopCh:
			log.Info("[JobQueue Manager] Delayed backup worker stopping")
			return
		case <-m.delayedBackupTicker.C:
			log.Debug("[JobQueue Manager] Running delayed backup processing")
			if err := m.queue.ProcessDelayedS3Backups(); err != nil {
				log.Errorf("[JobQueue Manager] Error processing delayed S3 backups: %v", err)
			}
		}
	}
}

// counterFlushWorker periodically flushes in-memory counters from Redis to DB
func (m *Manager) counterFlushWorker() {
	defer m.wg.Done()
	for {
		select {
		case <-m.stopCh:
			log.Info("[JobQueue Manager] Counter flush worker stopping")
			return
		case <-m.counterFlushTicker.C:
			if err := m.flushCountersOnce(); err != nil {
				log.Errorf("[JobQueue Manager] Counter flush error: %v", err)
			}
		}
	}
}

// tieringWorker periodically runs Phase A tiering sweep to demote inactive images from hot to warm/cold.
func (m *Manager) tieringWorker() {
	defer m.wg.Done()
	for {
		select {
		case <-m.stopCh:
			log.Info("[JobQueue Manager] Tiering worker stopping")
			return
		case <-m.tieringTicker.C:
			if err := m.runTieringSweepOnce(); err != nil {
				log.Errorf("[JobQueue Manager] Tiering sweep error: %v", err)
			}
		}
	}
}

func (m *Manager) flushCountersOnce() error {
	// Flush Redis -> DB (batched CASE update)
	return metrics.FlushAll()
}

// IsRunning returns whether the manager is currently running
func (m *Manager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

// getAppSettings safely returns the current app settings
func getAppSettings() *models.AppSettings {
	return models.GetAppSettings()
}

// RunTieringSweepOnce exposes a manual trigger for a single tiering sweep (admin use).
func (m *Manager) RunTieringSweepOnce() error {
	return m.runTieringSweepOnce()
}
