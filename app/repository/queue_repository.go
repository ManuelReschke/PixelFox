package repository

import (
	"context"
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
