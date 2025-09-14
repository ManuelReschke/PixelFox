package repository

import (
	"fmt"
	"strings"
	"time"

	"github.com/ManuelReschke/PixelFox/app/models"
	"gorm.io/gorm"
)

// imageRepository implements the ImageRepository interface
type imageRepository struct {
	db *gorm.DB
}

// NewImageRepository creates a new image repository instance
func NewImageRepository(db *gorm.DB) ImageRepository {
	return &imageRepository{db: db}
}

// Create creates a new image in the database
func (r *imageRepository) Create(image *models.Image) error {
	return r.db.Create(image).Error
}

// GetByID retrieves an image by its ID
func (r *imageRepository) GetByID(id uint) (*models.Image, error) {
	var image models.Image
	err := r.db.Preload("User").Preload("Metadata").Preload("StoragePool").
		Preload("Tags").Preload("Comments").First(&image, id).Error
	if err != nil {
		return nil, err
	}
	return &image, nil
}

// GetByUUID retrieves an image by its UUID
func (r *imageRepository) GetByUUID(uuid string) (*models.Image, error) {
	var image models.Image
	err := r.db.Preload("User").Preload("Metadata").Preload("StoragePool").
		Where("uuid = ?", uuid).First(&image).Error
	if err != nil {
		return nil, err
	}
	return &image, nil
}

// GetByFilename retrieves an image by its filename
func (r *imageRepository) GetByFilename(filename string) (*models.Image, error) {
	var image models.Image
	err := r.db.Where("file_name = ?", filename).First(&image).Error
	if err != nil {
		return nil, err
	}
	return &image, nil
}

// GetByShareLink retrieves an image by its share link
func (r *imageRepository) GetByShareLink(shareLink string) (*models.Image, error) {
	var image models.Image
	err := r.db.Preload("User").Preload("Metadata").Preload("StoragePool").
		Where("share_link = ?", shareLink).First(&image).Error
	if err != nil {
		return nil, err
	}
	return &image, nil
}

// GetByUserID retrieves images belonging to a specific user with pagination
func (r *imageRepository) GetByUserID(userID uint, offset, limit int) ([]models.Image, error) {
	var images []models.Image
	err := r.db.Preload("StoragePool").Where("user_id = ?", userID).
		Order("created_at DESC").Offset(offset).Limit(limit).Find(&images).Error
	return images, err
}

// Update updates an existing image in the database
func (r *imageRepository) Update(image *models.Image) error {
	return r.db.Save(image).Error
}

// Delete soft deletes an image by its ID
func (r *imageRepository) Delete(id uint) error {
	// Soft-delete image and related records in a transaction
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Soft delete variants
		if err := tx.Where("image_id = ?", id).Delete(&models.ImageVariant{}).Error; err != nil {
			return err
		}
		// Soft delete metadata
		if err := tx.Where("image_id = ?", id).Delete(&models.ImageMetadata{}).Error; err != nil {
			return err
		}
		// Soft delete the image itself
		if err := tx.Delete(&models.Image{}, id).Error; err != nil {
			return err
		}
		return nil
	})
}

// List retrieves a paginated list of images
func (r *imageRepository) List(offset, limit int) ([]models.Image, error) {
	var images []models.Image
	err := r.db.Preload("User").Preload("StoragePool").
		Order("created_at DESC").Offset(offset).Limit(limit).Find(&images).Error
	return images, err
}

// Count returns the total number of images
func (r *imageRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&models.Image{}).Count(&count).Error
	return count, err
}

// CountByUserID returns the number of images for a specific user
func (r *imageRepository) CountByUserID(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.Image{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

// Search searches for images by title, description, or UUID
func (r *imageRepository) Search(query string) ([]models.Image, error) {
	var images []models.Image
	searchPattern := "%" + strings.TrimSpace(query) + "%"
	err := r.db.Preload("User").Preload("StoragePool").
		Where("title LIKE ? OR description LIKE ? OR uuid LIKE ?",
			searchPattern, searchPattern, searchPattern).Find(&images).Error
	return images, err
}

// GetPublicImages retrieves public images with pagination
func (r *imageRepository) GetPublicImages(offset, limit int) ([]models.Image, error) {
	var images []models.Image
	err := r.db.Preload("User").Preload("StoragePool").
		Where("is_public = ?", true).
		Order("created_at DESC").Offset(offset).Limit(limit).Find(&images).Error
	return images, err
}

// GetRecentImages retrieves the most recently uploaded images
func (r *imageRepository) GetRecentImages(limit int) ([]models.Image, error) {
	var images []models.Image
	err := r.db.Preload("User").Preload("StoragePool").
		Order("created_at DESC").Limit(limit).Find(&images).Error
	return images, err
}

// UpdateViewCount increments the view count for an image
func (r *imageRepository) UpdateViewCount(id uint) error {
	return r.db.Model(&models.Image{}).Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + ?", 1)).Error
}

// UpdateDownloadCount increments the download count for an image
func (r *imageRepository) UpdateDownloadCount(id uint) error {
	return r.db.Model(&models.Image{}).Where("id = ?", id).
		UpdateColumn("download_count", gorm.Expr("download_count + ?", 1)).Error
}

// GetVariants retrieves all variants for a specific image
func (r *imageRepository) GetVariants(imageID uint) ([]models.ImageVariant, error) {
	var variants []models.ImageVariant
	err := r.db.Where("image_id = ?", imageID).Find(&variants).Error
	return variants, err
}

// DeleteVariants deletes all variants for a specific image
func (r *imageRepository) DeleteVariants(imageID uint) error {
	return r.db.Where("image_id = ?", imageID).Delete(&models.ImageVariant{}).Error
}

// GetDailyStats returns daily image upload statistics for a date range
func (r *imageRepository) GetDailyStats(startDate, endDate time.Time) ([]models.DailyStats, error) {
	var results []struct {
		Date  string `json:"date"`
		Count int64  `json:"count"`
	}

	// Query to get daily image upload counts
	// Use DATE_FORMAT for MySQL compatibility and proper date formatting
	err := r.db.Model(&models.Image{}).
		Select("DATE_FORMAT(created_at, '%Y-%m-%d') as date, COUNT(*) as count").
		Where("created_at BETWEEN ? AND ?", startDate, endDate).
		Group("DATE_FORMAT(created_at, '%Y-%m-%d')").
		Order("date").
		Find(&results).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get daily image stats: %w", err)
	}

	// Convert to DailyStats slice
	dailyStats := make([]models.DailyStats, len(results))
	for i, result := range results {
		dailyStats[i] = models.DailyStats{
			Date:  result.Date,
			Count: int(result.Count),
		}
	}

	return dailyStats, nil
}

// GetByUserIDAndFileHash retrieves an image by user ID and file hash for duplicate detection
func (r *imageRepository) GetByUserIDAndFileHash(userID uint, fileHash string) (*models.Image, error) {
	var image models.Image
	err := r.db.Where("user_id = ? AND file_hash = ?", userID, fileHash).First(&image).Error
	if err != nil {
		return nil, err
	}
	return &image, nil
}
