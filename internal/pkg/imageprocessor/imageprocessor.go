package imageprocessor

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/disintegration/imaging"
	"github.com/gofiber/fiber/v2/log"
	"github.com/kolesa-team/go-webp/encoder"
	"github.com/kolesa-team/go-webp/webp"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"gorm.io/gorm"
)

// Thumbnail sizes
const (
	SmallThumbnailSize  = 200
	MediumThumbnailSize = 500
)

// Directory paths and worker settings
var (
	// Processing limits to avoid overloading the server
	MaxWorkers = 3
	Throttler  = make(chan struct{}, MaxWorkers)

	// Image quality settings for WebP conversion - can be adjusted via config
	WebPQuality     = 90 // Default quality for WebP conversion (1-100)
	SmallThumbSize  = 200
	MediumThumbSize = 500

	// Directory Paths
	OriginalDir = "uploads/original"
	VariantsDir = "uploads/variants"

	// Tool availability flags
	IsPNGQuantAvailable  = false
	IsJPEGOptimAvailable = false
	IsFFmpegAvailable    = false

	// Function for database updates - can be mocked for testing
	UpdateImageRecordFunc = updateImageRecord
)

// init initializes global variables and settings
func init() {
	// Check if ffmpeg is available once at startup
	IsFFmpegAvailable = checkFfmpegAvailable()
	if !IsFFmpegAvailable {
		log.Warn("[ImageProcessor] ffmpeg not found in PATH. AVIF conversion will be disabled.")
	} else {
		log.Info("[ImageProcessor] ffmpeg found, AVIF conversion enabled.")
	}
}

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
	Image *models.Image
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
	})
	return processor
}

// Start initializes the worker pool
func (p *ImageProcessor) Start() {
	p.mutex.Lock()
	if p.started {
		p.mutex.Unlock()
		return
	}
	p.started = true
	p.mutex.Unlock()

	p.wg.Add(MaxWorkers)
	for i := 0; i < MaxWorkers; i++ {
		go p.worker(i)
	}
	log.Info("[ImageProcessor] Started worker pool with ", MaxWorkers, " workers")
}

// Stop gracefully shuts down the worker pool
func (p *ImageProcessor) Stop() {
	p.mutex.Lock()
	if !p.started {
		p.mutex.Unlock()
		return
	}
	log.Info("[ImageProcessor] Stopping worker pool...")
	close(p.jobs)
	p.started = false
	p.mutex.Unlock()
	p.wg.Wait()
	log.Info("[ImageProcessor] Worker pool stopped")
}

// worker processes jobs from the queue
func (p *ImageProcessor) worker(id int) {
	defer p.wg.Done()
	log.Infof("[ImageProcessor] Worker %d started", id)

	for job := range p.jobs {
		if job == nil || job.Image == nil {
			log.Warnf("[ImageProcessor] Worker %d received nil job, skipping", id)
			continue
		}

		p.memoryThrottle <- struct{}{}
		log.Debugf("[ImageProcessor] Worker %d acquired throttle for image %s", id, job.Image.UUID)
		currentActive := atomic.AddInt32(&p.activeProcesses, 1)
		log.Infof("[ImageProcessor] Worker %d processing image %s (Active: %d)", id, job.Image.UUID, currentActive)

		// Process the image - processImage handles status updates internally now using cache
		err := processImage(job.Image)

		atomic.AddInt32(&p.activeProcesses, -1)
		<-p.memoryThrottle
		log.Debugf("[ImageProcessor] Worker %d released throttle for image %s", id, job.Image.UUID)

		if err != nil {
			log.Errorf("[ImageProcessor] Worker %d finished processing image %s with error", id, job.Image.UUID)
			// Status FAILED is set within processImage's defer block using SetImageStatus (cache)
		} else {
			log.Infof("[ImageProcessor] Worker %d completed processing image %s successfully", id, job.Image.UUID)
			// Status COMPLETED is set at the end of processImage using SetImageStatus (cache)
		}
		job = nil
	}
	log.Infof("[ImageProcessor] Worker %d stopped", id)
}

// EnqueueImage adds an image to the processing queue
func (p *ImageProcessor) EnqueueImage(image *models.Image) error {
	if image == nil || image.UUID == "" {
		return fmt.Errorf("cannot enqueue invalid image data")
	}
	proc := GetProcessor()

	proc.mutex.Lock()
	if !proc.started {
		proc.mutex.Unlock()
		proc.Start()
	} else {
		proc.mutex.Unlock()
	}

	job := &ProcessJob{Image: image}

	select {
	case proc.jobs <- job:
		log.Infof("[ImageProcessor] Enqueued image %s for processing", image.UUID)
		return nil
	default:
		log.Errorf("[ImageProcessor] Failed to enqueue image %s: job channel likely full or closed.", image.UUID)
		// Use the new SetImageStatus (cache) on enqueue failure
		if err := SetImageStatus(image.UUID, STATUS_FAILED); err != nil {
			log.Errorf("[ImageProcessor] Additionally failed to set FAILED status in cache for %s: %v", image.UUID, err)
		}
		return fmt.Errorf("failed to enqueue image %s: job channel busy or closed", image.UUID)
	}
}

