package repository

import (
	"github.com/ManuelReschke/PixelFox/app/models"
	"gorm.io/gorm"
)

// newsRepository implements the NewsRepository interface
type newsRepository struct {
	db *gorm.DB
}

// NewNewsRepository creates a new news repository instance
func NewNewsRepository(db *gorm.DB) NewsRepository {
	return &newsRepository{db: db}
}

// Create creates a new news article in the database
func (r *newsRepository) Create(news *models.News) error {
	return r.db.Create(news).Error
}

// GetByID retrieves a news article by its ID
func (r *newsRepository) GetByID(id uint) (*models.News, error) {
	var news models.News
	err := r.db.Preload("User").First(&news, id).Error
	if err != nil {
		return nil, err
	}
	return &news, nil
}

// GetBySlug retrieves a news article by its slug
func (r *newsRepository) GetBySlug(slug string) (*models.News, error) {
	var news models.News
	err := r.db.Preload("User").Where("slug = ?", slug).First(&news).Error
	if err != nil {
		return nil, err
	}
	return &news, nil
}

// GetPublished retrieves published news articles with pagination
func (r *newsRepository) GetPublished(offset, limit int) ([]models.News, error) {
	var news []models.News
	err := r.db.Preload("User").Where("published = ?", true).
		Order("created_at DESC").Offset(offset).Limit(limit).Find(&news).Error
	return news, err
}

// GetAll retrieves all news articles with pagination
func (r *newsRepository) GetAll(offset, limit int) ([]models.News, error) {
	var news []models.News
	err := r.db.Preload("User").Order("created_at DESC").
		Offset(offset).Limit(limit).Find(&news).Error
	return news, err
}

// Update updates an existing news article in the database
func (r *newsRepository) Update(news *models.News) error {
	return r.db.Save(news).Error
}

// Delete soft deletes a news article by its ID
func (r *newsRepository) Delete(id uint) error {
	return r.db.Delete(&models.News{}, id).Error
}

// Count returns the total number of news articles
func (r *newsRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&models.News{}).Count(&count).Error
	return count, err
}

// GetAllWithoutPagination retrieves all news articles without pagination
func (r *newsRepository) GetAllWithoutPagination() ([]models.News, error) {
	var news []models.News
	err := r.db.Preload("User").Order("created_at DESC").Find(&news).Error
	return news, err
}

// SlugExists checks if a slug already exists
func (r *newsRepository) SlugExists(slug string) (bool, error) {
	var count int64
	err := r.db.Model(&models.News{}).Where("slug = ?", slug).Count(&count).Error
	return count > 0, err
}

// SlugExistsExceptID checks if a slug exists excluding a specific ID
func (r *newsRepository) SlugExistsExceptID(slug string, id uint) (bool, error) {
	var count int64
	err := r.db.Model(&models.News{}).Where("slug = ? AND id != ?", slug, id).Count(&count).Error
	return count > 0, err
}
