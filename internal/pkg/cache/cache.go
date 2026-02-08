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
	password := env.GetEnv("CACHE_PASSWORD", "")

	candidateHosts := []string{host}
	if host == "cache" {
		candidateHosts = append(candidateHosts, "pxlfox-cache")
	}
	if host == "pxlfox-cache" {
		candidateHosts = append(candidateHosts, "cache")
	}
	candidateHosts = append(candidateHosts, "localhost", "127.0.0.1")

	seen := make(map[string]struct{})
	var lastErr error
	for _, candidateHost := range candidateHosts {
		if candidateHost == "" {
			continue
		}
		if _, ok := seen[candidateHost]; ok {
			continue
		}
		seen[candidateHost] = struct{}{}

		candidate := redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%s", candidateHost, port),
			Password: password,
			DB:       0, // use default DB
		})

		pong, err := candidate.Ping(ctx).Result()
		if err == nil {
			client = candidate
			log.Printf("Successfully connected to Dragonfly cache (%s:%s): %s", candidateHost, port, pong)
			return
		}

		lastErr = err
		_ = candidate.Close()
	}

	// Keep a client instance for retry attempts later, even if initial connect failed.
	client = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Password: password,
		DB:       0, // use default DB
	})
	log.Printf("Warning: Could not connect to Dragonfly cache: %v", lastErr)
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