// ProcessImage queues an image for processing (convenience function)
func ProcessImage(image *models.Image) error {
	if image == nil || image.UUID == "" {
		return fmt.Errorf("cannot process invalid image data")
	}
	// Set initial status to PENDING using the new cache function
	if err := SetImageStatus(image.UUID, STATUS_PENDING); err != nil {
		log.Errorf("[ImageProcessor] Failed to set initial PENDING status in cache for %s: %v", image.UUID, err)
		// Decide if we should still enqueue or return error
		// Let's return error here, as setting initial state failed
		return fmt.Errorf("failed to set initial pending status for %s: %w", image.UUID, err)
	}
	return GetProcessor().EnqueueImage(image)
}

// processImage handles the actual image processing
func processImage(imageModel *models.Image) (errResult error) {
	log.Debugf("[ImageProcessor] Processing image: %s", imageModel.UUID)

	// Defer function to handle panics and ensure status is set to FAILED in cache on any error exit.
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[ImageProcessor] PANIC while processing image %s: %v", imageModel.UUID, r)
			errResult = fmt.Errorf("panic occurred during processing: %v", r)
		}
		if errResult != nil {
			log.Errorf("[ImageProcessor] Final Error for image %s: %v", imageModel.UUID, errResult)
			// Use the new SetImageStatus (cache)
			if statusErr := SetImageStatus(imageModel.UUID, STATUS_FAILED); statusErr != nil {
				log.Errorf("[ImageProcessor] Additionally failed to set FAILED status in cache for %s: %v", imageModel.UUID, statusErr)
			}
		}
	}()

	// Set status to PROCESSING in cache at the beginning of actual work
	if err := SetImageStatus(imageModel.UUID, STATUS_PROCESSING); err != nil {
		log.Errorf("[ImageProcessor] Failed to set PROCESSING status in cache for %s: %v", imageModel.UUID, err)
		// Continue processing, but return this error if nothing else fails
		errResult = fmt.Errorf("failed to set processing status: %w", err)
	}

	// Validation
	if imageModel == nil || imageModel.UUID == "" || imageModel.FilePath == "" || imageModel.FileName == "" {
		return fmt.Errorf("invalid image data provided") // Assign to errResult implicitly
	}

	originalFilePath := filepath.Join(imageModel.FilePath, imageModel.FileName)
	log.Debugf("[ImageProcessor] Original file path: %s", originalFilePath)

	if _, err := os.Stat(originalFilePath); os.IsNotExist(err) {
		return fmt.Errorf("original file not found: %s", originalFilePath)
	} else if err != nil {
		return fmt.Errorf("error accessing original file '%s': %w", originalFilePath, err)
	}

	relativePath := strings.TrimPrefix(imageModel.FilePath, OriginalDir)
	relativePath = strings.TrimPrefix(relativePath, string(filepath.Separator))
	log.Debugf("[ImageProcessor] Relative path for variants: %s", relativePath)

	variantsBaseDir := filepath.Join(VariantsDir, relativePath)
	baseFileName := imageModel.UUID

	if err := os.MkdirAll(variantsBaseDir, 0755); err != nil {
		return fmt.Errorf("failed to create variants directory: %w", err)
	}

	optimizedWebPPath := filepath.Join(variantsBaseDir, baseFileName+".webp")
	optimizedAVIFPath := filepath.Join(variantsBaseDir, baseFileName+".avif")
	smallThumbWebPPath := filepath.Join(variantsBaseDir, baseFileName+"_small.webp")
	smallThumbAVIFPath := filepath.Join(variantsBaseDir, baseFileName+"_small.avif")
	mediumThumbWebPPath := filepath.Join(variantsBaseDir, baseFileName+"_medium.webp")
	mediumThumbAVIFPath := filepath.Join(variantsBaseDir, baseFileName+"_medium.avif")

	var hasWebp, hasAvif, hasThumbnailSmall, hasThumbnailMedium bool
	var width, height int
	var smallThumb image.Image
	var mediumThumb image.Image
	defer func() {
		smallThumb = nil
		mediumThumb = nil
		log.Debugf("[ImageProcessor] Cleared thumbnail references for %s", imageModel.UUID)
	}()

	if err := ExtractMetadata(imageModel, originalFilePath); err != nil {
		log.Warnf("[ImageProcessor] Could not extract metadata for %s: %v. Processing continues.", imageModel.UUID, err)
	} else {
		log.Debugf("[ImageProcessor] Successfully extracted metadata for %s", imageModel.UUID)
	}

	lowerFilePath := strings.ToLower(originalFilePath)
	lowerFileType := strings.ToLower(strings.TrimPrefix(imageModel.FileType, "."))
	isAVIF := strings.HasSuffix(lowerFilePath, ".avif") || lowerFileType == "avif"

	log.Debugf("[ImageProcessor] Checking AVIF input: %s (Path: %s, Type: %s) => isAVIF=%v",
		imageModel.UUID, originalFilePath, imageModel.FileType, isAVIF)

	// --- Special Handling for AVIF input ---
	if isAVIF {
		log.Infof("[ImageProcessor] AVIF input file detected: %s", imageModel.UUID)
		if !IsFFmpegAvailable {
			return fmt.Errorf("ffmpeg/ffprobe not available for processing AVIF input")
		}
		var ffprobeErr error
		width, height, ffprobeErr = getImageDimensionsWithFFprobe(originalFilePath)
		if ffprobeErr != nil {
			return fmt.Errorf("failed to get AVIF dimensions: %w", ffprobeErr)
		}
		log.Infof("[ImageProcessor] AVIF dimensions successfully retrieved: %dx%d for %s", width, height, imageModel.UUID)
		hasWebp, hasAvif, hasThumbnailSmall, hasThumbnailMedium = false, false, false, false

		// Update Database record (flags, dimensions, metadata)
		if err := UpdateImageRecordFunc(imageModel, width, height, hasWebp, hasAvif, hasThumbnailSmall, hasThumbnailMedium); err != nil {
			return err // Return DB update error
		}
		log.Infof("[ImageProcessor] AVIF input file %s processed successfully (DB updated).", imageModel.UUID)

		// Set completed status in cache
		if err := SetImageStatus(imageModel.UUID, STATUS_COMPLETED); err != nil {
			log.Errorf("[ImageProcessor] Failed to set COMPLETED status in cache for AVIF input %s: %v", imageModel.UUID, err)
			return fmt.Errorf("failed to set final status for AVIF input: %w", err)
		}
		return nil // Success for AVIF handling
	}

	// --- Handling for non-AVIF input (e.g., JPEG, PNG, GIF, WebP) ---
	log.Debugf("[ImageProcessor] Opening and decoding image using imaging.Open: %s", originalFilePath)
	imgDecoded, err := imaging.Open(originalFilePath, imaging.AutoOrientation(true))
	if err != nil {
		return fmt.Errorf("failed to open/decode image '%s': %w", originalFilePath, err)
	}
	defer func() {
		imgDecoded = nil
		log.Debugf("[ImageProcessor] Cleared main decoded image reference for %s", imageModel.UUID)
	}()
	log.Debugf("[ImageProcessor] Successfully decoded image: %s", imageModel.UUID)

	bounds := imgDecoded.Bounds()
	width = bounds.Dx()
	height = bounds.Dy()
	log.Infof("[ImageProcessor] Processing image %s (%dx%d)", imageModel.UUID, width, height)

	isGif := strings.HasSuffix(strings.ToLower(originalFilePath), ".gif")

	// --- GIF Handling ---
	if isGif {
		log.Debugf("[ImageProcessor] GIF detected, creating WebP/AVIF thumbnails for %s", imageModel.UUID)
		// Small Thumbnail
		smallThumb = imaging.Resize(imgDecoded, SmallThumbnailSize, 0, imaging.Lanczos)
		if err := saveWebP(smallThumb, smallThumbWebPPath); err != nil {
			log.Errorf("[ImageProcessor] Failed to save small WebP thumbnail for GIF %s: %v", imageModel.UUID, err)
		} else {
			hasThumbnailSmall = true
			log.Debugf("[ImageProcessor] Saved small WebP thumbnail for GIF %s", imageModel.UUID)
			if IsFFmpegAvailable {
				if err := convertToAVIF(smallThumb, smallThumbAVIFPath); err != nil {
					log.Errorf("[ImageProcessor] Failed to create small AVIF thumbnail for GIF %s: %v", imageModel.UUID, err)
				} else {
					log.Debugf("[ImageProcessor] Saved small AVIF thumbnail for GIF %s", smallThumbAVIFPath)
				}
			}
		}
		smallThumb = nil
		// Medium Thumbnail
		mediumThumb = imaging.Resize(imgDecoded, MediumThumbnailSize, 0, imaging.Lanczos)
		if err := saveWebP(mediumThumb, mediumThumbWebPPath); err != nil {
			log.Errorf("[ImageProcessor] Failed to save medium WebP thumbnail for GIF %s: %v", imageModel.UUID, err)
		} else {
			hasThumbnailMedium = true
			log.Debugf("[ImageProcessor] Saved medium WebP thumbnail for GIF %s", imageModel.UUID)
			if IsFFmpegAvailable {
				if err := convertToAVIF(mediumThumb, mediumThumbAVIFPath); err != nil {
					log.Errorf("[ImageProcessor] Failed to create medium AVIF thumbnail for GIF %s: %v", imageModel.UUID, err)
				} else {
					log.Debugf("[ImageProcessor] Saved medium AVIF thumbnail for GIF %s", mediumThumbAVIFPath)
				}
			}
		}
		mediumThumb = nil
	} else {
		// --- Standard Image Handling ---
		log.Debugf("[ImageProcessor] Standard image detected, creating optimized WebP/AVIF and thumbnails for %s", imageModel.UUID)
		// Optimized WebP
		if err := saveWebP(imgDecoded, optimizedWebPPath); err != nil {
			log.Errorf("[ImageProcessor] Failed to create optimized WebP for %s: %v", imageModel.UUID, err)
		} else {
			hasWebp = true
			log.Debugf("[ImageProcessor] Saved optimized WebP for %s", imageModel.UUID)
		}
		// Optimized AVIF
		if IsFFmpegAvailable {
			if err := convertToAVIF(imgDecoded, optimizedAVIFPath); err != nil {
				log.Errorf("[ImageProcessor] Failed to convert to optimized AVIF for %s: %v", imageModel.UUID, err)
			} else {
				hasAvif = true
				log.Debugf("[ImageProcessor] Saved optimized AVIF for %s", imageModel.UUID)
			}
		} else {
			log.Warnf("[ImageProcessor] Skipping optimized AVIF conversion for %s: ffmpeg not found.", imageModel.UUID)
		}
		// Small Thumbnail
		smallThumb = imaging.Resize(imgDecoded, SmallThumbnailSize, 0, imaging.Lanczos)
		if err := saveWebP(smallThumb, smallThumbWebPPath); err != nil {
			log.Errorf("[ImageProcessor] Failed to save small WebP thumbnail for %s: %v", imageModel.UUID, err)
		} else {
			hasThumbnailSmall = true
			log.Debugf("[ImageProcessor] Saved small WebP thumbnail for %s", imageModel.UUID)
			if IsFFmpegAvailable {
				if err := convertToAVIF(smallThumb, smallThumbAVIFPath); err != nil {
					log.Errorf("[ImageProcessor] Failed to save small AVIF thumbnail for %s: %v", imageModel.UUID, err)
				} else {
					log.Debugf("[ImageProcessor] Saved small AVIF thumbnail for %s", imageModel.UUID)
				}
			}
		}
		smallThumb = nil
		// Medium Thumbnail
		mediumThumb = imaging.Resize(imgDecoded, MediumThumbnailSize, 0, imaging.Lanczos)
		if err := saveWebP(mediumThumb, mediumThumbWebPPath); err != nil {
			log.Errorf("[ImageProcessor] Failed to save medium WebP thumbnail for %s: %v", imageModel.UUID, err)
		} else {
			hasThumbnailMedium = true
			log.Debugf("[ImageProcessor] Saved medium WebP thumbnail for %s", imageModel.UUID)
			if IsFFmpegAvailable {
				if err := convertToAVIF(mediumThumb, mediumThumbAVIFPath); err != nil {
					log.Errorf("[ImageProcessor] Failed to save medium AVIF thumbnail for %s: %v", imageModel.UUID, err)
				} else {
					log.Debugf("[ImageProcessor] Saved medium AVIF thumbnail for %s", imageModel.UUID)
				}
			}
		}
		mediumThumb = nil
		imgDecoded = nil // Release main image memory
		log.Debugf("[ImageProcessor] Released main decoded image reference after standard processing: %s", imageModel.UUID)
	}

	// --- Database Update ---
	if err := UpdateImageRecordFunc(imageModel, width, height, hasWebp, hasAvif, hasThumbnailSmall, hasThumbnailMedium); err != nil {
		return err // Return DB update error
	}

	// If we reached here, processing was successful (or errors were logged but didn't stop processing)
	log.Infof("[ImageProcessor] Successfully processed image %s (DB updated).", imageModel.UUID)

	// Set final status to COMPLETED in cache
	if err := SetImageStatus(imageModel.UUID, STATUS_COMPLETED); err != nil {
		log.Errorf("[ImageProcessor] Failed to set final COMPLETED status in cache for %s: %v", imageModel.UUID, err)
		return fmt.Errorf("failed to set final status: %w", err)
	}

	// Check if errResult was set earlier (e.g., failed to set PROCESSING status)
	if errResult != nil {
		log.Warnf("[ImageProcessor] Process finished for %s, but encountered non-fatal error earlier: %v", imageModel.UUID, errResult)
		// Decide whether to return the earlier non-fatal error or nil
		// Returning nil because the main processing succeeded and COMPLETED status was set.
		return nil
	}

	return nil // Indicate success
}

