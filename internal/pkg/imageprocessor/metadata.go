package imageprocessor

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2/log"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/mknote"

	"github.com/ManuelReschke/PixelFox/app/models"
)

func init() {
	// Register Nikon and Canon maker notes
	exif.RegisterParsers(mknote.All...)
}

// ExtractMetadata extracts EXIF metadata from an image file
func ExtractMetadata(image *models.Image, filePath string) error {
	// Open the image file
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("error opening image file: %w", err)
	}
	defer f.Close()

	// Try to decode EXIF data
	x, err := exif.Decode(f)
	if err != nil {
		// Some images don't have EXIF data, this is not a critical error
		log.Info(fmt.Sprintf("No EXIF data found for image %s: %v", image.UUID, err))
		return nil
	}

	// Extract all metadata into a map for JSON storage
	allMetadata := make(map[string]interface{})

	// Manually walk through common EXIF tags to avoid type issues
	for _, tag := range []exif.FieldName{
		exif.Model, exif.Make, exif.Software, exif.Artist,
		exif.Copyright, exif.ExposureTime, exif.FNumber, exif.ISOSpeedRatings,
		exif.FocalLength, exif.ExposureProgram, exif.MeteringMode,
		exif.Flash, exif.FocalLengthIn35mmFilm, exif.WhiteBalance,
		exif.SceneCaptureType, exif.GPSLatitude, exif.GPSLongitude,
		exif.GPSAltitude, exif.DateTime, exif.DateTimeOriginal,
		exif.DateTimeDigitized, exif.SubjectArea, exif.ExposureMode,
	} {
		if tagVal, err := x.Get(tag); err == nil {
			raw := tagVal.String()
			clean := strings.Trim(raw, `"`)
			allMetadata[string(tag)] = clean
		}
	}

	// Extract specific metadata fields

	// 1. Camera Model (strip quotes)
	if m, err := x.Get(exif.Model); err == nil {
		s := strings.Trim(m.String(), `"`)
		trimmed := strings.TrimSpace(s)
		image.CameraModel = &trimmed
	}

	// 2. Date and Time
	if dt, err := x.DateTime(); err == nil {
		image.TakenAt = &dt
	}

	// 3. GPS Coordinates
	if lat, long, err := x.LatLong(); err == nil {
		image.Latitude = &lat
		image.Longitude = &long
	}

	// 4. Exposure Time
	if expTag, err := x.Get(exif.ExposureTime); err == nil {
		raw := expTag.String()
		trimmed := strings.Trim(raw, `"`)
		image.ExposureTime = &trimmed
	}

	// 5. Aperture (F-Number)
	if fTag, err := x.Get(exif.FNumber); err == nil {
		// F-number is typically stored as a rational
		floatVal, err := fTag.Float(0)
		if err == nil {
			apertureStr := fmt.Sprintf("f/%.1f", floatVal)
			image.Aperture = &apertureStr
		} else {
			trimmed := strings.Trim(fTag.String(), `"`)
			image.Aperture = &trimmed
		}
	}

	// 6. ISO
	if isoTag, err := x.Get(exif.ISOSpeedRatings); err == nil {
		isoVal, err := isoTag.Int(0)
		if err == nil {
			iso := int(isoVal)
			image.ISO = &iso
		}
	}

	// 7. Focal Length
	if flTag, err := x.Get(exif.FocalLength); err == nil {
		floatVal, err := flTag.Float(0)
		if err == nil {
			focalStr := fmt.Sprintf("%.1fmm", floatVal)
			image.FocalLength = &focalStr
		} else {
			trimmed := strings.Trim(flTag.String(), `"`)
			image.FocalLength = &trimmed
		}
	}

	metadataJSON, err := json.Marshal(allMetadata)
	if err != nil {
		log.Error(fmt.Sprintf("Error marshaling metadata to JSON: %v", err))
		// Continue even if JSON marshaling fails
	} else {
		meta := models.JSON(metadataJSON)
		image.Metadata = &meta
	}

	return nil
}
