package repository

import (
	"fmt"
	"strings"

	"github.com/ManuelReschke/PixelFox/app/models"
	"gorm.io/gorm"
)

// userRepository implements the UserRepository interface
type userRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new user repository instance
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// Create creates a new user in the database
func (r *userRepository) Create(user *models.User) error {
	return r.db.Create(user).Error
}

// GetByID retrieves a user by their ID
func (r *userRepository) GetByID(id uint) (*models.User, error) {
	var user models.User
	err := r.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByEmail retrieves a user by their email address
func (r *userRepository) GetByEmail(email string) (*models.User, error) {
	var user models.User
	err := r.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByActivationToken retrieves a user by their activation token
func (r *userRepository) GetByActivationToken(token string) (*models.User, error) {
	var user models.User
	err := r.db.Where("activation_token = ?", token).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// Update updates an existing user in the database
func (r *userRepository) Update(user *models.User) error {
	return r.db.Save(user).Error
}

// Delete soft deletes a user by their ID
func (r *userRepository) Delete(id uint) error {
	return r.db.Delete(&models.User{}, id).Error
}

// List retrieves a paginated list of users
func (r *userRepository) List(offset, limit int) ([]models.User, error) {
	var users []models.User
	err := r.db.Order("created_at DESC").Offset(offset).Limit(limit).Find(&users).Error
	return users, err
}

// Count returns the total number of users
func (r *userRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&models.User{}).Count(&count).Error
	return count, err
}

// Search searches for users by name or email
func (r *userRepository) Search(query string) ([]models.User, error) {
	var users []models.User
	searchPattern := "%" + strings.TrimSpace(query) + "%"
	err := r.db.Where("name LIKE ? OR email LIKE ?", searchPattern, searchPattern).Find(&users).Error
	return users, err
}

// GetWithStats retrieves users with their statistics (image count, album count, storage usage)
func (r *userRepository) GetWithStats(offset, limit int) ([]UserWithStats, error) {
	var users []models.User
	err := r.db.Order("created_at DESC").Offset(offset).Limit(limit).Find(&users).Error
	if err != nil {
		return nil, err
	}

	var usersWithStats []UserWithStats
	for _, user := range users {
		stats, err := r.getUserStats(user.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get stats for user %d: %w", user.ID, err)
		}

		usersWithStats = append(usersWithStats, UserWithStats{
			User:         user,
			ImageCount:   stats.ImageCount,
			AlbumCount:   stats.AlbumCount,
			StorageUsage: stats.StorageUsage,
		})
	}

	return usersWithStats, nil
}

// SearchWithStats searches for users with their statistics
func (r *userRepository) SearchWithStats(query string) ([]UserWithStats, error) {
	users, err := r.Search(query)
	if err != nil {
		return nil, err
	}

	var usersWithStats []UserWithStats
	for _, user := range users {
		stats, err := r.getUserStats(user.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get stats for user %d: %w", user.ID, err)
		}

		usersWithStats = append(usersWithStats, UserWithStats{
			User:         user,
			ImageCount:   stats.ImageCount,
			AlbumCount:   stats.AlbumCount,
			StorageUsage: stats.StorageUsage,
		})
	}

	return usersWithStats, nil
}

// userStats represents internal user statistics
type userStats struct {
	ImageCount   int64
	AlbumCount   int64
	StorageUsage int64
}

// getUserStats retrieves statistics for a specific user
func (r *userRepository) getUserStats(userID uint) (*userStats, error) {
	var stats userStats

	// Get image count
	err := r.db.Model(&models.Image{}).Where("user_id = ?", userID).Count(&stats.ImageCount).Error
	if err != nil {
		return nil, fmt.Errorf("failed to count images: %w", err)
	}

	// Get album count
	err = r.db.Model(&models.Album{}).Where("user_id = ?", userID).Count(&stats.AlbumCount).Error
	if err != nil {
		return nil, fmt.Errorf("failed to count albums: %w", err)
	}

	// Get storage usage (sum of file sizes)
	err = r.db.Model(&models.Image{}).Where("user_id = ?", userID).
		Select("COALESCE(SUM(file_size), 0)").Row().Scan(&stats.StorageUsage)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate storage usage: %w", err)
	}

	return &stats, nil
}
