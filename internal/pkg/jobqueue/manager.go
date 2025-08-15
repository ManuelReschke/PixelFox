package jobqueue

import (
	"sync"
	"time"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/gofiber/fiber/v2/log"
)

// Manager manages the global job queue and background tasks
type Manager struct {
	queue               *Queue
	retryTicker         *time.Ticker
	delayedBackupTicker *time.Ticker
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

	// Signal workers to stop
	close(m.stopCh)
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
