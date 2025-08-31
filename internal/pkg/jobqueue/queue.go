package jobqueue

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2/log"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/ManuelReschke/PixelFox/internal/pkg/cache"
)

const (
	// Redis key prefixes
	JobKeyPrefix     = "job:"
	JobQueueKey      = "job_queue"
	JobProcessingKey = "job_processing"
	JobStatsKey      = "job_stats"

	// Job settings
	DefaultMaxRetries = 3
	JobTTL            = 24 * time.Hour // Jobs expire after 24 hours
)

// Queue manages background jobs using Redis
type Queue struct {
	client     *redis.Client
	workers    int
	workerPool chan struct{}
	stopCh     chan struct{}
	wg         sync.WaitGroup
	mu         sync.Mutex
	running    bool
}

// NewQueue creates a new job queue
func NewQueue(workers int) *Queue {
	if workers <= 0 {
		workers = 3 // Default number of workers
	}

	return &Queue{
		client:     cache.GetClient(),
		workers:    workers,
		workerPool: make(chan struct{}, workers),
		stopCh:     make(chan struct{}),
	}
}

// Start starts the job queue workers
func (q *Queue) Start() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.running {
		return
	}

	q.running = true
	log.Infof("[JobQueue] Starting %d workers", q.workers)

	// Initialize worker pool
	for i := 0; i < q.workers; i++ {
		q.workerPool <- struct{}{}
	}

	// Start workers
	for i := 0; i < q.workers; i++ {
		q.wg.Add(1)
		go q.worker(i)
	}

	// Start stuck-processing sweeper (recovers jobs stuck in processing due to crashes)
	q.wg.Add(1)
	go q.stuckSweeper(10*time.Minute, 1*time.Minute)
}

// Stop stops the job queue workers
func (q *Queue) Stop() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if !q.running {
		return
	}

	log.Info("[JobQueue] Stopping workers...")
	close(q.stopCh)
	q.running = false
	q.wg.Wait()
	log.Info("[JobQueue] All workers stopped")
}

// stuckSweeper periodically scans the processing list and requeues jobs stuck for longer than maxAge
func (q *Queue) stuckSweeper(maxAge time.Duration, interval time.Duration) {
	defer q.wg.Done()
	log.Infof("[JobQueue] Stuck sweeper running (maxAge=%s, interval=%s)", maxAge, interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	ctx := context.Background()
	for {
		select {
		case <-q.stopCh:
			log.Info("[JobQueue] Stuck sweeper stopping")
			return
		case <-ticker.C:
			ids, err := q.client.LRange(ctx, JobProcessingKey, 0, -1).Result()
			if err != nil {
				log.Errorf("[JobQueue] Sweeper LRange error: %v", err)
				continue
			}
			now := time.Now()
			for _, id := range ids {
				jobKey := JobKeyPrefix + id
				data, err := q.client.Get(ctx, jobKey).Result()
				if err != nil {
					// Job data missing; remove from processing list
					if err != redis.Nil {
						log.Errorf("[JobQueue] Sweeper Get error for %s: %v", id, err)
					}
					_ = q.client.LRem(ctx, JobProcessingKey, 1, id).Err()
					continue
				}
				var job Job
				if uerr := json.Unmarshal([]byte(data), &job); uerr != nil {
					log.Errorf("[JobQueue] Sweeper unmarshal error for %s: %v", id, uerr)
					_ = q.client.LRem(ctx, JobProcessingKey, 1, id).Err()
					continue
				}
				if job.Status != JobStatusProcessing {
					// Clean up stray entry
					_ = q.client.LRem(ctx, JobProcessingKey, 1, id).Err()
					continue
				}
				// Determine when processing started
				started := job.ProcessedAt
				if started == nil || started.IsZero() {
					// Fallback to UpdatedAt/CreatedAt
					tmp := job.UpdatedAt
					if tmp.IsZero() {
						tmp = job.CreatedAt
					}
					started = &tmp
				}
				if now.Sub(*started) > maxAge {
					log.Warnf("[JobQueue] Recovering stuck job %s (type=%s), age=%s", job.ID, job.Type, now.Sub(*started))
					job.Status = JobStatusPending
					job.ErrorMsg = "recovered by sweeper"
					job.UpdatedAt = now
					q.updateJob(ctx, &job)
					// Move from processing back to pending
					_ = q.client.LRem(ctx, JobProcessingKey, 1, id).Err()
					_ = q.client.RPush(ctx, JobQueueKey, id).Err()
				}
			}
		}
	}
}

// worker processes jobs from the queue
func (q *Queue) worker(id int) {
	defer q.wg.Done()
	log.Infof("[JobQueue] Worker %d started", id)

	ctx := context.Background()

	for {
		select {
		case <-q.stopCh:
			log.Infof("[JobQueue] Worker %d stopping", id)
			return
		default:
			// Acquire worker slot
			<-q.workerPool

			// Try to get a job from the queue
			job, err := q.dequeueJob(ctx)
			if err != nil {
				if err != redis.Nil {
					log.Errorf("[JobQueue] Worker %d: Error dequeuing job: %v", id, err)
				}
				// Release worker slot and wait before retry
				q.workerPool <- struct{}{}
				time.Sleep(time.Second)
				continue
			}

			if job != nil {
				log.Infof("[JobQueue] Worker %d processing job %s (Type: %s)", id, job.ID, job.Type)
				q.processJob(ctx, job)
			}

			// Release worker slot
			q.workerPool <- struct{}{}
		}
	}
}

// EnqueueJob adds a new job to the queue
func (q *Queue) EnqueueJob(jobType JobType, payload map[string]interface{}) (*Job, error) {
	ctx := context.Background()

	job := &Job{
		ID:         uuid.New().String(),
		Type:       jobType,
		Status:     JobStatusPending,
		Payload:    payload,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		RetryCount: 0,
		MaxRetries: DefaultMaxRetries,
	}

	// Store job data
	jobData, err := json.Marshal(job)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal job: %w", err)
	}

	jobKey := JobKeyPrefix + job.ID

	// Use a pipeline for atomic operations
	pipe := q.client.Pipeline()
	pipe.Set(ctx, jobKey, jobData, JobTTL)
	pipe.LPush(ctx, JobQueueKey, job.ID)
	pipe.HIncrBy(ctx, JobStatsKey, string(JobStatusPending), 1)

	if _, err := pipe.Exec(ctx); err != nil {
		return nil, fmt.Errorf("failed to enqueue job: %w", err)
	}

	log.Infof("[JobQueue] Enqueued job %s (Type: %s)", job.ID, job.Type)
	return job, nil
}

