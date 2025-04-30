package imageprocessor

import (
	"bytes"
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
)

// Thumbnail sizes
const (
	SmallThumbnailSize  = 200
	MediumThumbnailSize = 500
)

// Directory paths
const (
	OriginalDir = "uploads/original"
	VariantsDir = "uploads/variants"
	MaxWorkers  = 3
)

// Global variables
var isFFmpegAvailable bool

// init initializes global variables and settings
func init() {
	// Check if ffmpeg is available once at startup
	isFFmpegAvailable = checkFfmpegAvailable()
	if !isFFmpegAvailable {
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
		if !isFFmpegAvailable {
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
		if err := updateImageRecord(imageModel, width, height, hasWebp, hasAvif, hasThumbnailSmall, hasThumbnailMedium); err != nil {
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
			if isFFmpegAvailable {
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
			if isFFmpegAvailable {
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
		if isFFmpegAvailable {
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
			if isFFmpegAvailable {
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
			if isFFmpegAvailable {
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
	if err := updateImageRecord(imageModel, width, height, hasWebp, hasAvif, hasThumbnailSmall, hasThumbnailMedium); err != nil {
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

// updateImageRecord updates the database record for the image.
func updateImageRecord(imageModel *models.Image, width, height int, hasWebp, hasAvif, hasThumbSmall, hasThumbMedium bool) error {
	db := database.GetDB()
	if db == nil {
		log.Error("[ImageProcessor] Database connection is nil, cannot update image record.")
		return fmt.Errorf("database connection is nil")
	}
	updateData := map[string]interface{}{
		"has_webp":             hasWebp,
		"has_avif":             hasAvif,
		"has_thumbnail_small":  hasThumbSmall,
		"has_thumbnail_medium": hasThumbMedium,
		"width":                width,
		"height":               height,
		"camera_model":         imageModel.CameraModel,
		"exposure_time":        imageModel.ExposureTime,
		"aperture":             imageModel.Aperture,
		"focal_length":         imageModel.FocalLength,
		"metadata":             imageModel.Metadata,
	}
	if imageModel.ISO != nil {
		updateData["iso"] = *imageModel.ISO
	}
	if imageModel.Latitude != nil {
		updateData["latitude"] = *imageModel.Latitude
	}
	if imageModel.Longitude != nil {
		updateData["longitude"] = *imageModel.Longitude
	}
	if imageModel.TakenAt != nil && !imageModel.TakenAt.IsZero() {
		updateData["taken_at"] = *imageModel.TakenAt
	}

	log.Debugf("[ImageProcessor] Updating database record for %s with data: %+v", imageModel.UUID, updateData)
	if err := db.Model(&models.Image{}).Where("uuid = ?", imageModel.UUID).Updates(updateData).Error; err != nil {
		log.Errorf("[ImageProcessor] Failed to update image %s in database: %v", imageModel.UUID, err)
		return fmt.Errorf("failed to update image in database: %w", err)
	}
	log.Infof("[ImageProcessor] Successfully updated database record for image %s", imageModel.UUID)
	return nil
}

// convertToAVIF converts an image (provided as image.Image) to AVIF format using ffmpeg.
func convertToAVIF(img image.Image, outputPath string) error {
	if !isFFmpegAvailable {
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
	if !isFFmpegAvailable {
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
	if imageModel == nil || imageModel.UUID == "" || imageModel.FilePath == "" {
		log.Warn("[GetImagePath] Called with invalid image data (nil model, empty UUID or FilePath)")
		return ""
	}
	originalPathDir := imageModel.FilePath
	relativePath := strings.TrimPrefix(originalPathDir, OriginalDir)
	relativePath = strings.TrimPrefix(relativePath, string(filepath.Separator))
	baseFileName := imageModel.UUID
	variantSuffix := ""
	lowerSize := strings.ToLower(size)
	switch lowerSize {
	case "small":
		variantSuffix = "_small"
	case "medium":
		variantSuffix = "_medium"
	case "":
		variantSuffix = ""
	case "original":
		if imageModel.FileName == "" {
			log.Warnf("[GetImagePath] Cannot get original path for %s: FileName is empty", imageModel.UUID)
			return ""
		}
		return filepath.Join(imageModel.FilePath, imageModel.FileName)
	default:
		log.Warnf("[GetImagePath] Unknown size requested: '%s' for image %s. Returning empty path.", size, imageModel.UUID)
		return ""
	}

	var finalExt string
	requestedFormat := strings.ToLower(format)
	isThumbnail := lowerSize == "small" || lowerSize == "medium"
	fileExt := strings.ToLower(strings.TrimPrefix(imageModel.FileType, "."))
	isGifInput := fileExt == "gif"

	switch requestedFormat {
	case "webp":
		if isThumbnail && imageModel.HasThumbnailSmall {
			finalExt = ".webp"
		} else if !isThumbnail && imageModel.HasWebp {
			finalExt = ".webp"
		} else if isGifInput && isThumbnail {
			if lowerSize == "small" && imageModel.HasThumbnailSmall {
				finalExt = ".webp"
			} else if lowerSize == "medium" && imageModel.HasThumbnailMedium {
				finalExt = ".webp"
			}
		}
	case "avif":
		// AVIF Thumbnails depend on WebP thumbnails existing AND ffmpeg being available
		canHaveAvifThumb := (lowerSize == "small" && imageModel.HasThumbnailSmall) || (lowerSize == "medium" && imageModel.HasThumbnailMedium)
		if isThumbnail && canHaveAvifThumb && isFFmpegAvailable {
			finalExt = ".avif"
		} else if !isThumbnail && imageModel.HasAVIF {
			finalExt = ".avif"
		}
	default:
		log.Warnf("[GetImagePath] Unknown format requested: '%s' for image %s. Returning empty path.", format, imageModel.UUID)
		return ""
	}

	if finalExt == "" {
		log.Debugf("[GetImagePath] Format '%s' (size '%s') not available or flags indicate missing for image %s. Returning empty path.", format, size, imageModel.UUID)
		return ""
	}

	variantFileName := baseFileName + variantSuffix + finalExt
	variantPath := filepath.Join(VariantsDir, relativePath, variantFileName)
	log.Debugf("[GetImagePath] Constructed variant path for %s (Format: %s, Size: %s): %s", imageModel.UUID, format, size, variantPath)
	return variantPath
}
