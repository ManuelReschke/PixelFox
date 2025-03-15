package cache

import (
	"context"
	"fmt"
	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
	"github.com/redis/go-redis/v9"
	"log"
	"time"
)

var (
	client *redis.Client
	ctx    = context.Background()
)

// SetupCache initializes the connection to the Dragonfly cache server
func SetupCache() {
	host := env.GetEnv("CACHE_HOST", "localhost")
	port := env.GetEnv("CACHE_PORT", "6379")

	client = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	// Test the connection
	pong, err := client.Ping(ctx).Result()
	if err != nil {
		log.Printf("Warning: Could not connect to Dragonfly cache: %v", err)
	} else {
		log.Printf("Successfully connected to Dragonfly cache: %s", pong)
	}
}

// GetClient returns the Redis client instance
func GetClient() *redis.Client {
	if client == nil {
		SetupCache()
	}
	return client
}

// Set stores a value in the cache with the given key and expiration time
func Set(key string, value interface{}, expiration time.Duration) error {
	return GetClient().Set(ctx, key, value, expiration).Err()
}

// Get retrieves a value from the cache by key
func Get(key string) (string, error) {
	return GetClient().Get(ctx, key).Result()
}

// GetInt retrieves an integer value from the cache by key
func GetInt(key string) (int, error) {
	val, err := GetClient().Get(ctx, key).Int()
	if err != nil {
		return 0, err
	}
	return val, nil
}

// Delete removes a value from the cache by key
func Delete(key string) error {
	return GetClient().Del(ctx, key).Err()
}
