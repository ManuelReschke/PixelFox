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
		// Wait for a free slot in the memory semaphore
		p.memoryThrottle <- struct{}{}

		// Increase active processes counter
		atomic.AddInt32(&p.activeProcesses, 1)

		log.Info(fmt.Sprintf("[ImageProcessor] Worker %d processing image %s (Active: %d)",
			id, job.Image.UUID, atomic.LoadInt32(&p.activeProcesses)))

		err := processImage(job.Image)

		// Free up the memory semaphore slot
		<-p.memoryThrottle

		// Decrease active processes counter
		atomic.AddInt32(&p.activeProcesses, -1)

		if err != nil {
			log.Error(fmt.Sprintf("[ImageProcessor] Worker %d failed to process image %s: %v", id, job.Image.UUID, err))
			// Set failed status
			SetImageStatus(job.Image.UUID, STATUS_FAILED)
		} else {
			log.Info(fmt.Sprintf("[ImageProcessor] Worker %d completed processing image %s", id, job.Image.UUID))
			// Set completed status
			SetImageStatus(job.Image.UUID, STATUS_COMPLETED)
		}

		// Explicitly promote garbage collection after processing large images
		// and create a short pause to give GC time to work
		job = nil    // Explicitly release job reference
		runtime.GC() // Force garbage collection
		time.Sleep(100 * time.Millisecond)
	}

	log.Info(fmt.Sprintf("[ImageProcessor] Worker %d stopped", id))
}

// EnqueueImage adds an image to the processing queue
func (p *ImageProcessor) EnqueueImage(image *models.Image) {
	if !p.started {
		p.Start()
	}

	p.jobs <- &ProcessJob{
		Image: image,
	}
	log.Info(fmt.Sprintf("[ImageProcessor] Enqueued image %s for processing", image.UUID))
}

// ProcessImage queues an image for processing
func ProcessImage(image *models.Image) error {
	SetImageStatus(image.UUID, STATUS_PENDING)
	GetProcessor().EnqueueImage(image)
	return nil
}

