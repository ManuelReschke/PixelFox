package repository

import (
	"context"
	"sort"
	"time"

	"github.com/ManuelReschke/PixelFox/internal/pkg/cache"
)

// queueRepository implements the QueueRepository interface
type queueRepository struct {
	// Note: This repository doesn't use GORM DB since it operates on Redis/Cache
}

// NewQueueRepository creates a new queue repository instance
func NewQueueRepository() QueueRepository {
	return &queueRepository{}
}

// GetAllKeys retrieves all keys from Redis cache
func (r *queueRepository) GetAllKeys() ([]string, error) {
	redisClient := cache.GetClient()
	ctx := context.Background()

	keys, err := redisClient.Keys(ctx, "*").Result()
	if err != nil {
		return nil, err
	}

	return keys, nil
}

// GetValue retrieves a value for a specific key from Redis
func (r *queueRepository) GetValue(key string) (string, error) {
	redisClient := cache.GetClient()
	ctx := context.Background()

	value, err := redisClient.Get(ctx, key).Result()
	if err != nil {
		return "", err
	}

	return value, nil
}

// GetTTL retrieves the time-to-live for a specific key
func (r *queueRepository) GetTTL(key string) (time.Duration, error) {
	redisClient := cache.GetClient()
	ctx := context.Background()

	ttl, err := redisClient.TTL(ctx, key).Result()
	if err != nil {
		return -1, err
	}

	return ttl, nil
}

// DeleteKey deletes a specific key from Redis
func (r *queueRepository) DeleteKey(key string) (int64, error) {
	redisClient := cache.GetClient()
	ctx := context.Background()

	result, err := redisClient.Del(ctx, key).Result()
	if err != nil {
		return 0, err
	}

	return result, nil
}

// GetListLength returns the length of a Redis list
func (r *queueRepository) GetListLength(key string) (int64, error) {
	redisClient := cache.GetClient()
	ctx := context.Background()

	length, err := redisClient.LLen(ctx, key).Result()
	if err != nil {
		return 0, err
	}

	return length, nil
}

// FindKeysByPatterns retrieves keys for the provided Redis match patterns using SCAN.
func (r *queueRepository) FindKeysByPatterns(patterns []string) ([]string, error) {
	redisClient := cache.GetClient()
	ctx := context.Background()

	uniqueKeys := make(map[string]struct{})

	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}

		var cursor uint64
		for {
			keys, nextCursor, err := redisClient.Scan(ctx, cursor, pattern, 500).Result()
			if err != nil {
				return nil, err
			}

			for _, key := range keys {
				uniqueKeys[key] = struct{}{}
			}

			cursor = nextCursor
			if cursor == 0 {
				break
			}
		}
	}

	keys := make([]string, 0, len(uniqueKeys))
	for key := range uniqueKeys {
		keys = append(keys, key)
	}

	sort.Strings(keys)
	return keys, nil
}

// DeleteKeys deletes keys in batches and returns the total number of deleted keys.
func (r *queueRepository) DeleteKeys(keys []string) (int64, error) {
	if len(keys) == 0 {
		return 0, nil
	}

	redisClient := cache.GetClient()
	ctx := context.Background()

	const batchSize = 500
	var totalDeleted int64

	for i := 0; i < len(keys); i += batchSize {
		end := i + batchSize
		if end > len(keys) {
			end = len(keys)
		}

		deleted, err := redisClient.Del(ctx, keys[i:end]...).Result()
		if err != nil {
			return totalDeleted, err
		}
		totalDeleted += deleted
	}

	return totalDeleted, nil
}
