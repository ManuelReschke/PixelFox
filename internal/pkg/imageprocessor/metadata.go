package imageprocessor

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/disintegration/imaging"
	"github.com/gofiber/fiber/v2/log"
	"github.com/rwcarlsen/goexif/exif"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
)

// ExtractImageMetadata extracts metadata from a file and returns a metadata struct, width, and height
func ExtractImageMetadata(file *os.File) (*models.ImageMetadata, int, int, error) {
	// Reset the file pointer to the beginning
	_, err := file.Seek(0, 0)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("error seeking file: %w", err)
	}

	// Open the image with imaging for dimension extraction
	img, err := imaging.Decode(file)
	if err != nil {
		log.Error(fmt.Sprintf("Error decoding image to get dimensions: %v", err))
		return nil, 0, 0, fmt.Errorf("error decoding image: %w", err)
	}

	// Extract dimensions
	width := img.Bounds().Dx()
	height := img.Bounds().Dy()

	// Reset the file pointer for EXIF extraction
	_, err = file.Seek(0, 0)
	if err != nil {
		return nil, width, height, fmt.Errorf("error seeking file for EXIF: %w", err)
	}

	// Create metadata struct
	metadata := &models.ImageMetadata{}

	// Try to extract EXIF data
	x, err := exif.Decode(file)
	if err != nil {
		// If no EXIF data is found, just return with dimensions
		log.Warn(fmt.Sprintf("No EXIF data found: %v", err))
		return metadata, width, height, nil
	}

	// Collect all metadata in a map
	allMetadata := make(map[string]interface{})

	// Extrahiere wichtige EXIF-Tags direkt
	for _, name := range []exif.FieldName{
		exif.Model,
		exif.DateTime,
		exif.DateTimeOriginal,
		exif.ExposureTime,
		exif.FNumber,
		exif.ISOSpeedRatings,
		exif.FocalLength,
		exif.GPSLatitude,
		exif.GPSLongitude,
	} {
		tag, err := x.Get(name)
		if err == nil {
			val, err := tag.StringVal()
			if err == nil {
				allMetadata[string(name)] = val
			}
		}
	}

	// 1. Camera Model
	metadata.CameraModel = getExifTagValue(x, exif.Model)

	// 2. Date Taken
	dateTimeTag, err := x.DateTime()
	if err == nil {
		metadata.TakenAt = &dateTimeTag
	}

	// 3. GPS Coordinates
	lat, long, err := x.LatLong()
	if err == nil {
		metadata.Latitude = &lat
		metadata.Longitude = &long
	}

	// 4. Exposure Time
	expVal, err := getExifTagFloat(x, exif.ExposureTime)
	if err == nil {
		metadata.ExposureTime = fmt.Sprintf("%.1f", expVal)
	} else {
		metadata.ExposureTime = getExifTagValue(x, exif.ExposureTime)
	}

	// 5. Aperture
	apertureVal, err := getExifTagFloat(x, exif.FNumber)
	if err == nil {
		metadata.Aperture = fmt.Sprintf("f/%.1f", apertureVal)
	} else {
		metadata.Aperture = getExifTagValue(x, exif.FNumber)
	}

	// 6. ISO
	isoVal, err := getExifTagFloat(x, exif.ISOSpeedRatings)
	if err == nil {
		iso := int(isoVal)
		metadata.ISO = &iso
	} else {
		metadata.ISO = nil
	}

	// 7. Focal Length
	flVal, err := getExifTagFloat(x, exif.FocalLength)
	if err == nil {
		metadata.FocalLength = fmt.Sprintf("%.1fmm", flVal)
	} else {
		metadata.FocalLength = getExifTagValue(x, exif.FocalLength)
	}

	// Store all metadata as JSON
	metadataJSON, err := json.Marshal(allMetadata)
	if err != nil {
		log.Error(fmt.Sprintf("Error marshaling metadata to JSON: %v", err))
	} else {
		metadata.RawMetadata = models.JSON(metadataJSON)
	}

	return metadata, width, height, nil
}

// getExifTagValue holt den Wert eines EXIF-Tags als String
func getExifTagValue(x *exif.Exif, tagName exif.FieldName) string {
	tag, err := x.Get(tagName)
	if err != nil {
		return ""
	}
	val, err := tag.StringVal()
	if err != nil {
		log.Warn(fmt.Sprintf("Error getting string value for tag %s: %v", tagName, err))
		return ""
	}
	return val
}