// updateImageRecord updates the database record for the image and creates variants.
func updateImageRecord(imageModel *models.Image, width, height int, hasWebp, hasAvif, hasThumbSmall, hasThumbMedium bool) error {
	db := database.GetDB()
	if db == nil {
		log.Error("[ImageProcessor] Database connection is nil, cannot update image record.")
		return fmt.Errorf("database connection is nil")
	}

	// Update image dimensions only (remove variant flags)
	imageUpdateData := map[string]interface{}{
		"width":  width,
		"height": height,
	}

	log.Debugf("[ImageProcessor] Updating image record for %s with data: %+v", imageModel.UUID, imageUpdateData)
	if err := db.Model(&models.Image{}).Where("uuid = ?", imageModel.UUID).Updates(imageUpdateData).Error; err != nil {
		log.Errorf("[ImageProcessor] Failed to update image %s in database: %v", imageModel.UUID, err)
		return fmt.Errorf("failed to update image in database: %w", err)
	}

	// Update the imageModel struct with the new dimensions so variants have correct width/height
	imageModel.Width = width
	imageModel.Height = height

	// Create variant records based on what was successfully processed
	if err := createImageVariants(db, imageModel, hasWebp, hasAvif, hasThumbSmall, hasThumbMedium); err != nil {
		return fmt.Errorf("failed to create image variants: %w", err)
	}

	// Get or create metadata record
	var metadata models.ImageMetadata
	result := db.Where("image_id = ?", imageModel.ID).First(&metadata)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			// Create new metadata record if not found
			metadata = models.ImageMetadata{
				ImageID: imageModel.ID,
			}
		} else {
			log.Errorf("[ImageProcessor] Failed to fetch metadata for image %s: %v", imageModel.UUID, result.Error)
			return fmt.Errorf("failed to fetch metadata: %w", result.Error)
		}
	}

	// Update metadata fields if they exist in the image model
	if imageModel.Metadata != nil {
		// If the image already has metadata, use it directly
		metadata = *imageModel.Metadata

		// Make sure the ImageID is set correctly
		metadata.ImageID = imageModel.ID
	}

	// Save the metadata record
	var saveErr error
	if metadata.ID == 0 {
		// Create new record
		saveErr = db.Create(&metadata).Error
	} else {
		// Update existing record
		saveErr = db.Save(&metadata).Error
	}

	if saveErr != nil {
		log.Errorf("[ImageProcessor] Failed to save metadata for image %s: %v", imageModel.UUID, saveErr)
		return fmt.Errorf("failed to save metadata: %w", saveErr)
	}

	log.Infof("[ImageProcessor] Successfully updated database record and metadata for image %s", imageModel.UUID)
	return nil
}

