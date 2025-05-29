package utils

import (
	"crypto/md5"
	"fmt"
	"strings"
)

// GetGravatarURL generates a Gravatar URL for the given email address
// Default size is 200px if not specified
func GetGravatarURL(email string, size int) string {
	if size <= 0 {
		size = 200
	}

	// Trim leading and trailing whitespace
	email = strings.TrimSpace(email)
	// Force lowercase
	email = strings.ToLower(email)

	// Create MD5 hash
	hash := md5.Sum([]byte(email))
	hashString := fmt.Sprintf("%x", hash)

	// Return Gravatar URL
	return fmt.Sprintf("https://www.gravatar.com/avatar/%s?s=%d&d=mp", hashString, size)
}