// processImage handles the actual image processing
func processImage(imageModel *models.Image) error {
	log.Debugf("[ImageProcessor] Processing image: %s", imageModel.UUID)
	SetImageStatus(imageModel.UUID, STATUS_PROCESSING)

	// Validation
	if imageModel == nil || imageModel.UUID == "" || imageModel.FilePath == "" {
		log.Error("[ImageProcessor] Invalid image data")
		return fmt.Errorf("invalid image data")
	}

	// Extract path components
	originalFilePath := filepath.Join(imageModel.FilePath, imageModel.FileName)

	// The rest of the file remains the same, but we'll use originalFilePath instead of imageModel.FilePath
	dirPath := filepath.Dir(originalFilePath)
	baseFileName := imageModel.UUID
	relativePath := strings.TrimPrefix(dirPath, OriginalDir)
	relativePath = strings.TrimPrefix(relativePath, string(filepath.Separator))

	// Setup paths
	variantsBaseDir := filepath.Join(VariantsDir, relativePath)

	// Ensure variants directory exists
	if err := os.MkdirAll(variantsBaseDir, 0755); err != nil {
		log.Errorf("[ImageProcessor] Failed to create variants directory for %s: %v", imageModel.UUID, err)
		return fmt.Errorf("failed to create variants directory: %w", err)
	}

	// Prepare paths for variants
	optimizedWebPPath := filepath.Join(variantsBaseDir, baseFileName+".webp")
	optimizedAVIFPath := filepath.Join(variantsBaseDir, baseFileName+".avif")
	smallThumbWebPPath := filepath.Join(variantsBaseDir, baseFileName+"_small.webp")
	smallThumbAVIFPath := filepath.Join(variantsBaseDir, baseFileName+"_small.avif")
	mediumThumbWebPPath := filepath.Join(variantsBaseDir, baseFileName+"_medium.webp")
	mediumThumbAVIFPath := filepath.Join(variantsBaseDir, baseFileName+"_medium.avif")

	// Flags to track successful operations
	var hasWebp, hasAvif, hasThumbnailSmall, hasThumbnailMedium bool
	// Image metadata
	var width, height int

	// Track generated thumbnails
	var smallThumb, mediumThumb image.Image

	// Extract metadata from the original image before any processing
	// This should happen early to ensure we capture all original image metadata
	if err := ExtractMetadata(imageModel, originalFilePath); err != nil {
		log.Warnf("[ImageProcessor] Could not extract metadata for %s: %v. Processing continues.", imageModel.UUID, err)
		// Continue processing even if metadata extraction fails
	} else {
		log.Debugf("[ImageProcessor] Successfully extracted metadata for %s", imageModel.UUID)
	}

	// Read file and detect image type
	imgFile, err := os.Open(originalFilePath)
	if err != nil {
		log.Errorf("[ImageProcessor] Failed to open image %s: %v", imageModel.UUID, err)
		return fmt.Errorf("failed to open image: %w", err)
	}
	defer imgFile.Close()

	// Read image bytes - minimizes file handle usage
	imgBytes, err := io.ReadAll(imgFile)
	if err != nil {
		log.Errorf("[ImageProcessor] Failed to read image bytes for %s: %v", imageModel.UUID, err)
		return fmt.Errorf("failed to read image bytes: %w", err)
	}

	// Try to decode the image, with auto-orientation
	imgDecoded, err := imaging.Decode(bytes.NewReader(imgBytes), imaging.AutoOrientation(true))
	if err != nil {
		// Fallback if decode fails
		log.Warnf("[ImageProcessor] Decode failed for %s, trying direct open: %v", imageModel.UUID, err)
		imgDecoded, err = imaging.Open(originalFilePath, imaging.AutoOrientation(true))
		if err != nil {
			SetImageStatus(imageModel.UUID, STATUS_FAILED)
			log.Errorf("[ImageProcessor] Failed to decode image %s: %v", imageModel.UUID, err)
			return fmt.Errorf("failed to decode image: %w", err)
		}
	}

	// Extract and store dimensions
	width = imgDecoded.Bounds().Dx()
	height = imgDecoded.Bounds().Dy()
	log.Infof("[ImageProcessor] Processing image %s (%dx%d)", imageModel.UUID, width, height)

	// Detect if it's a GIF - special handling for animated images
	isGif := strings.HasSuffix(strings.ToLower(originalFilePath), ".gif")

	// Different processing workflow for GIF vs other image types
	if isGif {
		// --- GIF Handling ---
		// For GIFs we only create thumbnails, not optimized versions
		log.Debugf("[ImageProcessor] GIF detected, creating thumbnails for %s", imageModel.UUID)

		// Small WebP Thumbnail
		smallThumb = imaging.Resize(imgDecoded, SmallThumbnailSize, 0, imaging.Lanczos)
		if err := saveWebP(smallThumb, smallThumbWebPPath); err != nil {
			log.Errorf("[ImageProcessor] Failed to save small WebP thumbnail for GIF %s: %v", imageModel.UUID, err)
			smallThumb = nil // Free memory if save failed
		} else {
			hasThumbnailSmall = true
			log.Debugf("[ImageProcessor] Saved small WebP thumbnail for GIF %s", imageModel.UUID)
		}

		// Medium WebP Thumbnail
		mediumThumb = imaging.Resize(imgDecoded, MediumThumbnailSize, 0, imaging.Lanczos)
		if err := saveWebP(mediumThumb, mediumThumbWebPPath); err != nil {
			log.Errorf("[ImageProcessor] Failed to save medium WebP thumbnail for GIF %s: %v", imageModel.UUID, err)
			mediumThumb = nil // Free memory if save failed
		} else {
			hasThumbnailMedium = true
			log.Debugf("[ImageProcessor] Saved medium WebP thumbnail for GIF %s", imageModel.UUID)
		}

		// If ffmpeg is available, create AVIF thumbnails
		if isFFmpegAvailable {
			// Small AVIF thumbnail
			if smallThumb != nil { // Check if small webp thumbnail was created
				if err := convertToAVIF(smallThumb, smallThumbAVIFPath); err != nil {
					log.Errorf("Error creating small AVIF thumbnail for GIF %s: %v", imageModel.UUID, err)
				} else {
					log.Debugf("[ImageProcessor] Small AVIF thumbnail created for GIF: %s", smallThumbAVIFPath)
					// hasThumbnailSmall already true from WebP
				}
			}

			// Medium AVIF thumbnail
			if mediumThumb != nil { // Check if medium webp thumbnail was created
				if err := convertToAVIF(mediumThumb, mediumThumbAVIFPath); err != nil {
					log.Errorf("Error creating medium AVIF thumbnail for GIF %s: %v", imageModel.UUID, err)
				} else {
					log.Debugf("[ImageProcessor] Medium AVIF thumbnail created for GIF: %s", mediumThumbAVIFPath)
					// hasThumbnailMedium already true from WebP
				}
			}
		}
	} else {
		// --- Standard Image Handling (non-GIF) ---
		// Create Optimized WebP version
		if err := saveWebP(imgDecoded, optimizedWebPPath); err != nil {
			log.Errorf("[ImageProcessor] Failed to create optimized WebP for %s: %v", imageModel.UUID, err)
			// Continue processing even if WebP fails
		} else {
			hasWebp = true // Set flag on success
			log.Debugf("[ImageProcessor] Saved optimized WebP for %s", imageModel.UUID)
		}

		// Create Optimized AVIF version
		if isFFmpegAvailable {
			if err := convertToAVIF(imgDecoded, optimizedAVIFPath); err != nil {
				log.Errorf("[ImageProcessor] Failed to convert to AVIF for %s: %v", imageModel.UUID, err)
			} else {
				hasAvif = true // Set flag on success
				log.Debug(fmt.Sprintf("[ImageProcessor] Converted to AVIF for %s", imageModel.UUID))
			}
		} else {
			log.Warnf("[ImageProcessor] Skipping AVIF conversion for %s: ffmpeg not found.", imageModel.UUID)
		}

		// --- Generate Thumbnails ---

		// Small Thumbnail
		smallThumb = imaging.Resize(imgDecoded, SmallThumbnailSize, 0, imaging.Lanczos)
		if err := saveWebP(smallThumb, smallThumbWebPPath); err != nil {
			log.Errorf("[ImageProcessor] Failed to save small WebP thumbnail for %s: %v", imageModel.UUID, err)
			// Continue processing, but flag will remain false
		} else {
			hasThumbnailSmall = true // Set flag on success
			log.Debugf("[ImageProcessor] Saved small WebP thumbnail for %s", imageModel.UUID)
			// --- Generate Small AVIF Thumbnail ---
			if isFFmpegAvailable && smallThumb != nil {
				if err := convertToAVIF(smallThumb, smallThumbAVIFPath); err != nil {
					log.Errorf("[ImageProcessor] Failed to save small AVIF thumbnail for %s: %v", imageModel.UUID, err)
				} else {
					log.Debugf("[ImageProcessor] Saved small AVIF thumbnail for %s", imageModel.UUID)
				}
			}
		}

		// Medium Thumbnail
		mediumThumb = imaging.Resize(imgDecoded, MediumThumbnailSize, 0, imaging.Lanczos)
		if err := saveWebP(mediumThumb, mediumThumbWebPPath); err != nil {
			log.Errorf("[ImageProcessor] Failed to save medium WebP thumbnail for %s: %v", imageModel.UUID, err)
			// Continue processing, but flag will remain false
		} else {
			hasThumbnailMedium = true // Set flag on success
			log.Debugf("[ImageProcessor] Saved medium WebP thumbnail for %s", imageModel.UUID)
			// --- Generate Medium AVIF Thumbnail ---
			if isFFmpegAvailable && mediumThumb != nil {
				if err := convertToAVIF(mediumThumb, mediumThumbAVIFPath); err != nil {
					log.Errorf("[ImageProcessor] Failed to save medium AVIF thumbnail for %s: %v", imageModel.UUID, err)
				} else {
					log.Debugf("[ImageProcessor] Saved medium AVIF thumbnail for %s", imageModel.UUID)
				}
			}
		}

		// Release image data from memory as soon as possible
		imgDecoded = nil
		smallThumb = nil
		mediumThumb = nil
	}

	// Update database
	db := database.GetDB()
	// Use a map for updates to handle potential zero values correctly (like lat/lon)
	updateData := map[string]interface{}{
		"has_webp":             hasWebp,            // Use flag variable
		"has_avif":             hasAvif,            // Use flag variable
		"has_thumbnail_small":  hasThumbnailSmall,  // Use flag variable
		"has_thumbnail_medium": hasThumbnailMedium, // Use flag variable
		"width":                width,
		"height":               height,
		// Include metadata fields
		"camera_model":  imageModel.CameraModel,
		"exposure_time": imageModel.ExposureTime,
		"aperture":      imageModel.Aperture,
		"focal_length":  imageModel.FocalLength,
		"metadata":      imageModel.Metadata,
	}

	// ISO is a pointer, so only include if not nil
	if imageModel.ISO != nil {
		updateData["iso"] = *imageModel.ISO
	}

	// Latitude & Longitude are pointers, so only include if not nil
	if imageModel.Latitude != nil {
		updateData["latitude"] = *imageModel.Latitude
	}

	if imageModel.Longitude != nil {
		updateData["longitude"] = *imageModel.Longitude
	}

	// TakenAt is a pointer, only include if not nil and not zero value
	if imageModel.TakenAt != nil && !imageModel.TakenAt.IsZero() {
		updateData["taken_at"] = imageModel.TakenAt
	}

	if err := db.Model(imageModel).Updates(updateData).Error; err != nil {
		log.Errorf("[ImageProcessor] Failed to update image %s in database: %v", imageModel.UUID, err)
		return fmt.Errorf("failed to update image in database: %w", err)
	}

	return nil
}