// convertToAVIF converts an image (provided as image.Image) to AVIF format using ffmpeg.
func convertToAVIF(img image.Image, outputPath string) error {
	if !IsFFmpegAvailable {
		return fmt.Errorf("ffmpeg is not available for AVIF conversion")
	}
	if img == nil {
		return fmt.Errorf("input image for AVIF conversion is nil")
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory for AVIF output %s: %w", outputPath, err)
	}

	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	imgToEncode := img
	if width%2 != 0 || height%2 != 0 {
		log.Debugf("[ImageProcessor] Image %s has odd dimensions (%dx%d), adjusting for AVIF conversion to %s", filepath.Base(outputPath), width, height, outputPath)
		newWidth := width + (width % 2)
		newHeight := height + (height % 2)
		newImg := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				newImg.Set(x, y, img.At(x+bounds.Min.X, y+bounds.Min.Y))
			}
		}
		imgToEncode = newImg
		log.Debugf("[ImageProcessor] Adjusted dimensions to %dx%d for AVIF", newWidth, newHeight)
	}

	r, w := io.Pipe()
	defer r.Close()

	cmd := exec.Command("ffmpeg", "-f", "image2pipe", "-vcodec", "png", "-i", "pipe:0", "-c:v", "libsvtav1", "-crf", "35", "-preset", "8", "-pix_fmt", "yuv420p", "-movflags", "+faststart", "-y", outputPath)
	cmd.Stdin = r
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	encodeErrChan := make(chan error, 1)
	go func() {
		defer w.Close()
		log.Debugf("[ImageProcessor] Starting PNG encode to pipe for %s", outputPath)
		err := png.Encode(w, imgToEncode)
		if err == nil {
			log.Debugf("[ImageProcessor] Finished PNG encode to pipe for %s", outputPath)
		} else {
			log.Errorf("[ImageProcessor] Error during PNG encode to pipe for %s: %v", outputPath, err)
		}
		encodeErrChan <- err
	}()

	log.Debugf("[ImageProcessor] Starting ffmpeg command for %s", outputPath)
	runErr := cmd.Run()
	log.Debugf("[ImageProcessor] Finished ffmpeg command for %s", outputPath)
	encodeErr := <-encodeErrChan

	if encodeErr != nil {
		return fmt.Errorf("failed to encode image to pipe for %s: %w", outputPath, encodeErr)
	}
	if runErr != nil {
		_ = os.Remove(outputPath)
		return fmt.Errorf("ffmpeg command failed for %s: %w\nStderr: %s", outputPath, runErr, stderr.String())
	}
	log.Debugf("[ImageProcessor] Successfully created AVIF: %s", outputPath)
	return nil
}

