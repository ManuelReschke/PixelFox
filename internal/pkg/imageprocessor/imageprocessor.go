package imageprocessor

import (
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/disintegration/imaging"
	"github.com/gofiber/fiber/v2/log"
	"github.com/kolesa-team/go-webp/encoder"
	"github.com/kolesa-team/go-webp/webp"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
)

// Thumbnail sizes
const (
	SmallThumbnailSize  = 200
	MediumThumbnailSize = 500
)

// Directory paths
const (
	OriginalDir   = "uploads/original"
	OptimizedDir  = "uploads/optimized"
	ThumbnailsDir = "uploads/thumbnails"
	MaxWorkers    = 2
)

// ImageProcessor handles image processing with a worker pool
type ImageProcessor struct {
	jobs            chan *ProcessJob
	wg              sync.WaitGroup
	started         bool
	mutex           sync.Mutex
	activeProcesses int32         // Anzahl der aktuell aktiven Verarbeitungsprozesse
	memoryThrottle  chan struct{} // Semaphore zur Begrenzung der gleichzeitigen Verarbeitungen
}

// ProcessJob represents a single image processing job
type ProcessJob struct {
	Image        *models.Image
	OriginalPath string
}

// Global processor instance
var processor *ImageProcessor
var once sync.Once

// GetProcessor returns the singleton image processor instance
func GetProcessor() *ImageProcessor {
	once.Do(func() {
		processor = &ImageProcessor{
			jobs:           make(chan *ProcessJob, 100),
			memoryThrottle: make(chan struct{}, MaxWorkers), // Begrenze auf MaxWorkers gleichzeitige Verarbeitungen
		}
		processor.Start()
	})
	return processor
}

// Start initializes the worker pool
func (p *ImageProcessor) Start() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.started {
		return
	}

	p.started = true
	for i := 0; i < MaxWorkers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}

	log.Info("[ImageProcessor] Started worker pool with ", MaxWorkers, " workers")
}

// Stop gracefully shuts down the worker pool
func (p *ImageProcessor) Stop() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if !p.started {
		return
	}

	close(p.jobs)
	p.wg.Wait()
	p.started = false
	log.Info("[ImageProcessor] Worker pool stopped")
}

// worker processes jobs from the queue
func (p *ImageProcessor) worker(id int) {
	defer p.wg.Done()
	log.Info(fmt.Sprintf("[ImageProcessor] Worker %d started", id))

	for job := range p.jobs {
		// Warte auf einen freien Platz im Speicher-Semaphore
		p.memoryThrottle <- struct{}{}

		// Erhu00f6he den Zu00e4hler fu00fcr aktive Prozesse
		atomic.AddInt32(&p.activeProcesses, 1)

		log.Info(fmt.Sprintf("[ImageProcessor] Worker %d processing image %s (Active: %d)",
			id, job.Image.UUID, atomic.LoadInt32(&p.activeProcesses)))

		err := processImage(job.Image, job.OriginalPath)

		// Gib den Platz im Speicher-Semaphore frei
		<-p.memoryThrottle

		// Verringere den Zu00e4hler fu00fcr aktive Prozesse
		atomic.AddInt32(&p.activeProcesses, -1)

		if err != nil {
			log.Error(fmt.Sprintf("[ImageProcessor] Worker %d failed to process image %s: %v", id, job.Image.UUID, err))
		} else {
			log.Info(fmt.Sprintf("[ImageProcessor] Worker %d completed processing image %s", id, job.Image.UUID))
		}

		// Warte kurz, um dem GC Zeit zu geben
		time.Sleep(100 * time.Millisecond)
	}

	log.Info(fmt.Sprintf("[ImageProcessor] Worker %d stopped", id))
}

// EnqueueImage adds an image to the processing queue
func (p *ImageProcessor) EnqueueImage(image *models.Image, originalPath string) {
	if !p.started {
		p.Start()
	}

	p.jobs <- &ProcessJob{
		Image:        image,
		OriginalPath: originalPath,
	}
	log.Info(fmt.Sprintf("[ImageProcessor] Enqueued image %s for processing", image.UUID))
}

// ProcessImage queues an image for processing
func ProcessImage(image *models.Image, originalPath string) error {
	// Setze initialen Status auf "pending"
	SetImageStatus(image.UUID, STATUS_PENDING)
	GetProcessor().EnqueueImage(image, originalPath)
	return nil
}

