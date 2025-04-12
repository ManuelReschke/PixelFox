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

	// Available image formats
	HasWebP bool
	HasAVIF bool

	// Paths for optimized preview formats (thumbnails)
	PreviewWebPPath string
	PreviewAVIFPath string

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
}