// dequeueJob gets the next job from the queue
func (q *Queue) dequeueJob(ctx context.Context) (*Job, error) {
	// Move job from pending queue to processing queue atomically
	result, err := q.client.BRPopLPush(ctx, JobQueueKey, JobProcessingKey, time.Second).Result()
	if err != nil {
		return nil, err
	}

	jobID := result
	jobKey := JobKeyPrefix + jobID

	// Get job data
	jobData, err := q.client.Get(ctx, jobKey).Result()
	if err != nil {
		// Job data not found, remove from processing queue
		q.client.LRem(ctx, JobProcessingKey, 1, jobID)
		return nil, fmt.Errorf("job data not found for ID %s", jobID)
	}

	var job Job
	if err := json.Unmarshal([]byte(jobData), &job); err != nil {
		// Invalid job data, remove from processing queue
		q.client.LRem(ctx, JobProcessingKey, 1, jobID)
		return nil, fmt.Errorf("failed to unmarshal job %s: %w", jobID, err)
	}

	return &job, nil
}

// processJob processes a single job
func (q *Queue) processJob(ctx context.Context, job *Job) {
	job.MarkAsProcessing()
	q.updateJob(ctx, job)

	var err error
	switch job.Type {
	case JobTypeImageProcessing:
		err = q.processImageProcessingJob(ctx, job)
	case JobTypeS3Backup:
		err = q.processS3BackupJob(ctx, job)
	case JobTypeS3Delete:
		err = q.processS3DeleteJob(ctx, job)
	case JobTypePoolMoveEnqueue:
		err = q.processPoolMoveEnqueueJob(job)
	case JobTypeMoveImage:
		err = q.processMoveImageJob(job)
	default:
		err = fmt.Errorf("unknown job type: %s", job.Type)
	}

	if err != nil {
		if err == ErrRequeue {
			// Already requeued for node routing; do not mark failed or completed
			return
		}
		log.Errorf("[JobQueue] Job %s failed: %v", job.ID, err)
		job.MarkAsFailed(err.Error())

		// Check if job can be retried
		if job.IsRetryable() {
			log.Infof("[JobQueue] Retrying job %s (Attempt %d/%d)", job.ID, job.RetryCount, job.MaxRetries)
			job.MarkAsRetrying()
			q.updateJob(ctx, job)

			// Re-enqueue for retry after a delay
			time.AfterFunc(time.Minute*time.Duration(job.RetryCount), func() {
				q.client.LPush(ctx, JobQueueKey, job.ID)
			})
		} else {
			log.Errorf("[JobQueue] Job %s permanently failed after %d retries", job.ID, job.RetryCount)
			q.updateJobStats(ctx, JobStatusFailed, 1)
		}
	} else {
		log.Infof("[JobQueue] Job %s completed successfully", job.ID)
		job.MarkAsCompleted()
		q.updateJobStats(ctx, JobStatusCompleted, 1)
		// Remove completed job from Redis entirely
		q.removeCompletedJob(ctx, job.ID)
	}

	if job.Status != JobStatusCompleted {
		q.updateJob(ctx, job)
	}
	q.removeFromProcessing(ctx, job.ID)
}

