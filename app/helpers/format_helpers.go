package helpers

import (
	"fmt"
	"github.com/a-h/templ"
	
)

// FormatCoordinate formatiert Koordinaten für die Anzeige
func FormatCoordinate(lat, lng float64) string {
	return fmt.Sprintf("%.6f, %.6f", lat, lng)
}

// FormatMapLink erstellt einen Google Maps Link für Koordinaten
func FormatMapLink(lat, lng float64) templ.SafeURL {
	return templ.SafeURL(fmt.Sprintf("https://www.google.com/maps?q=%f,%f", lat, lng))
}

// FormatFileSize formatiert eine Dateigröße in Bytes in ein lesbares Format
func FormatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// FormatTime formatiert einen Zeitstempel in ein lesbares Format
func FormatTime(t time.Time) string {
	return t.Format("02.01.2006 15:04")
}
