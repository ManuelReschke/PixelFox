package jobqueue

import (
	"sync"
	"time"

	"github.com/gofiber/fiber/v2/log"
)

// Manager manages the global job queue and background tasks
type Manager struct {
	queue       *Queue
	retryTicker *time.Ticker
	stopCh      chan struct{}
	wg          sync.WaitGroup
	mu          sync.Mutex
	running     bool
}

var (
	globalManager *Manager
	managerOnce   sync.Once
)

// GetManager returns the global job queue manager (singleton)
func GetManager() *Manager {
	managerOnce.Do(func() {
		globalManager = &Manager{
			queue:  NewQueue(3), // 3 workers for backup jobs
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

	// Start retry mechanism - every 2 minutes
	m.retryTicker = time.NewTicker(2 * time.Minute)
	m.wg.Add(1)
	go m.retryWorker()

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
	log.Info("[JobQueue Manager] Started retry worker (interval: 2 minutes)")

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

// IsRunning returns whether the manager is currently running
func (m *Manager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}
