package repository

import (
	"github.com/ManuelReschke/PixelFox/app/models"
	"gorm.io/gorm"
)

// pageRepository implements the PageRepository interface
type pageRepository struct {
	db *gorm.DB
}

// NewPageRepository creates a new page repository instance
func NewPageRepository(db *gorm.DB) PageRepository {
	return &pageRepository{db: db}
}

// Create creates a new page in the database
func (r *pageRepository) Create(page *models.Page) error {
	return r.db.Create(page).Error
}

// GetByID retrieves a page by its ID
func (r *pageRepository) GetByID(id uint) (*models.Page, error) {
	var page models.Page
	err := r.db.First(&page, id).Error
	if err != nil {
		return nil, err
	}
	return &page, nil
}

// GetBySlug retrieves a page by its slug
func (r *pageRepository) GetBySlug(slug string) (*models.Page, error) {
	var page models.Page
	err := r.db.Where("slug = ? AND is_active = ?", slug, true).First(&page).Error
	if err != nil {
		return nil, err
	}
	return &page, nil
}

// GetAll retrieves all pages
func (r *pageRepository) GetAll() ([]models.Page, error) {
	var pages []models.Page
	err := r.db.Order("created_at DESC").Find(&pages).Error
	return pages, err
}

// GetActive retrieves all active pages
func (r *pageRepository) GetActive() ([]models.Page, error) {
	var pages []models.Page
	err := r.db.Where("is_active = ?", true).Order("created_at DESC").Find(&pages).Error
	return pages, err
}

// Update updates an existing page in the database
func (r *pageRepository) Update(page *models.Page) error {
	return r.db.Save(page).Error
}

// Delete soft deletes a page by its ID
func (r *pageRepository) Delete(id uint) error {
	return r.db.Delete(&models.Page{}, id).Error
}

// SlugExists checks if a slug already exists
func (r *pageRepository) SlugExists(slug string) (bool, error) {
	var count int64
	err := r.db.Model(&models.Page{}).Where("slug = ?", slug).Count(&count).Error
	return count > 0, err
}

// SlugExistsExceptID checks if a slug exists excluding a specific ID
func (r *pageRepository) SlugExistsExceptID(slug string, id uint) (bool, error) {
	var count int64
	err := r.db.Model(&models.Page{}).Where("slug = ? AND id != ?", slug, id).Count(&count).Error
	return count > 0, err
}
