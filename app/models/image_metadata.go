package models

import (
	"time"
)

// ImageMetadata enthält die extrahierten Metadaten aus einem Bild
type ImageMetadata struct {
	CameraModel  string
	TakenAt      *time.Time
	Latitude     *float64
	Longitude    *float64
	ExposureTime string
	Aperture     string
	ISO          *int
	FocalLength  string
	RawMetadata  JSON
}
