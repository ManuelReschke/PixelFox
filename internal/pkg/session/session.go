package session

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/gofiber/storage/redis"

	"github.com/ManuelReschke/PixelFox/internal/pkg/cache"
	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
)

var sessionStore *session.Store

func NewSessionStore() *session.Store {
	// Get Redis client configuration from existing cache setup
	cacheClient := cache.GetClient()
	host := "localhost"
	port := 6379
	password := env.GetEnv("CACHE_PASSWORD", "")
	if cacheClient != nil {
		addr := cacheClient.Options().Addr
		if h, p, err := net.SplitHostPort(addr); err == nil {
			host = h
			if v, err := strconv.Atoi(p); err == nil {
				port = v
			}
		}
		// Prefer password from the underlying client if present
		if p := cacheClient.Options().Password; p != "" {
			password = p
		}
	}

	// Create Redis storage for sessions using database 1 (cache uses DB 0)
	storage := redis.New(redis.Config{
		Host:     host,
		Port:     port,
		Password: password,
		Database: 1, // Separate database for sessions
		Reset:    false,
	})

	sessionStore = session.New(session.Config{
		Storage:        storage,
		CookieHTTPOnly: true,
		// CookieSecure:   true, // Enable in production with HTTPS
		Expiration: time.Hour * 1,
		KeyLookup:  "cookie:session_id",
	})

	return sessionStore
}

func GetSessionStore() *session.Store {
	return sessionStore
}

// SetSessionValue stores a key-value pair in the user's individual session
func SetSessionValue(c *fiber.Ctx, key string, value string) error {
	if sessionStore == nil {
		return fmt.Errorf("session store not initialized")
	}

	sess, err := sessionStore.Get(c)
	if err != nil {
		return fmt.Errorf("failed to get session: %v", err)
	}

	sess.Set(key, value)
	return sess.Save()
}

// GetSessionValue retrieves a value by key from the user's individual session
func GetSessionValue(c *fiber.Ctx, key string) string {
	if sessionStore == nil {
		return ""
	}

	sess, err := sessionStore.Get(c)
	if err != nil {
		return ""
	}

	value := sess.Get(key)
	if value == nil {
		return ""
	}

	if strValue, ok := value.(string); ok {
		return strValue
	}

	return ""
}

// Legacy functions for backward compatibility - DEPRECATED
// These should not be used in new code as they are not multi-user safe
var globalKeyValueStore map[string]string

func initGlobalStore() {
	if globalKeyValueStore == nil {
		globalKeyValueStore = make(map[string]string)
	}
}

// DEPRECATED: Use SetKeyValue(c, key, value) instead
func SetKeyValue(key string, value string) {
	initGlobalStore()
	globalKeyValueStore[key] = value
}

// DEPRECATED: Use GetValueByKey(c, key) instead
func GetValueByKey(key string) string {
	initGlobalStore()
	if value, exists := globalKeyValueStore[key]; exists {
		return value
	}
	return ""
}