// updateJob updates job data in Redis
func (q *Queue) updateJob(ctx context.Context, job *Job) {
	jobData, err := json.Marshal(job)
	if err != nil {
		log.Errorf("[JobQueue] Failed to marshal job %s: %v", job.ID, err)
		return
	}

	jobKey := JobKeyPrefix + job.ID
	if err := q.client.Set(ctx, jobKey, jobData, JobTTL).Err(); err != nil {
		log.Errorf("[JobQueue] Failed to update job %s: %v", job.ID, err)
	}
}

// requeueJob moves a job back to the pending queue and resets its status
func (q *Queue) requeueJob(ctx context.Context, job *Job) error {
	job.Status = JobStatusPending
	job.UpdatedAt = time.Now()
	q.updateJob(ctx, job)
	// Remove from processing list and push to the end of the queue
	if err := q.client.LRem(ctx, JobProcessingKey, 1, job.ID).Err(); err != nil {
		log.Errorf("[JobQueue] Failed to remove job %s from processing: %v", job.ID, err)
	}
	if err := q.client.RPush(ctx, JobQueueKey, job.ID).Err(); err != nil {
		log.Errorf("[JobQueue] Failed to requeue job %s: %v", job.ID, err)
		return err
	}
	return nil
}

// removeFromProcessing removes a job from the processing queue
func (q *Queue) removeFromProcessing(ctx context.Context, jobID string) {
	if err := q.client.LRem(ctx, JobProcessingKey, 1, jobID).Err(); err != nil {
		log.Errorf("[JobQueue] Failed to remove job %s from processing queue: %v", jobID, err)
	}
}

// removeCompletedJob completely removes a completed job from Redis
func (q *Queue) removeCompletedJob(ctx context.Context, jobID string) {
	jobKey := JobKeyPrefix + jobID
	if err := q.client.Del(ctx, jobKey).Err(); err != nil {
		log.Errorf("[JobQueue] Failed to remove completed job %s from Redis: %v", jobID, err)
	} else {
		log.Debugf("[JobQueue] Successfully removed completed job %s from Redis", jobID)
	}
}

// updateJobStats updates job statistics
func (q *Queue) updateJobStats(ctx context.Context, status JobStatus, delta int64) {
	if err := q.client.HIncrBy(ctx, JobStatsKey, string(status), delta).Err(); err != nil {
		log.Errorf("[JobQueue] Failed to update job stats: %v", err)
	}
}

// GetJob retrieves a job by ID
func (q *Queue) GetJob(ctx context.Context, jobID string) (*Job, error) {
	jobKey := JobKeyPrefix + jobID
	jobData, err := q.client.Get(ctx, jobKey).Result()
	if err != nil {
		return nil, err
	}

	var job Job
	if err := json.Unmarshal([]byte(jobData), &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}

	return &job, nil
}

// GetJobStats returns statistics about job statuses
func (q *Queue) GetJobStats(ctx context.Context) (map[JobStatus]int64, error) {
	stats, err := q.client.HGetAll(ctx, JobStatsKey).Result()
	if err != nil {
		return nil, err
	}

	result := make(map[JobStatus]int64)
	for status, count := range stats {
		if countInt, err := json.Number(count).Int64(); err == nil {
			result[JobStatus(status)] = countInt
		}
	}

	return result, nil
}

// GetQueueSize returns the number of pending jobs
func (q *Queue) GetQueueSize(ctx context.Context) (int64, error) {
	return q.client.LLen(ctx, JobQueueKey).Result()
}

// GetProcessingSize returns the number of jobs being processed
func (q *Queue) GetProcessingSize(ctx context.Context) (int64, error) {
	return q.client.LLen(ctx, JobProcessingKey).Result()
}
