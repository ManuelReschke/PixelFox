package jobqueue

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetManager(t *testing.T) {
	// Reset the singleton for testing
	globalManager = nil
	managerOnce = sync.Once{}

	// Test singleton behavior
	manager1 := GetManager()
	manager2 := GetManager()

	assert.NotNil(t, manager1)
	assert.Same(t, manager1, manager2, "GetManager should return the same instance")

	// Test initial state
	assert.NotNil(t, manager1.queue)
	assert.NotNil(t, manager1.stopCh)
	assert.False(t, manager1.running)
	assert.Nil(t, manager1.retryTicker)
}

func TestManager_GetQueue(t *testing.T) {
	// Reset the singleton for testing
	globalManager = nil
	managerOnce = sync.Once{}

	manager := GetManager()
	queue := manager.GetQueue()

	assert.NotNil(t, queue)
	assert.Same(t, manager.queue, queue)
}

func TestManager_IsRunning(t *testing.T) {
	// Reset the singleton for testing
	globalManager = nil
	managerOnce = sync.Once{}

	manager := GetManager()

	// Initial state should be not running
	assert.False(t, manager.IsRunning())

	// Manually set running state to test the method
	manager.mu.Lock()
	manager.running = true
	manager.mu.Unlock()

	assert.True(t, manager.IsRunning())

	// Reset running state
	manager.mu.Lock()
	manager.running = false
	manager.mu.Unlock()

	assert.False(t, manager.IsRunning())
}

func TestManager_StartStop(t *testing.T) {
	// Skip this test as it requires Redis connection and workers hang
	t.Skip("Skipping integration test - workers block on Redis operations")
}

func TestManager_StartStopConcurrency(t *testing.T) {
	// Skip this test as it requires Redis connection and workers hang
	t.Skip("Skipping integration test - workers block on Redis operations")
}

func TestManager_RetryWorkerLifecycle(t *testing.T) {
	// Skip this test as it requires Redis connection and workers hang
	t.Skip("Skipping integration test - workers block on Redis operations")
}

func TestManager_RetryWorkerTickerInterval(t *testing.T) {
	// Skip this test as it requires Redis connection and workers hang
	t.Skip("Skipping integration test - workers block on Redis operations")
}

func TestManager_StopWithoutStart(t *testing.T) {
	// Reset the singleton for testing
	globalManager = nil
	managerOnce = sync.Once{}

	manager := GetManager()

	// Stop without starting should be safe
	assert.False(t, manager.IsRunning())
	manager.Stop()
	assert.False(t, manager.IsRunning())
}

func TestManager_MultipleStartStop(t *testing.T) {
	// Skip this test as it requires Redis connection and workers hang
	t.Skip("Skipping integration test - workers block on Redis operations")
}

func TestManager_ThreadSafety(t *testing.T) {
	// Skip this test as it requires Redis connection and workers hang
	t.Skip("Skipping integration test - workers block on Redis operations")
}

func TestNewManagerStructure(t *testing.T) {
	// Reset the singleton for testing
	globalManager = nil
	managerOnce = sync.Once{}

	manager := GetManager()

	// Verify internal structure
	assert.NotNil(t, manager.queue)
	assert.NotNil(t, manager.stopCh)
	assert.False(t, manager.running)
	assert.Nil(t, manager.retryTicker)

	// Verify queue has correct number of workers
	assert.Equal(t, 5, manager.queue.workers)
}

func TestManagerSingletonReset(t *testing.T) {
	// Get first instance
	globalManager = nil
	managerOnce = sync.Once{}
	manager1 := GetManager()

	// Reset and get second instance
	globalManager = nil
	managerOnce = sync.Once{}
	manager2 := GetManager()

	// They should be different instances (because we reset the singleton)
	assert.NotSame(t, manager1, manager2)
}