// getImageDimensionsWithFFprobe returns the dimensions of an image using ffprobe.
func getImageDimensionsWithFFprobe(filePath string) (int, int, error) {
	if !IsFFmpegAvailable {
		return 0, 0, fmt.Errorf("ffprobe (part of ffmpeg) is not available")
	}
	log.Debugf("[ImageProcessor] Running ffprobe for dimensions: %s", filePath)
	cmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height", "-of", "csv=s=x:p=0", filePath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	output := strings.TrimSpace(stdout.String())
	stderrOutput := strings.TrimSpace(stderr.String())
	if err != nil {
		log.Errorf("[ImageProcessor] ffprobe command failed for %s. Error: %v, Stderr: %s", filePath, err, stderrOutput)
		return 0, 0, fmt.Errorf("ffprobe command failed for '%s': %w, stderr: %s", filePath, err, stderrOutput)
	}
	log.Debugf("[ImageProcessor] ffprobe output for %s: '%s'", filePath, output)
	if stderrOutput != "" {
		log.Warnf("[ImageProcessor] ffprobe stderr for %s: '%s'", filePath, stderrOutput)
	}
	parts := strings.Split(output, "x")
	if len(parts) != 2 {
		log.Errorf("[ImageProcessor] Unexpected ffprobe output format for %s: '%s'", filePath, output)
		return 0, 0, fmt.Errorf("invalid ffprobe output format: %s", output)
	}
	width, errW := strconv.Atoi(parts[0])
	height, errH := strconv.Atoi(parts[1])
	if errW != nil || errH != nil {
		log.Errorf("[ImageProcessor] Failed to parse ffprobe dimensions for %s from output '%s': W_err=%v, H_err=%v", filePath, output, errW, errH)
		return 0, 0, fmt.Errorf("failed to parse dimensions from ffprobe output '%s'", output)
	}
	log.Debugf("[ImageProcessor] Parsed dimensions from ffprobe for %s: %dx%d", filePath, width, height)
	return width, height, nil
}

