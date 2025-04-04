package imageprocessor

import (
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/disintegration/imaging"
	"github.com/gofiber/fiber/v2/log"
	"github.com/kolesa-team/go-webp/encoder"
	"github.com/kolesa-team/go-webp/webp"
)

// Thumbnail-Größen
const (
	SmallThumbnailSize  = 200
	MediumThumbnailSize = 500
)

// Verzeichnispfade
const (
	OriginalDir   = "uploads/original"
	OptimizedDir  = "uploads/optimized"
	ThumbnailsDir = "uploads/thumbnails"
)

// ProcessImage verarbeitet ein hochgeladenes Bild und erstellt optimierte Versionen
func ProcessImage(image *models.Image, originalPath string) error {
	log.Info(fmt.Sprintf("[ImageProcessor] Starte Verarbeitung für Bild %s", image.UUID))

	img, err := imaging.Open(originalPath)
	if err != nil {
		return fmt.Errorf("fehler beim Öffnen des Originalbildes: %w", err)
	}

	originalDir := filepath.Dir(originalPath)
	log.Info(fmt.Sprintf("[ProcessImage] Original Dir: %s", originalDir))

	// Entferne "uploads/original/" aus dem Pfad mit String-Ersetzung
	relativePath := strings.Replace(originalDir, OriginalDir+"/", "", 1)
	relativePath = strings.Replace(relativePath, "./"+OriginalDir+"/", "", 1)

	fileName := filepath.Base(originalPath)
	fileExt := filepath.Ext(fileName)
	fileNameWithoutExt := strings.TrimSuffix(fileName, fileExt)

	// Erstelle die Verzeichnisstruktur
	dirs := []string{
		filepath.Join(OptimizedDir, "webp", relativePath),
		filepath.Join(ThumbnailsDir, "small", "webp", relativePath),
		filepath.Join(ThumbnailsDir, "medium", "webp", relativePath),
		filepath.Join("temp"),
	}

	// Prüfe, ob ffmpeg verfügbar ist
	haveFfmpeg := checkFfmpegAvailable()
	if haveFfmpeg {
		// Füge AVIF-Verzeichnisse hinzu, wenn ffmpeg verfügbar ist
		dirs = append(dirs,
			filepath.Join(OptimizedDir, "avif", relativePath),
			filepath.Join(ThumbnailsDir, "small", "avif", relativePath),
			filepath.Join(ThumbnailsDir, "medium", "avif", relativePath),
		)
	} else {
		log.Warn("[ImageProcessor] ffmpeg nicht gefunden, AVIF-Konvertierung wird übersprungen")
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("fehler beim Erstellen des Verzeichnisses %s: %w", dir, err)
		}
	}

	// Thumbnails erstellen
	smallThumb := imaging.Resize(img, SmallThumbnailSize, 0, imaging.Lanczos)
	mediumThumb := imaging.Resize(img, MediumThumbnailSize, 0, imaging.Lanczos)

	// Optimierter Versionen speichern
	optimizedWebP := filepath.Join(OptimizedDir, "webp", relativePath, fileNameWithoutExt+".webp")
	smallWebP := filepath.Join(ThumbnailsDir, "small", "webp", relativePath, fileNameWithoutExt+".webp")
	mediumWebP := filepath.Join(ThumbnailsDir, "medium", "webp", relativePath, fileNameWithoutExt+".webp")

	// WebP-Versionen speichern
	if err := saveWebP(img, optimizedWebP); err != nil {
		log.Error(fmt.Sprintf("Fehler beim Speichern der optimierten WebP-Version: %v", err))
	} else {
		log.Info(fmt.Sprintf("[ImageProcessor] WebP-Version erstellt: %s", optimizedWebP))
	}

	if err := saveWebP(smallThumb, smallWebP); err != nil {
		log.Error(fmt.Sprintf("Fehler beim Speichern des kleinen WebP-Thumbnails: %v", err))
	} else {
		log.Info(fmt.Sprintf("[ImageProcessor] Kleines WebP-Thumbnail erstellt: %s", smallWebP))
	}

	if err := saveWebP(mediumThumb, mediumWebP); err != nil {
		log.Error(fmt.Sprintf("Fehler beim Speichern des mittleren WebP-Thumbnails: %v", err))
	} else {
		log.Info(fmt.Sprintf("[ImageProcessor] Mittleres WebP-Thumbnail erstellt: %s", mediumWebP))
	}

	// AVIF-Konvertierung nur durchführen, wenn ffmpeg verfügbar ist
	hasAvif := false
	if haveFfmpeg {
		// Temporäre JPEG-Dateien für AVIF-Konvertierung
		tempOriginal := filepath.Join("temp", fileNameWithoutExt+"_original.jpg")
		tempSmall := filepath.Join("temp", fileNameWithoutExt+"_small.jpg")
		tempMedium := filepath.Join("temp", fileNameWithoutExt+"_medium.jpg")

		// Temporäre Dateien speichern
		if err := imaging.Save(img, tempOriginal); err != nil {
			log.Error(fmt.Sprintf("Fehler beim Speichern der temporären Originaldatei: %v", err))
		} else if err := imaging.Save(smallThumb, tempSmall); err != nil {
			log.Error(fmt.Sprintf("Fehler beim Speichern der temporären kleinen Thumbnail-Datei: %v", err))
		} else if err := imaging.Save(mediumThumb, tempMedium); err != nil {
			log.Error(fmt.Sprintf("Fehler beim Speichern der temporären mittleren Thumbnail-Datei: %v", err))
		} else {
			// AVIF-Pfade
			optimizedAVIF := filepath.Join(OptimizedDir, "avif", relativePath, fileNameWithoutExt+".avif")
			smallAVIF := filepath.Join(ThumbnailsDir, "small", "avif", relativePath, fileNameWithoutExt+".avif")
			mediumAVIF := filepath.Join(ThumbnailsDir, "medium", "avif", relativePath, fileNameWithoutExt+".avif")

			// AVIF-Versionen erstellen
			avifErrors := false
			if err := convertToAVIF(tempOriginal, optimizedAVIF); err != nil {
				log.Error(fmt.Sprintf("Fehler beim Erstellen der optimierten AVIF-Version: %v", err))
				avifErrors = true
			} else {
				log.Info(fmt.Sprintf("[ImageProcessor] AVIF-Version erstellt: %s", optimizedAVIF))
			}

			if err := convertToAVIF(tempSmall, smallAVIF); err != nil {
				log.Error(fmt.Sprintf("Fehler beim Erstellen des kleinen AVIF-Thumbnails: %v", err))
				avifErrors = true
			} else {
				log.Info(fmt.Sprintf("[ImageProcessor] Kleines AVIF-Thumbnail erstellt: %s", smallAVIF))
			}

			if err := convertToAVIF(tempMedium, mediumAVIF); err != nil {
				log.Error(fmt.Sprintf("Fehler beim Erstellen des mittleren AVIF-Thumbnails: %v", err))
				avifErrors = true
			} else {
				log.Info(fmt.Sprintf("[ImageProcessor] Mittleres AVIF-Thumbnail erstellt: %s", mediumAVIF))
			}

			// Temporäre Dateien löschen
			os.Remove(tempOriginal)
			os.Remove(tempSmall)
			os.Remove(tempMedium)

			// Setze hasAvif nur, wenn keine Fehler aufgetreten sind
			hasAvif = !avifErrors
		}
	}

	// Datenbank aktualisieren
	db := database.GetDB()
	db.Model(image).Updates(map[string]interface{}{
		"has_webp":        true,
		"has_avif":        hasAvif,
		"has_thumbnails":  true,
		"thumbnail_sizes": "small,medium",
	})

	log.Info(fmt.Sprintf("[ImageProcessor] Bildverarbeitung abgeschlossen für %s", image.UUID))
	return nil
}

