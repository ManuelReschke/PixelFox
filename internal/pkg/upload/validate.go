package upload

import (
	"errors"
	"net/http"
	"path/filepath"
	"strings"
)

var allowedExt = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".webp": true,
	".avif": true,
	".bmp":  true,
	// Note: SVG is intentionally excluded due to XSS risk without sanitization
}

var allowedMime = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"image/webp": true,
	"image/avif": true,
	"image/bmp":  true,
	// "image/svg+xml": false // require sanitizer if enabled in future
}

// ValidateImageBySniff checks the provided filename (extension) and the first bytes (head)
// against a whitelist of image types. Returns detected mime or an error.
func ValidateImageBySniff(filename string, head []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	if !allowedExt[ext] {
		return "", errors.New("Nur folgende Bildformate werden unterstützt: JPG, JPEG, PNG, GIF, WEBP, AVIF, BMP")
	}

	detected := http.DetectContentType(head)

	// Block obvious scriptable types regardless of extension
	if strings.HasPrefix(detected, "text/html") || strings.HasPrefix(detected, "application/xhtml") {
		return "", errors.New("Ungültiger Dateityp: HTML‑Inhalte sind nicht erlaubt")
	}
	if strings.HasPrefix(detected, "text/xml") || strings.HasPrefix(detected, "application/xml") || detected == "image/svg+xml" {
		// Block SVG/XML until sanitizer is available
		return "", errors.New("SVG/XML werden aus Sicherheitsgründen nicht unterstützt")
	}

	// Some formats (e.g., AVIF) may return octet-stream depending on Go version; allow by extension
	if detected == "application/octet-stream" && allowedExt[ext] {
		return detected, nil
	}

	if allowedMime[detected] {
		return detected, nil
	}

	// Fallback: for jpegs sometimes detected is "application/octet-stream"; already handled
	return "", errors.New("Der Dateityp wird nicht unterstützt")
}
