package repository

import (
	"sync"

	"gorm.io/gorm"
)

// Factory manages repository instances and ensures they are singletons
type Factory struct {
	db    *gorm.DB
	repos *Repositories
	once  sync.Once
}

// NewFactory creates a new repository factory
func NewFactory(db *gorm.DB) *Factory {
	return &Factory{
		db: db,
	}
}

// GetRepositories returns a singleton instance of all repositories
func (f *Factory) GetRepositories() *Repositories {
	f.once.Do(func() {
		f.repos = NewRepositories(f.db)
	})
	return f.repos
}

// GetUserRepository returns the user repository instance
func (f *Factory) GetUserRepository() UserRepository {
	return f.GetRepositories().User
}

// GetImageRepository returns the image repository instance
func (f *Factory) GetImageRepository() ImageRepository {
	return f.GetRepositories().Image
}

// GetAlbumRepository returns the album repository instance
func (f *Factory) GetAlbumRepository() AlbumRepository {
	return f.GetRepositories().Album
}

// GetStoragePoolRepository returns the storage pool repository instance
func (f *Factory) GetStoragePoolRepository() StoragePoolRepository {
	return f.GetRepositories().StoragePool
}

// GetSettingRepository returns the setting repository instance
func (f *Factory) GetSettingRepository() SettingRepository {
	return f.GetRepositories().Setting
}

// GetPageRepository returns the page repository instance
func (f *Factory) GetPageRepository() PageRepository {
	return f.GetRepositories().Page
}

// GetNewsRepository returns the news repository instance
func (f *Factory) GetNewsRepository() NewsRepository {
	return f.GetRepositories().News
}

// GetQueueRepository returns the queue repository instance
func (f *Factory) GetQueueRepository() QueueRepository {
	return f.GetRepositories().Queue
}

// Global factory instance
var globalFactory *Factory
var factoryOnce sync.Once

// InitializeFactory initializes the global repository factory
func InitializeFactory(db *gorm.DB) {
	factoryOnce.Do(func() {
		globalFactory = NewFactory(db)
	})
}

// GetGlobalFactory returns the global repository factory instance
func GetGlobalFactory() *Factory {
	if globalFactory == nil {
		panic("Repository factory not initialized. Call InitializeFactory first.")
	}
	return globalFactory
}

// GetGlobalRepositories returns the global repositories instance
func GetGlobalRepositories() *Repositories {
	return GetGlobalFactory().GetRepositories()
}
