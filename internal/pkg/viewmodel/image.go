package viewmodel

// Image contains all information needed for displaying an image in the ImageViewer
type Image struct {
	// Website domain (e.g. https://pixelfox.cc)
	Domain string

	// Preview path (thumbnail or original)
	PreviewPath string

	// Complete path to original for download
	FilePathWithDomain string

	// Display name of the file
	DisplayName string

	// ShareLink URL
	ShareURL string

	// Available image formats (determined dynamically from variants)
	HasWebP bool
	HasAVIF bool

	// Flag indicating if any optimized versions (Medium, Small, WebP, AVIF) are available
	HasOptimizedVersions bool

	// Paths for optimized preview formats (medium thumbnails)
	PreviewWebPPath     string
	PreviewAVIFPath     string
	PreviewOriginalPath string

	// Paths for small thumbnails (WebP, AVIF und Original)
	SmallWebPPath     string
	SmallAVIFPath     string
	SmallOriginalPath string

	// Paths for optimized full-size versions
	OptimizedWebPPath string
	OptimizedAVIFPath string

	// Original path (for download)
	OriginalPath string

	// Additional metadata
	Width  int
	Height int

	// Image UUID for status tracking
	UUID string

	// Processing status flag
	IsProcessing bool

	// Metadata fields
	CameraModel  string
	TakenAt      string
	Latitude     string
	Longitude    string
	ExposureTime string
	Aperture     string
	ISO          string
	FocalLength  string

	// Human-readable sizes for each format and size category
	OptimizedOriginalSize string
	OptimizedWebPSize     string
	OptimizedAVIFSize     string

	MediumOriginalSize string
	MediumWebPSize     string
	MediumAVIFSize     string

	SmallOriginalSize string
	SmallWebPSize     string
	SmallAVIFSize     string

	// Raw byte sizes for calculations
	OptimizedOriginalBytes int64
	OptimizedWebPBytes     int64
	OptimizedAVIFBytes     int64

	MediumOriginalBytes int64
	MediumWebPBytes     int64
	MediumAVIFBytes     int64

	SmallOriginalBytes int64
	SmallWebPBytes     int64
	SmallAVIFBytes     int64
}
