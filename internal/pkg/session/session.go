package session

import (
	"sync"
	"time"

	"github.com/gofiber/fiber/v2/middleware/session"
)

var sessionStore *session.Store
var sessionKeyValue map[string]string
var sessionMutex sync.RWMutex // Mutex für Thread-Safety

func NewSessionStore() *session.Store {
	sessionStore = session.New(session.Config{
		CookieHTTPOnly: true,
		// CookieSecure:   true,
		Expiration: time.Hour * 1,
	})

	sessionKeyValue = make(map[string]string)

	return sessionStore
}

func GetSessionStore() *session.Store {
	return sessionStore
}

func SetKeyValue(key string, value string) {
	sessionMutex.Lock()         // Lock vor dem Schreiben
	defer sessionMutex.Unlock() // Unlock nach dem Schreiben
	sessionKeyValue[key] = value
}

func GetValueByKey(key string) string {
	sessionMutex.RLock()         // Read-Lock vor dem Lesen
	defer sessionMutex.RUnlock() // Unlock nach dem Lesen
	return sessionKeyValue[key]
}
