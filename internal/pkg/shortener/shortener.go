package shortener

import (
	"crypto/rand"
	"fmt"
	"strings"
)

// Alphabet für die Umwandlung (62 Zeichen: 0-9, a-z, A-Z)
const alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// GenerateSecureSlug creates a cryptographically secure random Base62 slug.
func GenerateSecureSlug(length int) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("invalid slug length: %d", length)
	}

	// Rejection sampling to avoid modulo bias.
	// 248 is the largest multiple of 62 below 256.
	const maxRandomByte = 248

	slug := make([]byte, length)
	buf := make([]byte, length*2)
	written := 0

	for written < length {
		if _, err := rand.Read(buf); err != nil {
			return "", fmt.Errorf("failed to read secure random bytes: %w", err)
		}

		for _, b := range buf {
			if b >= maxRandomByte {
				continue
			}
			slug[written] = alphabet[int(b)%len(alphabet)]
			written++
			if written == length {
				break
			}
		}
	}

	return string(slug), nil
}

// EncodeID wandelt eine numerische ID in einen kurzen alphanumerischen String um
// Ähnlich wie bei URL-Shortenern wird jede Zahl in eine Basis-62 Darstellung umgewandelt
func EncodeID(id uint) string {
	if id == 0 {
		return string(alphabet[0])
	}

	base := len(alphabet)
	encoded := strings.Builder{}

	for id > 0 {
		remained := id % uint(base)
		encoded.WriteByte(alphabet[remained])
		id = id / uint(base)
	}

	// Umkehren des Strings, da wir von rechts nach links gearbeitet haben
	reversed := make([]byte, encoded.Len())
	str := encoded.String()
	for i := 0; i < encoded.Len(); i++ {
		reversed[encoded.Len()-1-i] = str[i]
	}

	return string(reversed)
}

// DecodeID wandelt einen alphanumerischen String zurück in eine ID
func DecodeID(encoded string) uint {
	base := len(alphabet)
	var id uint = 0

	for i := 0; i < len(encoded); i++ {
		char := encoded[i]
		value := strings.IndexByte(alphabet, char)
		if value == -1 {
			// Ungültiges Zeichen, ignorieren
			continue
		}
		id = id*uint(base) + uint(value)
	}

	return id
}
