package jobqueue

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ManuelReschke/PixelFox/internal/pkg/cache"
	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
	"github.com/redis/go-redis/v9"
)

const isolatedJobQueueTestRedisDB = 14

func resolveTestRedis(t *testing.T) (string, string, string) {
	t.Helper()

	hosts := []string{
		env.GetEnv("CACHE_HOST", ""),
		"cache",
		"pxlfox-cache",
		"localhost",
		"127.0.0.1",
	}
	ports := []string{
		env.GetEnv("CACHE_PORT", "6379"),
		"6379",
	}
	passwords := []string{
		env.GetEnv("CACHE_PASSWORD", ""),
		"pixelfox",
		"",
	}

	seenHost := make(map[string]struct{})
	seenPort := make(map[string]struct{})
	seenPassword := make(map[string]struct{})
	uniqueHosts := make([]string, 0, len(hosts))
	uniquePorts := make([]string, 0, len(ports))
	uniquePasswords := make([]string, 0, len(passwords))

	for _, host := range hosts {
		if host == "" {
			continue
		}
		if _, ok := seenHost[host]; ok {
			continue
		}
		seenHost[host] = struct{}{}
		uniqueHosts = append(uniqueHosts, host)
	}
	for _, port := range ports {
		if port == "" {
			continue
		}
		if _, ok := seenPort[port]; ok {
			continue
		}
		seenPort[port] = struct{}{}
		uniquePorts = append(uniquePorts, port)
	}
	for _, password := range passwords {
		if _, ok := seenPassword[password]; ok {
			continue
		}
		seenPassword[password] = struct{}{}
		uniquePasswords = append(uniquePasswords, password)
	}

	var lastErr error
	for _, host := range uniqueHosts {
		for _, port := range uniquePorts {
			for _, password := range uniquePasswords {
				client := redis.NewClient(&redis.Options{
					Addr:     fmt.Sprintf("%s:%s", host, port),
					Password: password,
					DB:       0,
				})

				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				_, err := client.Ping(ctx).Result()
				cancel()
				_ = client.Close()
				if err == nil {
					return host, port, password
				}
				lastErr = err
			}
		}
	}

	t.Skipf("Skipping Redis-dependent test: no reachable Redis endpoint (%v)", lastErr)
	return "", "", ""
}

func configureTestCache(host, port, password string) {
	if env.Env == nil {
		env.Env = map[string]string{}
	}

	env.Env["CACHE_HOST"] = host
	env.Env["CACHE_PORT"] = port
	env.Env["CACHE_PASSWORD"] = password

	_ = os.Setenv("CACHE_HOST", host)
	_ = os.Setenv("CACHE_PORT", port)
	_ = os.Setenv("CACHE_PASSWORD", password)

	cache.SetupCache()
}

func resetJobQueueRedis(t *testing.T) {
	t.Helper()

	resetJobQueueRedisWithClient(t, cache.GetClient())
}

func resetJobQueueRedisWithClient(t *testing.T, client *redis.Client) {
	t.Helper()

	ctx := context.Background()

	keys := []string{
		JobQueueKey,
		JobProcessingKey,
		JobStatsKey,
	}

	iter := client.Scan(ctx, 0, JobKeyPrefix+"*", 0).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		t.Fatalf("failed to scan redis keys: %v", err)
	}

	if err := client.Del(ctx, keys...).Err(); err != nil {
		t.Fatalf("failed to cleanup redis keys: %v", err)
	}
}

func newIsolatedRedisClient(t *testing.T, db int) *redis.Client {
	t.Helper()

	host, port, password := resolveTestRedis(t)
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Password: password,
		DB:       db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	_, err := client.Ping(ctx).Result()
	cancel()
	if err != nil {
		_ = client.Close()
		t.Skipf("Skipping Redis-dependent test: isolated DB ping failed (%v)", err)
	}

	if err := client.FlushDB(context.Background()).Err(); err != nil {
		_ = client.Close()
		t.Fatalf("failed to flush isolated redis db %d: %v", db, err)
	}

	t.Cleanup(func() {
		_ = client.FlushDB(context.Background()).Err()
		_ = client.Close()
	})

	return client
}