// ProcessImageByUUID queues an image for processing by its UUID
func ProcessImageByUUID(uuid string) error {
	// Finde das Bild in der Datenbank
	db := database.GetDB()
	var image models.Image
	result := db.Where("uuid = ?", uuid).First(&image)
	if result.Error != nil {
		return fmt.Errorf("error finding image with UUID %s: %w", uuid, result.Error)
	}

	// Hole den Pfad zur Originalvariante
	originalPath, err := GetOriginalPath(&image)
	if err != nil {
		return fmt.Errorf("error getting original path for image %s: %w", uuid, err)
	}

	// Setze initialen Status auf "pending"
	SetImageStatus(uuid, STATUS_PENDING)
	GetProcessor().EnqueueImage(&image, originalPath)
	return nil
}

// processImage handles the actual image processing
func processImage(image *models.Image, originalPath string) error {
	log.Info(fmt.Sprintf("[ImageProcessor] Processing image %s", image.UUID))

	// Status auf "processing" setzen
	SetImageStatus(image.UUID, STATUS_PROCESSING)

	// Extrahiere Metadaten aus dem Bild
	if err := ExtractMetadataByUUID(image.UUID, originalPath); err != nil {
		log.Error(fmt.Sprintf("[ImageProcessor] Error extracting metadata: %v", err))
		// Fahre mit der Verarbeitung fort, auch wenn die Metadatenextraktion fehlschlu00e4gt
	}

	originalDir := filepath.Dir(originalPath)

	// Remove "uploads/original/" from path
	relativePath := strings.Replace(originalDir, OriginalDir+"/", "", 1)
	relativePath = strings.Replace(relativePath, "./"+OriginalDir+"/", "", 1)

	fileName := filepath.Base(originalPath)
	fileExt := filepath.Ext(fileName)
	// Verwende UUID als Basis für neue Dateinamen, um Probleme mit Sonderzeichen zu vermeiden
	fileNameWithoutExt := image.UUID

	// Check if image is a GIF | SVG | WEBP | AVIF
	isGif := strings.ToLower(fileExt) == ".gif"
	isSVG := strings.ToLower(fileExt) == ".svg"
	isWebP := strings.ToLower(fileExt) == ".webp"
	isAVIF := strings.ToLower(fileExt) == ".avif"
	// Skip optimization for GIF, SVG, WebP and AVIF
	skipOptimization := isGif || isSVG || isWebP || isAVIF

	// Create directory structure
	dirs := []string{
		filepath.Join(ThumbnailsDir, "small", "webp", relativePath),
		filepath.Join(ThumbnailsDir, "medium", "webp", relativePath),
	}

	// Add optimized directories only for images that need optimization
	if !skipOptimization {
		dirs = append(dirs, filepath.Join(OptimizedDir, "webp", relativePath))
	}

	// Check if ffmpeg is available
	haveFfmpeg := checkFfmpegAvailable()
	if haveFfmpeg && !skipOptimization {
		// Add AVIF directories if ffmpeg is available and optimization is needed
		dirs = append(dirs,
			filepath.Join(OptimizedDir, "avif", relativePath),
			filepath.Join(ThumbnailsDir, "small", "avif", relativePath),
			filepath.Join(ThumbnailsDir, "medium", "avif", relativePath),
		)
	} else if haveFfmpeg {
		// For GIF, WebP, AVIF and SVG, only add thumbnail AVIF directories
		dirs = append(dirs,
			filepath.Join(ThumbnailsDir, "small", "avif", relativePath),
			filepath.Join(ThumbnailsDir, "medium", "avif", relativePath),
		)
	} else {
		log.Warn("[ImageProcessor] ffmpeg not found, skipping AVIF conversion")
	}

	// Add temp directory
	dirs = append(dirs, filepath.Join("temp"))

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("error creating directory %s: %w", dir, err)
		}
	}

	// Special handling for WebP, AVIF and SVG - skip processing and just copy the file
	if isWebP || isAVIF || isSVG {
		var formatName string
		if isWebP {
			formatName = "WebP"
		} else if isAVIF {
			formatName = "AVIF"
		} else {
			formatName = "SVG"
		}

		log.Info(fmt.Sprintf("[ImageProcessor] %s is already in %s format, skipping optimization", fileName, formatName))

		// Extract metadata and dimensions from the image
		if err := ExtractMetadataByUUID(image.UUID, originalPath); err != nil {
			log.Error(fmt.Sprintf("[ImageProcessor] Error extracting metadata for %s format: %v", formatName, err))
			// Continue processing even if metadata extraction fails
		}

		// Get file size
		fileInfo, err := os.Stat(originalPath)
		if err != nil {
			log.Error(fmt.Sprintf("[ImageProcessor] Error getting file size: %v", err))
		}
		fileSize := fileInfo.Size()

		// Save the original as a variant
		if err := SaveOriginalVariant(image, originalPath, image.Width, image.Height, fileSize); err != nil {
			log.Error(fmt.Sprintf("[ImageProcessor] Error saving original variant: %v", err))
		}

		// Update database with metadata
		db := database.GetDB()
		db.Model(image).Updates(map[string]interface{}{
			"width":         image.Width,
			"height":        image.Height,
			"filesize":      fileSize,
			"camera_model":  image.CameraModel,
			"taken_at":      image.TakenAt,
			"latitude":      image.Latitude,
			"longitude":     image.Longitude,
			"exposure_time": image.ExposureTime,
			"aperture":      image.Aperture,
			"iso":           image.ISO,
			"focal_length":  image.FocalLength,
			"metadata":      image.Metadata,
		})
	} else {
		// Verarbeite Bilder in Blöcken, um Speicherverbrauch zu reduzieren
		hasWebp := true
		hasAvif := false

		// Block 1: Öffne Bild und extrahiere Metadaten und Dimensionen
		{
			// Extract metadata and dimensions from the image
			if err := ExtractMetadataByUUID(image.UUID, originalPath); err != nil {
				log.Error(fmt.Sprintf("[ImageProcessor] Error extracting metadata: %v", err))
				// Continue processing even if metadata extraction fails
			}

			// Open the image for processing
			img, err := imaging.Open(originalPath)
			if err != nil {
				return fmt.Errorf("error opening original image: %w", err)
			}

			// Block 2: Erstelle und speichere kleine Thumbnails
			smallThumb := imaging.Resize(img, SmallThumbnailSize, 0, imaging.Lanczos)
			smallWebP := filepath.Join(ThumbnailsDir, "small", "webp", relativePath, fileNameWithoutExt+".webp")
			if err := saveWebP(smallThumb, smallWebP); err != nil {
				log.Error(fmt.Sprintf("Error saving small WebP thumbnail: %v", err))
			} else {
				log.Info(fmt.Sprintf("[ImageProcessor] Small WebP thumbnail created: %s", smallWebP))
			}

			// Speichere temporäre Datei für AVIF, falls nötig
			if haveFfmpeg {
				tempSmall := filepath.Join("temp", fileNameWithoutExt+"_small.jpg")
				if err := imaging.Save(smallThumb, tempSmall); err != nil {
					log.Error(fmt.Sprintf("Error saving temporary small thumbnail: %v", err))
				} else {
					// Konvertiere zu AVIF
					smallAVIF := filepath.Join(ThumbnailsDir, "small", "avif", relativePath, fileNameWithoutExt+".avif")
					if err := convertToAVIF(tempSmall, smallAVIF); err != nil {
						log.Error(fmt.Sprintf("Error creating small AVIF thumbnail: %v", err))
					} else {
						log.Info(fmt.Sprintf("[ImageProcessor] Small AVIF thumbnail created: %s", smallAVIF))
					}
					// Lösche temporäre Datei
					os.Remove(tempSmall)
				}
			}

			// Gib Speicher frei
			smallThumb = nil

			// Block 3: Erstelle und speichere mittlere Thumbnails
			mediumThumb := imaging.Resize(img, MediumThumbnailSize, 0, imaging.Lanczos)
			mediumWebP := filepath.Join(ThumbnailsDir, "medium", "webp", relativePath, fileNameWithoutExt+".webp")
			if err := saveWebP(mediumThumb, mediumWebP); err != nil {
				log.Error(fmt.Sprintf("Error saving medium WebP thumbnail: %v", err))
			} else {
				log.Info(fmt.Sprintf("[ImageProcessor] Medium WebP thumbnail created: %s", mediumWebP))
			}

			// Speichere temporäre Datei für AVIF, falls nötig
			if haveFfmpeg {
				tempMedium := filepath.Join("temp", fileNameWithoutExt+"_medium.jpg")
				if err := imaging.Save(mediumThumb, tempMedium); err != nil {
					log.Error(fmt.Sprintf("Error saving temporary medium thumbnail: %v", err))
				} else {
					// Konvertiere zu AVIF
					mediumAVIF := filepath.Join(ThumbnailsDir, "medium", "avif", relativePath, fileNameWithoutExt+".avif")
					if err := convertToAVIF(tempMedium, mediumAVIF); err != nil {
						log.Error(fmt.Sprintf("Error creating medium AVIF thumbnail: %v", err))
					} else {
						log.Info(fmt.Sprintf("[ImageProcessor] Medium AVIF thumbnail created: %s", mediumAVIF))
					}
					// Lösche temporäre Datei
					os.Remove(tempMedium)
				}
			}

			// Gib Speicher frei
			mediumThumb = nil

			// Block 4: Erstelle optimierte Versionen nur für Bilder, die Optimierung benötigen
			if !skipOptimization {
				// Define optimized WebP path
				optimizedWebP := filepath.Join(OptimizedDir, "webp", relativePath, fileNameWithoutExt+".webp")

				// Save optimized WebP version
				if err := saveWebP(img, optimizedWebP); err != nil {
					log.Error(fmt.Sprintf("Error saving optimized WebP version: %v", err))
					hasWebp = false
				} else {
					log.Info(fmt.Sprintf("[ImageProcessor] WebP version created: %s", optimizedWebP))
				}

				// AVIF conversion only if ffmpeg is available
				if haveFfmpeg {
					tempOriginal := filepath.Join("temp", fileNameWithoutExt+"_original.jpg")
					if err := imaging.Save(img, tempOriginal); err != nil {
						log.Error(fmt.Sprintf("Error saving temporary original image: %v", err))
					} else {
						// Konvertiere zu AVIF
						optimizedAVIF := filepath.Join(OptimizedDir, "avif", relativePath, fileNameWithoutExt+".avif")
						if err := convertToAVIF(tempOriginal, optimizedAVIF); err != nil {
							log.Error(fmt.Sprintf("Error creating optimized AVIF version: %v", err))
						} else {
							log.Info(fmt.Sprintf("[ImageProcessor] AVIF version created: %s", optimizedAVIF))
							hasAvif = true
						}
						// Lösche temporäre Datei
						os.Remove(tempOriginal)
					}
				}
			}

			// Gib Speicher frei - wichtig, um RAM zu sparen
			img = nil
		}

		// Erzwinge Garbage Collection nach der Bildverarbeitung
		runtime.GC()

		// Update database
		db := database.GetDB()
		db.Model(image).Updates(map[string]interface{}{
			"has_webp":             hasWebp,
			"has_avif":             hasAvif,
			"has_thumbnail_small":  true,
			"has_thumbnail_medium": true,
			"width":                image.Width,
			"height":               image.Height,
			"camera_model":         image.CameraModel,
			"taken_at":             image.TakenAt,
			"latitude":             image.Latitude,
			"longitude":            image.Longitude,
			"exposure_time":        image.ExposureTime,
			"aperture":             image.Aperture,
			"iso":                  image.ISO,
			"focal_length":         image.FocalLength,
			"metadata":             image.Metadata,
		})

		log.Info(fmt.Sprintf("[ImageProcessor] Image processing completed for %s", image.UUID))

		// Status auf "completed" setzen
		SetImageStatus(image.UUID, STATUS_COMPLETED)
	}
	return nil
}