// saveWebP saves an image in WebP format using the go-webp library.
func saveWebP(img image.Image, outputPath string) error {
	if img == nil {
		return fmt.Errorf("input image for WebP saving is nil")
	}
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("error creating directory '%s' for WebP: %w", outputDir, err)
	}
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("error creating WebP file '%s': %w", outputPath, err)
	}
	defer outputFile.Close()
	options, err := encoder.NewLossyEncoderOptions(encoder.PresetDefault, 85)
	if err != nil {
		log.Errorf("[ImageProcessor] Failed to create WebP encoder options: %v", err)
		return fmt.Errorf("error creating webp encoder options: %w", err)
	}
	log.Debugf("[ImageProcessor] Encoding WebP to %s", outputPath)
	if err := webp.Encode(outputFile, img, options); err != nil {
		_ = outputFile.Close()
		_ = os.Remove(outputPath)
		log.Errorf("[ImageProcessor] Failed to encode WebP image to %s: %v", outputPath, err)
		return fmt.Errorf("error encoding WebP image to '%s': %w", outputPath, err)
	}
	log.Debugf("[ImageProcessor] Successfully saved WebP: %s", outputPath)
	return nil
}

// checkFfmpegAvailable checks if the ffmpeg command is available in the system's PATH.
func checkFfmpegAvailable() bool {
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		log.Warnf("[ImageProcessor] checkFfmpegAvailable: 'ffmpeg' command not found in PATH: %v", err)
		return false
	}
	return true
}

// GetImagePath returns the path to a specific image variant based on format and size.
func GetImagePath(imageModel *models.Image, format string, size string) string {
	if imageModel == nil || imageModel.UUID == "" {
		log.Warn("[GetImagePath] Called with invalid image data (nil model or empty UUID)")
		return ""
	}

	// Get database connection
	db := database.GetDB()
	if db == nil {
		log.Error("[GetImagePath] Database connection is nil")
		return ""
	}

	// Determine variant type based on format and size
	variantType := getVariantType(format, size)
	if variantType == "" {
		log.Warnf("[GetImagePath] Cannot determine variant type for format '%s' and size '%s'", format, size)
		return ""
	}

	// Handle original separately
	if variantType == "original" {
		if imageModel.FilePath == "" || imageModel.FileName == "" {
			log.Warnf("[GetImagePath] Cannot get original path for %s: FilePath or FileName is empty", imageModel.UUID)
			return ""
		}
		return filepath.Join(imageModel.FilePath, imageModel.FileName)
	}

	// Find variant in database
	variant, err := models.FindVariantByImageIDAndType(db, imageModel.ID, variantType)
	if err != nil {
		log.Debugf("[GetImagePath] Variant '%s' not found for image %s: %v", variantType, imageModel.UUID, err)
		return ""
	}

	// Construct full path
	fullPath := filepath.Join(variant.FilePath, variant.FileName)
	log.Debugf("[GetImagePath] Found variant path for %s (Type: %s): %s", imageModel.UUID, variantType, fullPath)
	return fullPath
}

// getVariantType determines the variant type based on format and size
func getVariantType(format, size string) string {
	lowerFormat := strings.ToLower(format)
	lowerSize := strings.ToLower(size)

	// Handle original
	if lowerSize == "original" || (lowerFormat == "original" && lowerSize == "") {
		return "original"
	}

	// Handle thumbnails with specific formats
	if lowerSize == "small" {
		switch lowerFormat {
		case "webp":
			return "thumbnail_small_webp"
		case "avif":
			return "thumbnail_small_avif"
		default:
			// Default to WebP for backwards compatibility
			return "thumbnail_small_webp"
		}
	}
	if lowerSize == "medium" {
		switch lowerFormat {
		case "webp":
			return "thumbnail_medium_webp"
		case "avif":
			return "thumbnail_medium_avif"
		default:
			// Default to WebP for backwards compatibility
			return "thumbnail_medium_webp"
		}
	}

	// Handle formats (full size)
	if lowerSize == "" || lowerSize == "full" {
		switch lowerFormat {
		case "webp":
			return "webp"
		case "avif":
			return "avif"
		}
	}

	return ""
}

