//go:build integration
// +build integration

package jobqueue

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRedisQueue(t *testing.T) (*Queue, context.Context) {
	t.Helper()

	client := newIsolatedRedisClient(t, isolatedJobQueueTestRedisDB)
	queue := NewQueue(1)
	queue.client = client
	resetJobQueueRedisWithClient(t, client)
	t.Cleanup(func() {
		resetJobQueueRedisWithClient(t, client)
	})
	return queue, context.Background()
}

func TestQueue_EnqueueJob(t *testing.T) {
	queue, ctx := setupRedisQueue(t)

	payload := map[string]interface{}{
		"image_uuid": "integration-job",
		"kind":       "thumb",
	}
	job, err := queue.EnqueueJob(JobTypeImageProcessing, payload)
	require.NoError(t, err)
	require.NotNil(t, job)
	assert.NotEmpty(t, job.ID)
	assert.Equal(t, JobStatusPending, job.Status)
	assert.Equal(t, JobTypeImageProcessing, job.Type)

	queueSize, err := queue.GetQueueSize(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 1, queueSize)

	stats, err := queue.GetJobStats(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 1, stats[JobStatusPending])
}

func TestQueue_EnqueueJob_PipelineError(t *testing.T) {
	queue := NewQueue(1)
	queue.client = redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:0",
		DialTimeout:  100 * time.Millisecond,
		ReadTimeout:  100 * time.Millisecond,
		WriteTimeout: 100 * time.Millisecond,
		PoolTimeout:  100 * time.Millisecond,
	})
	t.Cleanup(func() { _ = queue.client.Close() })

	job, err := queue.EnqueueJob(JobTypeImageProcessing, map[string]interface{}{"k": "v"})
	require.Error(t, err)
	assert.Nil(t, job)
}

func TestQueue_GetJob(t *testing.T) {
	queue, ctx := setupRedisQueue(t)

	created, err := queue.EnqueueJob(JobTypeImageProcessing, map[string]interface{}{"image_uuid": "abc"})
	require.NoError(t, err)

	stored, err := queue.GetJob(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, stored.ID)
	assert.Equal(t, JobTypeImageProcessing, stored.Type)
	assert.Equal(t, JobStatusPending, stored.Status)
}

func TestQueue_GetJob_NotFound(t *testing.T) {
	queue, ctx := setupRedisQueue(t)

	_, err := queue.GetJob(ctx, "missing-job-id")
	require.Error(t, err)
	assert.ErrorIs(t, err, redis.Nil)
}

func TestQueue_GetJobStats(t *testing.T) {
	queue, ctx := setupRedisQueue(t)

	queue.updateJobStats(ctx, JobStatusPending, 2)
	queue.updateJobStats(ctx, JobStatusFailed, 1)

	stats, err := queue.GetJobStats(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 2, stats[JobStatusPending])
	assert.EqualValues(t, 1, stats[JobStatusFailed])
}

func TestQueue_GetQueueSize(t *testing.T) {
	queue, ctx := setupRedisQueue(t)

	_, err := queue.EnqueueJob(JobTypeImageProcessing, map[string]interface{}{"idx": "1"})
	require.NoError(t, err)
	_, err = queue.EnqueueJob(JobTypeImageProcessing, map[string]interface{}{"idx": "2"})
	require.NoError(t, err)

	size, err := queue.GetQueueSize(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 2, size)
}

func TestQueue_GetProcessingSize(t *testing.T) {
	queue, ctx := setupRedisQueue(t)

	_, err := queue.EnqueueJob(JobTypeImageProcessing, map[string]interface{}{"idx": "1"})
	require.NoError(t, err)
	_, err = queue.dequeueJob(ctx)
	require.NoError(t, err)

	size, err := queue.GetProcessingSize(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 1, size)
}

func TestQueue_StartStop(t *testing.T) {
	queue, _ := setupRedisQueue(t)

	assert.False(t, queue.running)
	queue.Start()
	assert.True(t, queue.running)
	queue.Stop()
	assert.False(t, queue.running)
}

func TestQueue_updateJob(t *testing.T) {
	queue, ctx := setupRedisQueue(t)

	job := &Job{
		ID:         "manual-job-id",
		Type:       JobTypeImageProcessing,
		Status:     JobStatusFailed,
		Payload:    map[string]interface{}{"key": "value"},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		RetryCount: 2,
		MaxRetries: 3,
	}
	queue.updateJob(ctx, job)

	stored, err := queue.GetJob(ctx, job.ID)
	require.NoError(t, err)
	assert.Equal(t, JobStatusFailed, stored.Status)
	assert.Equal(t, 2, stored.RetryCount)
}

func TestQueue_removeFromProcessing(t *testing.T) {
	queue, ctx := setupRedisQueue(t)

	created, err := queue.EnqueueJob(JobTypeImageProcessing, map[string]interface{}{"idx": "1"})
	require.NoError(t, err)
	_, err = queue.dequeueJob(ctx)
	require.NoError(t, err)

	queue.removeFromProcessing(ctx, created.ID)

	size, err := queue.GetProcessingSize(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 0, size)
}

func TestQueue_removeCompletedJob(t *testing.T) {
	queue, ctx := setupRedisQueue(t)

	created, err := queue.EnqueueJob(JobTypeImageProcessing, map[string]interface{}{"idx": "1"})
	require.NoError(t, err)

	queue.removeCompletedJob(ctx, created.ID)

	_, err = queue.GetJob(ctx, created.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, redis.Nil)
}

func TestQueue_updateJobStats(t *testing.T) {
	queue, ctx := setupRedisQueue(t)

	queue.updateJobStats(ctx, JobStatusCompleted, 3)
	queue.updateJobStats(ctx, JobStatusFailed, 2)

	stats, err := queue.GetJobStats(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 3, stats[JobStatusCompleted])
	assert.EqualValues(t, 2, stats[JobStatusFailed])
}

func TestQueue_requeueJob(t *testing.T) {
	queue, ctx := setupRedisQueue(t)

	created, err := queue.EnqueueJob(JobTypeImageProcessing, map[string]interface{}{"idx": "1"})
	require.NoError(t, err)

	job, err := queue.dequeueJob(ctx)
	require.NoError(t, err)
	require.NotNil(t, job)

	require.NoError(t, queue.requeueJob(ctx, job))

	queueSize, err := queue.GetQueueSize(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 1, queueSize)

	processingSize, err := queue.GetProcessingSize(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 0, processingSize)

	reloaded, err := queue.GetJob(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, JobStatusPending, reloaded.Status)
}

func TestManager_StartStop(t *testing.T) {
	client := newIsolatedRedisClient(t, isolatedJobQueueTestRedisDB)
	resetJobQueueRedisWithClient(t, client)
	t.Cleanup(func() {
		resetJobQueueRedisWithClient(t, client)
	})

	globalManager = nil
	managerOnce = sync.Once{}
	manager := GetManager()
	manager.queue.client = client

	assert.False(t, manager.IsRunning())
	manager.Start()
	assert.True(t, manager.IsRunning())
	manager.Stop()
	assert.False(t, manager.IsRunning())
}