// saveWebP saves an image in WebP format
func saveWebP(img image.Image, outputPath string) error {
	// Ensure directory exists
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("error creating directory: %w", err)
	}

	// Open output file
	output, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("error creating WebP file: %w", err)
	}
	defer output.Close()

	// Configure WebP encoder
	options, err := encoder.NewLossyEncoderOptions(encoder.PresetDefault, 85)
	if err != nil {
		return fmt.Errorf("error creating encoder options: %w", err)
	}

	// Convert and save image
	if err := webp.Encode(output, img, options); err != nil {
		return fmt.Errorf("error encoding WebP image: %w", err)
	}

	return nil
}

// convertToAVIF converts an image to AVIF using ffmpeg
func convertToAVIF(inputPath, outputPath string) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}

	// Use ffmpeg for conversion
	cmd := exec.Command("ffmpeg", "-i", inputPath, "-c:v", "libaom-av1", "-crf", "30", "-b:v", "0", "-y", outputPath)
	return cmd.Run()
}

// checkFfmpegAvailable checks if ffmpeg is available
func checkFfmpegAvailable() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

// GetImagePathOld gibt den Pfad zu einer bestimmten Bildvariante zurück
func GetImagePathOld(image *models.Image, format string, size string) string {
	// Diese Funktion wird durch die neue Varianten-Struktur ersetzt
	// und bleibt nur für Kompatibilität mit älterem Code erhalten
	return "/image/serve/" + image.UUID
}
