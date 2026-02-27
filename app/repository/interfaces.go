package repository

import (
	"time"

	"github.com/ManuelReschke/PixelFox/app/models"
	"gorm.io/gorm"
)

// UserRepository defines the interface for user-related database operations
type UserRepository interface {
	Create(user *models.User) error
	GetByID(id uint) (*models.User, error)
	GetByEmail(email string) (*models.User, error)
	GetByActivationToken(token string) (*models.User, error)
	GetByAPIKeyHash(hash string) (*models.User, *models.UserSettings, error)
	GetStatsByUserID(userID uint) (*UserStats, error)
	Update(user *models.User) error
	Delete(id uint) error
	List(offset, limit int) ([]models.User, error)
	Count() (int64, error)
	Search(query string) ([]models.User, error)
	GetWithStats(offset, limit int) ([]UserWithStats, error)
	SearchWithStats(query string) ([]UserWithStats, error)
	GetDailyStats(startDate, endDate time.Time) ([]models.DailyStats, error)
}

// ImageRepository defines the interface for image-related database operations
type ImageRepository interface {
	Create(image *models.Image) error
	GetByID(id uint) (*models.Image, error)
	GetByUUID(uuid string) (*models.Image, error)
	GetByFilename(filename string) (*models.Image, error)
	GetByShareLink(shareLink string) (*models.Image, error)
	GetByUserID(userID uint, offset, limit int) ([]models.Image, error)
	Update(image *models.Image) error
	Delete(id uint) error
	List(offset, limit int) ([]models.Image, error)
	Count() (int64, error)
	CountByUserID(userID uint) (int64, error)
	Search(query string) ([]models.Image, error)
	GetPublicImages(offset, limit int) ([]models.Image, error)
	GetRecentImages(limit int) ([]models.Image, error)
	UpdateViewCount(id uint) error
	UpdateDownloadCount(id uint) error
	GetVariants(imageID uint) ([]models.ImageVariant, error)
	DeleteVariants(imageID uint) error
	GetDailyStats(startDate, endDate time.Time) ([]models.DailyStats, error)
	GetByUserIDAndFileHash(userID uint, fileHash string) (*models.Image, error)
}

// AlbumRepository defines the interface for album-related database operations
type AlbumRepository interface {
	Create(album *models.Album) error
	GetByID(id uint) (*models.Album, error)
	GetByUserID(userID uint) ([]models.Album, error)
	Update(album *models.Album) error
	Delete(id uint) error
	AddImage(albumID, imageID uint) error
	RemoveImage(albumID, imageID uint) error
	GetImages(albumID uint) ([]models.Image, error)
	Count() (int64, error)
	CountByUserID(userID uint) (int64, error)
}

// StoragePoolRepository defines the interface for storage pool operations
type StoragePoolRepository interface {
	Create(pool *models.StoragePool) error
	GetByID(id uint) (*models.StoragePool, error)
	GetAll() ([]models.StoragePool, error)
	GetActive() ([]models.StoragePool, error)
	GetByTier(tier string) ([]models.StoragePool, error)
	GetOptimalForUpload(fileSize int64) (*models.StoragePool, error)
	GetOptimalForFile(fileSize int64) (*models.StoragePool, error)
	Update(pool *models.StoragePool) error
	Delete(id uint) error
	UpdateUsage(id uint, sizeChange int64) error
	GetStats(id uint) (*models.StoragePoolStats, error)
	GetAllStats() ([]models.StoragePoolStats, error)

	// Additional methods for admin storage management
	GetHealthStatus() (map[uint]bool, error)
	IsPoolHealthy(id uint) (bool, error)
	CountImagesInPool(poolID uint) (int64, error)
	CountVariantsInPool(poolID uint) (int64, error)
	RecalculatePoolUsage(poolID uint) (int64, error)
	GetHealthSnapshots() (map[uint]HealthSnapshot, error)
}

// SettingRepository defines the interface for application settings
type SettingRepository interface {
	Get() (*models.AppSettings, error)
	Save(settings *models.AppSettings) error
	GetValue(key string) (string, error)
	SetValue(key, value string) error
}

// PageRepository defines the interface for page-related operations
type PageRepository interface {
	Create(page *models.Page) error
	GetByID(id uint) (*models.Page, error)
	GetBySlug(slug string) (*models.Page, error)
	GetAll() ([]models.Page, error)
	GetActive() ([]models.Page, error)
	Update(page *models.Page) error
	Delete(id uint) error
	SlugExists(slug string) (bool, error)
	SlugExistsExceptID(slug string, id uint) (bool, error)
}

// NewsRepository defines the interface for news-related operations
type NewsRepository interface {
	Create(news *models.News) error
	GetByID(id uint) (*models.News, error)
	GetBySlug(slug string) (*models.News, error)
	GetPublished(offset, limit int) ([]models.News, error)
	GetAll(offset, limit int) ([]models.News, error)
	GetAllWithoutPagination() ([]models.News, error)
	Update(news *models.News) error
	Delete(id uint) error
	Count() (int64, error)
	SlugExists(slug string) (bool, error)
	SlugExistsExceptID(slug string, id uint) (bool, error)
}

// QueueRepository defines the interface for cache/queue operations
type QueueRepository interface {
	GetAllKeys() ([]string, error)
	GetValue(key string) (string, error)
	GetTTL(key string) (time.Duration, error)
	DeleteKey(key string) (int64, error)
	GetListLength(key string) (int64, error)
	FindKeysByPatterns(patterns []string) ([]string, error)
	DeleteKeys(keys []string) (int64, error)
}

// UserWithStats represents a user with additional statistics
type UserWithStats struct {
	User         models.User
	ImageCount   int64
	AlbumCount   int64
	StorageUsage int64
}

// UserStats provides aggregated counts for a single user (images, albums, storage usage).
type UserStats struct {
	ImageCount   int64
	AlbumCount   int64
	StorageUsage int64
}

// Repositories struct holds all repository instances
type Repositories struct {
	User        UserRepository
	Image       ImageRepository
	Album       AlbumRepository
	StoragePool StoragePoolRepository
	Setting     SettingRepository
	Page        PageRepository
	News        NewsRepository
	Queue       QueueRepository
}

// NewRepositories creates a new instance of all repositories
func NewRepositories(db *gorm.DB) *Repositories {
	return &Repositories{
		User:        NewUserRepository(db),
		Image:       NewImageRepository(db),
		Album:       NewAlbumRepository(db),
		StoragePool: NewStoragePoolRepository(db),
		Setting:     NewSettingRepository(db),
		Page:        NewPageRepository(db),
		News:        NewNewsRepository(db),
		Queue:       NewQueueRepository(),
	}
}
