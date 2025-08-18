package repository

import (
	"github.com/ManuelReschke/PixelFox/app/models"
	"gorm.io/gorm"
)

// albumRepository implements the AlbumRepository interface
type albumRepository struct {
	db *gorm.DB
}

// NewAlbumRepository creates a new album repository instance
func NewAlbumRepository(db *gorm.DB) AlbumRepository {
	return &albumRepository{db: db}
}

// Create creates a new album in the database
func (r *albumRepository) Create(album *models.Album) error {
	return r.db.Create(album).Error
}

// GetByID retrieves an album by its ID
func (r *albumRepository) GetByID(id uint) (*models.Album, error) {
	var album models.Album
	err := r.db.Preload("User").Preload("Images").First(&album, id).Error
	if err != nil {
		return nil, err
	}
	return &album, nil
}

// GetByUserID retrieves all albums belonging to a specific user
func (r *albumRepository) GetByUserID(userID uint) ([]models.Album, error) {
	var albums []models.Album
	err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").Find(&albums).Error
	return albums, err
}

// Update updates an existing album in the database
func (r *albumRepository) Update(album *models.Album) error {
	return r.db.Save(album).Error
}

// Delete soft deletes an album by its ID
func (r *albumRepository) Delete(id uint) error {
	// First remove all album-image associations
	err := r.db.Exec("DELETE FROM album_images WHERE album_id = ?", id).Error
	if err != nil {
		return err
	}

	// Then delete the album
	return r.db.Delete(&models.Album{}, id).Error
}

// AddImage adds an image to an album
func (r *albumRepository) AddImage(albumID, imageID uint) error {
	// Check if the association already exists
	var count int64
	err := r.db.Table("album_images").
		Where("album_id = ? AND image_id = ?", albumID, imageID).
		Count(&count).Error
	if err != nil {
		return err
	}

	// If association doesn't exist, create it
	if count == 0 {
		return r.db.Exec("INSERT INTO album_images (album_id, image_id) VALUES (?, ?)",
			albumID, imageID).Error
	}

	return nil
}

// RemoveImage removes an image from an album
func (r *albumRepository) RemoveImage(albumID, imageID uint) error {
	return r.db.Exec("DELETE FROM album_images WHERE album_id = ? AND image_id = ?",
		albumID, imageID).Error
}

// GetImages retrieves all images in an album
func (r *albumRepository) GetImages(albumID uint) ([]models.Image, error) {
	var images []models.Image
	err := r.db.Table("images").
		Joins("JOIN album_images ON images.id = album_images.image_id").
		Where("album_images.album_id = ?", albumID).
		Preload("StoragePool").
		Order("images.created_at DESC").
		Find(&images).Error
	return images, err
}

// Count returns the total number of albums
func (r *albumRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&models.Album{}).Count(&count).Error
	return count, err
}

// CountByUserID returns the number of albums for a specific user
func (r *albumRepository) CountByUserID(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.Album{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}