// createImageVariants creates variant records in the database
func createImageVariants(db *gorm.DB, imageModel *models.Image, hasWebp, hasAvif, hasThumbSmall, hasThumbMedium bool) error {
	relativePath := strings.TrimPrefix(imageModel.FilePath, OriginalDir)
	relativePath = strings.TrimPrefix(relativePath, string(filepath.Separator))
	variantsBaseDir := filepath.Join(VariantsDir, relativePath)
	baseFileName := imageModel.UUID

	// Original variant is NO LONGER created here - original data is stored in the images table

	// Create WebP variant if available
	if hasWebp {
		webpPath := filepath.Join(variantsBaseDir, baseFileName+".webp")
		if fileInfo, err := os.Stat(webpPath); err == nil {
			webpVariant := models.ImageVariant{
				ImageID:     imageModel.ID,
				VariantType: "webp",
				FilePath:    variantsBaseDir,
				FileName:    baseFileName + ".webp",
				FileType:    ".webp",
				FileSize:    fileInfo.Size(),
				Width:       imageModel.Width,
				Height:      imageModel.Height,
				Quality:     85,
			}
			if err := db.Create(&webpVariant).Error; err != nil {
				log.Errorf("[ImageProcessor] Failed to create webp variant for %s: %v", imageModel.UUID, err)
			}
		}
	}

	// Create AVIF variant if available
	if hasAvif {
		avifPath := filepath.Join(variantsBaseDir, baseFileName+".avif")
		if fileInfo, err := os.Stat(avifPath); err == nil {
			avifVariant := models.ImageVariant{
				ImageID:     imageModel.ID,
				VariantType: "avif",
				FilePath:    variantsBaseDir,
				FileName:    baseFileName + ".avif",
				FileType:    ".avif",
				FileSize:    fileInfo.Size(),
				Width:       imageModel.Width,
				Height:      imageModel.Height,
				Quality:     35,
			}
			if err := db.Create(&avifVariant).Error; err != nil {
				log.Errorf("[ImageProcessor] Failed to create avif variant for %s: %v", imageModel.UUID, err)
			}
		}
	}

	// Create small thumbnail variants
	if hasThumbSmall {
		// Create WebP small thumbnail variant
		smallWebpPath := filepath.Join(variantsBaseDir, baseFileName+"_small.webp")
		if fileInfo, err := os.Stat(smallWebpPath); err == nil {
			smallVariant := models.ImageVariant{
				ImageID:     imageModel.ID,
				VariantType: "thumbnail_small_webp",
				FilePath:    variantsBaseDir,
				FileName:    baseFileName + "_small.webp",
				FileType:    ".webp",
				FileSize:    fileInfo.Size(),
				Width:       SmallThumbnailSize,
				Height:      calculateProportionalHeight(imageModel.Width, imageModel.Height, SmallThumbnailSize),
				Quality:     85,
			}
			if err := db.Create(&smallVariant).Error; err != nil {
				log.Errorf("[ImageProcessor] Failed to create small WebP thumbnail variant for %s: %v", imageModel.UUID, err)
			}
		}

		// Create AVIF small thumbnail variant if it exists
		smallAvifPath := filepath.Join(variantsBaseDir, baseFileName+"_small.avif")
		if fileInfo, err := os.Stat(smallAvifPath); err == nil {
			smallAvifVariant := models.ImageVariant{
				ImageID:     imageModel.ID,
				VariantType: "thumbnail_small_avif",
				FilePath:    variantsBaseDir,
				FileName:    baseFileName + "_small.avif",
				FileType:    ".avif",
				FileSize:    fileInfo.Size(),
				Width:       SmallThumbnailSize,
				Height:      calculateProportionalHeight(imageModel.Width, imageModel.Height, SmallThumbnailSize),
				Quality:     35,
			}
			if err := db.Create(&smallAvifVariant).Error; err != nil {
				log.Errorf("[ImageProcessor] Failed to create small AVIF thumbnail variant for %s: %v", imageModel.UUID, err)
			}
		}
	}

	// Create medium thumbnail variants
	if hasThumbMedium {
		// Create WebP medium thumbnail variant
		mediumWebpPath := filepath.Join(variantsBaseDir, baseFileName+"_medium.webp")
		if fileInfo, err := os.Stat(mediumWebpPath); err == nil {
			mediumVariant := models.ImageVariant{
				ImageID:     imageModel.ID,
				VariantType: "thumbnail_medium_webp",
				FilePath:    variantsBaseDir,
				FileName:    baseFileName + "_medium.webp",
				FileType:    ".webp",
				FileSize:    fileInfo.Size(),
				Width:       MediumThumbnailSize,
				Height:      calculateProportionalHeight(imageModel.Width, imageModel.Height, MediumThumbnailSize),
				Quality:     85,
			}
			if err := db.Create(&mediumVariant).Error; err != nil {
				log.Errorf("[ImageProcessor] Failed to create medium WebP thumbnail variant for %s: %v", imageModel.UUID, err)
			}
		}

		// Create AVIF medium thumbnail variant if it exists
		mediumAvifPath := filepath.Join(variantsBaseDir, baseFileName+"_medium.avif")
		if fileInfo, err := os.Stat(mediumAvifPath); err == nil {
			mediumAvifVariant := models.ImageVariant{
				ImageID:     imageModel.ID,
				VariantType: "thumbnail_medium_avif",
				FilePath:    variantsBaseDir,
				FileName:    baseFileName + "_medium.avif",
				FileType:    ".avif",
				FileSize:    fileInfo.Size(),
				Width:       MediumThumbnailSize,
				Height:      calculateProportionalHeight(imageModel.Width, imageModel.Height, MediumThumbnailSize),
				Quality:     35,
			}
			if err := db.Create(&mediumAvifVariant).Error; err != nil {
				log.Errorf("[ImageProcessor] Failed to create medium AVIF thumbnail variant for %s: %v", imageModel.UUID, err)
			}
		}
	}

	log.Debugf("[ImageProcessor] Successfully created variant records for image %s", imageModel.UUID)
	return nil
}

