package imageprocessor

import (
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/disintegration/imaging"
	"github.com/gofiber/fiber/v2/log"
	"github.com/kolesa-team/go-webp/encoder"
	"github.com/kolesa-team/go-webp/webp"
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
	MaxWorkers    = 10
)

// ImageProcessor handles image processing with a worker pool
type ImageProcessor struct {
	jobs    chan *ProcessJob
	wg      sync.WaitGroup
	started bool
	mutex   sync.Mutex
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
			jobs: make(chan *ProcessJob, 100),
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
		log.Info(fmt.Sprintf("[ImageProcessor] Worker %d processing image %s", id, job.Image.UUID))
		err := processImage(job.Image, job.OriginalPath)
		if err != nil {
			log.Error(fmt.Sprintf("[ImageProcessor] Worker %d failed to process image %s: %v", id, job.Image.UUID, err))
		} else {
			log.Info(fmt.Sprintf("[ImageProcessor] Worker %d completed processing image %s", id, job.Image.UUID))
		}
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
	GetProcessor().EnqueueImage(image, originalPath)
	return nil
}

// processImage handles the actual image processing
func processImage(image *models.Image, originalPath string) error {
	log.Info(fmt.Sprintf("[ImageProcessor] Processing image %s", image.UUID))

	originalDir := filepath.Dir(originalPath)

	// Remove "uploads/original/" from path
	relativePath := strings.Replace(originalDir, OriginalDir+"/", "", 1)
	relativePath = strings.Replace(relativePath, "./"+OriginalDir+"/", "", 1)

	fileName := filepath.Base(originalPath)
	fileExt := filepath.Ext(fileName)
	// Verwende UUID als Basis für neue Dateinamen, um Probleme mit Sonderzeichen zu vermeiden
	fileNameWithoutExt := image.UUID

	// Check if image is a GIF
	isGif := strings.ToLower(fileExt) == ".gif"
	// Check if image is already WebP or AVIF
	isWebP := strings.ToLower(fileExt) == ".webp"
	isAVIF := strings.ToLower(fileExt) == ".avif"
	// Skip optimization for GIF, WebP and AVIF
	skipOptimization := isGif || isWebP || isAVIF

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
		// For GIF, WebP and AVIF, only add thumbnail AVIF directories
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

	// Special handling for WebP and AVIF - skip processing and just copy the file
	if isWebP || isAVIF {
		var formatName string
		if isWebP {
			formatName = "WebP"
		} else {
			formatName = "AVIF"
		}
		
		log.Info(fmt.Sprintf("[ImageProcessor] %s is already in %s format, skipping optimization", fileName, formatName))
		
		// Versuche, die Bilddimensionen zu ermitteln
		width := 0
		height := 0
		
		// Öffne das Bild, um die Dimensionen zu ermitteln
		img, err := imaging.Open(originalPath)
		if err != nil {
			log.Error(fmt.Sprintf("Error opening image to get dimensions: %v", err))
		} else {
			// Dimensionen ermitteln
			width = img.Bounds().Dx()
			height = img.Bounds().Dy()
			log.Info(fmt.Sprintf("[ImageProcessor] Image dimensions: %dx%d", width, height))
		}
		
		// Update database - Wichtig: has_webp und has_avif auf false setzen, da keine optimierten Versionen existieren
		// Wir verwenden direkt das Original, daher brauchen wir keine optimierten Versionen
		db := database.GetDB()
		db.Model(image).Updates(map[string]interface{}{
			"has_webp":             false,  // Keine optimierte WebP-Version vorhanden
			"has_avif":             false,  // Keine optimierte AVIF-Version vorhanden
			"has_thumbnail_small":  false,
			"has_thumbnail_medium": false,
			"width":               width,  // Breite des Bildes
			"height":              height, // Höhe des Bildes
		})
		
		log.Info(fmt.Sprintf("[ImageProcessor] Image processing completed for %s", image.UUID))
		return nil
	}

	// Open the image for processing
	img, err := imaging.Open(originalPath)
	if err != nil {
		return fmt.Errorf("error opening original image: %w", err)
	}
	
	// Dimensionen des Bildes ermitteln und speichern
	width := img.Bounds().Dx()
	height := img.Bounds().Dy()
	log.Info(fmt.Sprintf("[ImageProcessor] Image dimensions: %dx%d", width, height))

	// Create thumbnails
	smallThumb := imaging.Resize(img, SmallThumbnailSize, 0, imaging.Lanczos)
	mediumThumb := imaging.Resize(img, MediumThumbnailSize, 0, imaging.Lanczos)

	// Define thumbnail paths
	smallWebP := filepath.Join(ThumbnailsDir, "small", "webp", relativePath, fileNameWithoutExt+".webp")
	mediumWebP := filepath.Join(ThumbnailsDir, "medium", "webp", relativePath, fileNameWithoutExt+".webp")

	// Save WebP thumbnails
	if err := saveWebP(smallThumb, smallWebP); err != nil {
		log.Error(fmt.Sprintf("Error saving small WebP thumbnail: %v", err))
	} else {
		log.Info(fmt.Sprintf("[ImageProcessor] Small WebP thumbnail created: %s", smallWebP))
	}

	if err := saveWebP(mediumThumb, mediumWebP); err != nil {
		log.Error(fmt.Sprintf("Error saving medium WebP thumbnail: %v", err))
	} else {
		log.Info(fmt.Sprintf("[ImageProcessor] Medium WebP thumbnail created: %s", mediumWebP))
	}

	// Process optimized versions only for images that need optimization
	hasWebp := true
	hasAvif := false

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
	}

	// AVIF conversion only if ffmpeg is available
	if haveFfmpeg {
		// Temporary JPEG files for AVIF conversion
		tempSmall := filepath.Join("temp", fileNameWithoutExt+"_small.jpg")
		tempMedium := filepath.Join("temp", fileNameWithoutExt+"_medium.jpg")
		tempFiles := []string{tempSmall, tempMedium}

		// For non-GIF images, also create optimized AVIF
		tempOriginal := ""
		if !skipOptimization {
			tempOriginal = filepath.Join("temp", fileNameWithoutExt+"_original.jpg")
			tempFiles = append(tempFiles, tempOriginal)
		}

		// Save temporary files
		saveError := false

		// Save thumbnails to temp files
		if err := imaging.Save(smallThumb, tempSmall); err != nil {
			log.Error(fmt.Sprintf("Error saving temporary small thumbnail: %v", err))
			saveError = true
		}

		if err := imaging.Save(mediumThumb, tempMedium); err != nil {
			log.Error(fmt.Sprintf("Error saving temporary medium thumbnail: %v", err))
			saveError = true
		}

		// Save original to temp file for images that need optimization
		if !skipOptimization && !saveError {
			if err := imaging.Save(img, tempOriginal); err != nil {
				log.Error(fmt.Sprintf("Error saving temporary original image: %v", err))
				saveError = true
			}
		}

		if !saveError {
			// Define AVIF paths for thumbnails
			smallAVIF := filepath.Join(ThumbnailsDir, "small", "avif", relativePath, fileNameWithoutExt+".avif")
			mediumAVIF := filepath.Join(ThumbnailsDir, "medium", "avif", relativePath, fileNameWithoutExt+".avif")

			// Define optimized AVIF path for images that need optimization
			optimizedAVIF := ""
			if !skipOptimization {
				optimizedAVIF = filepath.Join(OptimizedDir, "avif", relativePath, fileNameWithoutExt+".avif")
			}

			// Track AVIF conversion errors
			avifErrors := false

			// Convert thumbnails to AVIF
			if err := convertToAVIF(tempSmall, smallAVIF); err != nil {
				log.Error(fmt.Sprintf("Error creating small AVIF thumbnail: %v", err))
				avifErrors = true
			} else {
				log.Info(fmt.Sprintf("[ImageProcessor] Small AVIF thumbnail created: %s", smallAVIF))
			}

			if err := convertToAVIF(tempMedium, mediumAVIF); err != nil {
				log.Error(fmt.Sprintf("Error creating medium AVIF thumbnail: %v", err))
				avifErrors = true
			} else {
				log.Info(fmt.Sprintf("[ImageProcessor] Medium AVIF thumbnail created: %s", mediumAVIF))
			}

			// Convert original to AVIF for images that need optimization
			if !skipOptimization {
				if err := convertToAVIF(tempOriginal, optimizedAVIF); err != nil {
					log.Error(fmt.Sprintf("Error creating optimized AVIF version: %v", err))
					avifErrors = true
				} else {
					log.Info(fmt.Sprintf("[ImageProcessor] AVIF version created: %s", optimizedAVIF))
				}
			}

			// Set hasAvif only if no errors occurred
			hasAvif = !avifErrors
		}

		// Clean up temporary files
		for _, file := range tempFiles {
			os.Remove(file)
		}
	}

	// Update database
	db := database.GetDB()
	db.Model(image).Updates(map[string]interface{}{
		"has_webp":             hasWebp,
		"has_avif":             hasAvif,
		"has_thumbnail_small":  true,
		"has_thumbnail_medium": true,
		"width":               width,  // Breite des Bildes
		"height":              height, // Höhe des Bildes
	})

	log.Info(fmt.Sprintf("[ImageProcessor] Image processing completed for %s", image.UUID))
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

// GetImagePath returns the path to a specific image version
func GetImagePath(image *models.Image, format string, size string) string {
	// Extract file information from file path
	// Remove the "uploads/original/" part from the path
	relativePath := strings.Replace(image.FilePath, OriginalDir+"/", "", 1)
	relativePath = strings.Replace(relativePath, "./"+OriginalDir+"/", "", 1)
	fileNameWithoutExt := image.UUID

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
	default:
		// Fallback to original
		return filepath.Join(image.FilePath, image.UUID+image.FileType)
	}
}
