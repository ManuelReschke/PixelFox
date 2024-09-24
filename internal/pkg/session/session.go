package session

import (
	"time"

	"github.com/gofiber/fiber/v2/middleware/session"
)

var sessionStore *session.Store
var sessionKeyValue map[string]string

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
	sessionKeyValue[key] = value
}

func GetValueByKey(key string) string {
	return sessionKeyValue[key]
}