// calculateProportionalHeight calculates the proportional height for a given width
func calculateProportionalHeight(originalWidth, originalHeight, newWidth int) int {
	if originalWidth == 0 {
		return newWidth // fallback
	}
	ratio := float64(newWidth) / float64(originalWidth)
	return int(float64(originalHeight) * ratio)
}

// DeleteImageAndVariants removes all physical files and database records for an image
func DeleteImageAndVariants(imageModel *models.Image) error {
	if imageModel == nil || imageModel.UUID == "" {
		return fmt.Errorf("invalid image data provided")
	}

	db := database.GetDB()
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}

	log.Infof("[ImageProcessor] Starting deletion of image %s and all variants", imageModel.UUID)

	// Get all variants for this image
	variants, err := models.FindVariantsByImageID(db, imageModel.ID)
	if err != nil {
		log.Errorf("[ImageProcessor] Failed to find variants for image %s: %v", imageModel.UUID, err)
		// Continue with deletion attempt even if variants can't be found
	}

	// Delete all variant files
	for _, variant := range variants {
		filePath := filepath.Join(variant.FilePath, variant.FileName)
		if err := os.Remove(filePath); err != nil {
			if !os.IsNotExist(err) {
				log.Errorf("[ImageProcessor] Failed to delete variant file %s: %v", filePath, err)
			}
		} else {
			log.Debugf("[ImageProcessor] Deleted variant file: %s", filePath)
		}
	}

	// Delete original file if it exists and is different from variants
	originalPath := filepath.Join(imageModel.FilePath, imageModel.FileName)
	if err := os.Remove(originalPath); err != nil {
		if !os.IsNotExist(err) {
			log.Errorf("[ImageProcessor] Failed to delete original file %s: %v", originalPath, err)
		}
	} else {
		log.Debugf("[ImageProcessor] Deleted original file: %s", originalPath)
	}

	// Clean up empty directories
	// Try to remove the variants directory for this image
	relativePath := strings.TrimPrefix(imageModel.FilePath, OriginalDir)
	relativePath = strings.TrimPrefix(relativePath, string(filepath.Separator))
	variantsDir := filepath.Join(VariantsDir, relativePath)
	if err := os.Remove(variantsDir); err != nil {
		if !os.IsNotExist(err) {
			log.Debugf("[ImageProcessor] Could not remove variants directory %s (may not be empty): %v", variantsDir, err)
		}
	}

	// Remove original directory if empty
	originalDir := imageModel.FilePath
	if err := os.Remove(originalDir); err != nil {
		if !os.IsNotExist(err) {
			log.Debugf("[ImageProcessor] Could not remove original directory %s (may not be empty): %v", originalDir, err)
		}
	}

	// Delete database records - variants first due to foreign key constraints
	if err := db.Where("image_id = ?", imageModel.ID).Delete(&models.ImageVariant{}).Error; err != nil {
		log.Errorf("[ImageProcessor] Failed to delete image variants from database for %s: %v", imageModel.UUID, err)
		return fmt.Errorf("failed to delete image variants from database: %w", err)
	}

	// Delete metadata
	if err := db.Where("image_id = ?", imageModel.ID).Delete(&models.ImageMetadata{}).Error; err != nil {
		log.Errorf("[ImageProcessor] Failed to delete image metadata from database for %s: %v", imageModel.UUID, err)
		// Don't return error here, continue with image deletion
	}

	// Delete the main image record
	if err := db.Delete(imageModel).Error; err != nil {
		log.Errorf("[ImageProcessor] Failed to delete image from database for %s: %v", imageModel.UUID, err)
		return fmt.Errorf("failed to delete image from database: %w", err)
	}

	log.Infof("[ImageProcessor] Successfully deleted image %s and all variants", imageModel.UUID)
	return nil
}