// saveWebP speichert ein Bild im WebP-Format
func saveWebP(img image.Image, outputPath string) error {
	// Stelle sicher, dass das Verzeichnis existiert
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("fehler beim Erstellen des Verzeichnisses: %w", err)
	}

	// Öffne die Ausgabedatei
	output, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("fehler beim Erstellen der WebP-Datei: %w", err)
	}
	defer output.Close()

	// Konfiguriere den WebP-Encoder
	options, err := encoder.NewLossyEncoderOptions(encoder.PresetDefault, 85)
	if err != nil {
		return fmt.Errorf("fehler beim Erstellen der Encoder-Optionen: %w", err)
	}

	// Konvertiere und speichere das Bild
	if err := webp.Encode(output, img, options); err != nil {
		return fmt.Errorf("fehler beim Kodieren des WebP-Bildes: %w", err)
	}

	return nil
}

// convertToAVIF konvertiert ein Bild zu AVIF mit ffmpeg
func convertToAVIF(inputPath, outputPath string) error {
	// Stelle sicher, dass das Verzeichnis existiert
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}

	// Verwende ffmpeg für die Konvertierung
	cmd := exec.Command("ffmpeg", "-i", inputPath, "-c:v", "libaom-av1", "-crf", "30", "-b:v", "0", "-y", outputPath)
	return cmd.Run()
}

// checkFfmpegAvailable prüft, ob ffmpeg verfügbar ist
func checkFfmpegAvailable() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

// GetImagePath gibt den Pfad zu einer bestimmten Bildversion zurück
func GetImagePath(image *models.Image, format string, size string) string {
	// Extrahiere Dateiinformationen aus dem Dateipfad
	// Entferne den "uploads/original/"-Teil aus dem Pfad
	relativePath := strings.Replace(image.FilePath, OriginalDir+"/", "", 1)
	relativePath = strings.Replace(relativePath, "./"+OriginalDir+"/", "", 1)
	fileNameWithoutExt := strings.TrimSuffix(image.FileName, filepath.Ext(image.FileName))

	switch {
	case size == "" && format == "webp":
		return filepath.Join(OptimizedDir, "webp", relativePath, fileNameWithoutExt+".webp")
	case size == "" && format == "avif":
		return filepath.Join(OptimizedDir, "avif", relativePath, fileNameWithoutExt+".avif")
	case size == "small" && format == "webp":
		return filepath.Join(ThumbnailsDir, "small", "webp", relativePath, fileNameWithoutExt+".webp")
	case size == "small" && format == "avif":
		return filepath.Join(ThumbnailsDir, "small", "avif", relativePath, fileNameWithoutExt+".avif")
	case size == "medium" && format == "webp":
		return filepath.Join(ThumbnailsDir, "medium", "webp", relativePath, fileNameWithoutExt+".webp")
	case size == "medium" && format == "avif":
		return filepath.Join(ThumbnailsDir, "medium", "avif", relativePath, fileNameWithoutExt+".avif")
	case size == "small" && format == "":
		// Originales Format für kleines Thumbnail
		return filepath.Join(ThumbnailsDir, "small", relativePath, fileNameWithoutExt+filepath.Ext(image.FileName))
	case size == "medium" && format == "":
		// Originales Format für mittleres Thumbnail
		return filepath.Join(ThumbnailsDir, "medium", relativePath, fileNameWithoutExt+filepath.Ext(image.FileName))
	default:
		// Fallback zum Original
		return filepath.Join(image.FilePath, image.UUID+image.FileType)
	}
}
