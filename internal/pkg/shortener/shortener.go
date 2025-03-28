package shortener

import (
	"strings"
)

// Alphabet für die Umwandlung (62 Zeichen: 0-9, a-z, A-Z)
const alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

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