// convertToAVIF converts an image (provided as image.Image) to AVIF format using ffmpeg, reading from stdin pipe.
func convertToAVIF(img image.Image, outputPath string) error {
	if !isFFmpegAvailable { // Use the global variable check
		return fmt.Errorf("ffmpeg is not available")
	}
	if img == nil {
		return fmt.Errorf("input image is nil")
	}

	// Create parent directories if they don't exist
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory for AVIF output %s: %w", outputPath, err)
	}

	// Get image dimensions
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	// Check if dimensions are even (required for YUV420 color space in AVIF)
	hasOddDimensions := width%2 != 0 || height%2 != 0

	// If dimensions are odd, create a new image with even dimensions by adding padding
	if hasOddDimensions {
		log.Debugf("[ImageProcessor] Image has odd dimensions (%dx%d), adjusting for AVIF conversion", width, height)

		// Calculate new dimensions (make them even)
		newWidth := width
		newHeight := height

		if width%2 != 0 {
			newWidth++
		}

		if height%2 != 0 {
			newHeight++
		}

		// Create a new image with even dimensions
		newImg := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

		// Copy original image data
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				newImg.Set(x, y, img.At(x+bounds.Min.X, y+bounds.Min.Y))
			}
		}

		// Use the adjusted image
		img = newImg
	}

	// Create a pipe to stream PNG data to ffmpeg
	r, w := io.Pipe()

	// Improved AVIF encoding parameters for better compression/quality balance
	cmd := exec.Command("ffmpeg",
		"-f", "image2pipe", // Input format from pipe
		"-vcodec", "png", // Specify the codec of the piped data
		"-i", "pipe:0", // Read from stdin (pipe)
		"-c:v", "libsvtav1", // Use the SVT-AV1 encoder (typically more efficient)
		"-crf", "35", // Higher CRF = smaller file size, lower quality (30-40 is good for thumbnails)
		"-preset", "8", // Higher = faster encoding but lower compression (0-12)
		"-pix_fmt", "yuv420p", // Standard pixel format for web compatibility
		"-y", // Overwrite output file without asking
		outputPath,
	)

	// Set the command's standard input to the reader end of the pipe
	cmd.Stdin = r

	// Capture stderr for error messages
	var stderr strings.Builder
	cmd.Stderr = &stderr

	// Start a goroutine to encode the image as PNG and write it to the pipe
	encodeErrChan := make(chan error, 1) // Buffered channel to avoid blocking
	go func() {
		defer w.Close() // IMPORTANT: Close the writer when done to signal EOF to ffmpeg
		err := png.Encode(w, img)
		encodeErrChan <- err
	}()

	// Run the ffmpeg command
	runErr := cmd.Run()

	// Wait for the encoding goroutine to finish and check for errors
	encodeErr := <-encodeErrChan

	// Help GC by explicitly releasing the image reference
	img = nil

	if encodeErr != nil {
		return fmt.Errorf("failed to encode image to pipe: %w", encodeErr)
	}

	if runErr != nil {
		// Try to remove potentially corrupted output file on failure
		_ = os.Remove(outputPath)
		return fmt.Errorf("ffmpeg command failed: %w\nStderr: %s", runErr, stderr.String())
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

// checkFfmpegAvailable checks if ffmpeg is available
func checkFfmpegAvailable() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

// GetImagePath returns the path to a specific image version based on the new structure
func GetImagePath(imageModel *models.Image, format string, size string) string {
	if imageModel == nil || imageModel.FilePath == "" || imageModel.UUID == "" {
		log.Warn("GetImagePath called with invalid image data")
		return "" // Return empty if image or essential data is invalid
	}

	// Debug original path to identify the issue
	log.Debugf("[GetImagePath] Original path: %s", imageModel.FilePath)
	
	// Original path is stored in image.FilePath (e.g., uploads/original/2025/04/14)
	// Extract relative path part (e.g., 2025/04/14) directly
	
	// PROBLEM: filepath.Dir interpretiert den letzten Teil (Tag) als Dateinamen
	// und entfernt ihn, wenn es kein abschließender Schrägstrich vorhanden ist
	// LÖSUNG: Wir verwenden direkt den Teil nach OriginalDir ohne filepath.Dir
	
	// Ensure path doesn't have a trailing slash
	originalPath := strings.TrimSuffix(imageModel.FilePath, string(filepath.Separator))
	
	// Extract relative path after the OriginalDir part
	relativePath := strings.TrimPrefix(originalPath, OriginalDir)
	// Remove any leading path separator
	relativePath = strings.TrimPrefix(relativePath, string(filepath.Separator))
	
	log.Debugf("[GetImagePath] Extracted relative path: %s", relativePath)

	// Base filename without extension is the UUID
	baseFileName := imageModel.UUID

	// Construct variant filename part
	variantPart := ""
	switch strings.ToLower(size) {
	case "small":
		variantPart = "_small"
	case "medium":
		variantPart = "_medium"
	case "":
		// No size suffix for optimized full size
	default:
		log.Warnf("GetImagePath called with unknown size: '%s' for image %s", size, imageModel.UUID)
		// Fallback to original for unknown size?
		// Let's return original path as a safer fallback for unknown sizes.
		return imageModel.FilePath // Fallback to original
	}

	// Determine file extension based on format and availability
	ext := ""
	switch strings.ToLower(format) {
	case "webp":
		// Always return path for WebP thumbnails for GIFs
		// For non-GIFs, check if WebP version exists.
		if (imageModel.FileType == ".gif" && size != "") || imageModel.HasWebp {
			ext = ".webp"
		}
	case "avif":
		// Only return path if AVIF version exists (HasAvif flag)
		if imageModel.HasAVIF {
			ext = ".avif"
		}
	case "original":
		// Special case to get the original path
		return imageModel.FilePath
	default:
		log.Warnf("GetImagePath called with unknown format: '%s' for image %s", format, imageModel.UUID)
		// Fallback to original for unknown format?
		return imageModel.FilePath // Fallback to original
	}

	// If no valid extension determined (e.g., AVIF requested but not available),
	// fallback to the original image path.
	if ext == "" {
		log.Debugf("GetImagePath: Format '%s' (size '%s') not available for image %s, falling back to original.", format, size, imageModel.UUID)
		return imageModel.FilePath
	}

	// Construct the full path within the VariantsDir
	variantFileName := baseFileName + variantPart + ext
	variantPath := filepath.Join(VariantsDir, relativePath, variantFileName)
	
	log.Debugf("[GetImagePath] Final variant path: %s", variantPath)

	return variantPath
}