// getExifTagFloat holt den Wert eines EXIF-Tags als Float64
func getExifTagFloat(x *exif.Exif, tagName exif.FieldName) (float64, error) {
	tag, err := x.Get(tagName)
	if err != nil {
		return 0, err
	}
	val, err := tag.Float(0)
	if err != nil {
		log.Warn(fmt.Sprintf("Error getting float value for tag %s: %v", tagName, err))
		return 0, err
	}
	return val, nil
}

// ExtractMetadataFromImage extracts EXIF metadata and dimensions from an image file
func ExtractMetadataFromImage(image *models.Image, filePath string) error {
	// Open the image file with imaging for dimension extraction
	img, err := imaging.Open(filePath)
	if err != nil {
		log.Error(fmt.Sprintf("Error opening image to get dimensions: %v", err))
		return fmt.Errorf("error opening image: %w", err)
	}

	// Extract dimensions
	width := img.Bounds().Dx()
	height := img.Bounds().Dy()

	// Set dimensions in the image model
	image.Width = width
	image.Height = height

	// Open the file for EXIF extraction
	file, err := os.Open(filePath)
	if err != nil {
		log.Error(fmt.Sprintf("Error opening file for EXIF extraction: %v", err))
		return fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	// Try to extract EXIF data
	x, err := exif.Decode(file)
	if err != nil {
		// If no EXIF data is found, just return with dimensions
		log.Warn(fmt.Sprintf("No EXIF data found: %v", err))
		return nil
	}

	// Collect all metadata in a map
	allMetadata := make(map[string]interface{})

	// Extrahiere wichtige EXIF-Tags direkt
	for _, name := range []exif.FieldName{
		exif.Model,
		exif.DateTime,
		exif.DateTimeOriginal,
		exif.ExposureTime,
		exif.FNumber,
		exif.ISOSpeedRatings,
		exif.FocalLength,
		exif.GPSLatitude,
		exif.GPSLongitude,
	} {
		tag, err := x.Get(name)
		if err == nil {
			val, err := tag.StringVal()
			if err == nil {
				allMetadata[string(name)] = val
			}
		}
	}

	// 1. Camera Model
	image.CameraModel = getExifTagValue(x, exif.Model)

	// 2. Date Taken
	dateTimeTag, err := x.DateTime()
	if err == nil {
		image.TakenAt = &dateTimeTag
	}

	// 3. GPS Coordinates
	lat, long, err := x.LatLong()
	if err == nil {
		image.Latitude = &lat
		image.Longitude = &long
	}

	// 4. Exposure Time
	expVal, err := getExifTagFloat(x, exif.ExposureTime)
	if err == nil {
		image.ExposureTime = fmt.Sprintf("%.1f", expVal)
	} else {
		image.ExposureTime = getExifTagValue(x, exif.ExposureTime)
	}

	// 5. Aperture
	apertureVal, err := getExifTagFloat(x, exif.FNumber)
	if err == nil {
		image.Aperture = fmt.Sprintf("f/%.1f", apertureVal)
	} else {
		image.Aperture = getExifTagValue(x, exif.FNumber)
	}

	// 6. ISO
	isoVal, err := getExifTagFloat(x, exif.ISOSpeedRatings)
	if err == nil {
		iso := int(isoVal)
		image.ISO = &iso
	} else {
		image.ISO = nil
	}

	// 7. Focal Length
	flVal, err := getExifTagFloat(x, exif.FocalLength)
	if err == nil {
		image.FocalLength = fmt.Sprintf("%.1fmm", flVal)
	} else {
		image.FocalLength = getExifTagValue(x, exif.FocalLength)
	}

	metadataJSON, err := json.Marshal(allMetadata)
	if err != nil {
		log.Error(fmt.Sprintf("Error marshaling metadata to JSON: %v", err))
		// Continue even if JSON marshaling fails
	} else {
		image.Metadata = models.JSON(metadataJSON)
	}

	return nil
}

// ExtractMetadataByUUID extracts metadata for an image by its UUID
func ExtractMetadataByUUID(uuid string, filePath string) error {
	// Find the image in the database
	db := database.GetDB()
	var image models.Image
	result := db.Where("uuid = ?", uuid).First(&image)
	if result.Error != nil {
		return fmt.Errorf("error finding image with UUID %s: %w", uuid, result.Error)
	}

	// Extract metadata
	return ExtractMetadataFromImage(&image, filePath)
}
