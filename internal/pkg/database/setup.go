package database

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const maxRetries = 5
const retryDelay = 5 * time.Second
const defaultAutoMigrateLockTimeoutSec = 120
const defaultAutoMigrateLockName = "pixelfox_automigrate"

func SetupDatabase() {
	var err error
	// "user:pass@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local"
	// dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		env.GetEnv("DB_USER", ""),
		env.GetEnv("DB_PASSWORD", ""),
		env.GetEnv("DB_HOST", "127.0.0.1"),
		env.GetEnv("DB_PORT", "3306"),
		env.GetEnv("DB_NAME", ""),
	)

	for i := 0; i < maxRetries; i++ {
		DB, err = gorm.Open(mysql.New(mysql.Config{
			DSN:                       dsn,   // data source name
			DefaultStringSize:         256,   // default size for string fields
			DisableDatetimePrecision:  true,  // disable datetime precision, which not supported before MySQL 5.6
			DontSupportRenameIndex:    true,  // drop & create when rename index, rename index not supported before MySQL 5.7, MariaDB
			DontSupportRenameColumn:   true,  // `change` when rename column, rename column not supported before MySQL 8, MariaDB
			SkipInitializeWithVersion: false, // auto configure based on currently MySQL version
		}), &gorm.Config{})
		if err == nil {
			// Configure optimized connection pool settings
			sqlDB, err := DB.DB()
			if err != nil {
				log.Printf("Failed to get underlying sql.DB: %v", err)
			} else {
				// Optimize connection pool for higher load
				sqlDB.SetMaxIdleConns(10)                  // Max idle connections in pool (default: 2)
				sqlDB.SetMaxOpenConns(100)                 // Max concurrent connections (default: unlimited)
				sqlDB.SetConnMaxLifetime(time.Hour)        // Connection lifetime (default: never expire)
				sqlDB.SetConnMaxIdleTime(10 * time.Minute) // Max idle time before closing (MySQL 8.0+)

				log.Printf("Database connection pool configured: MaxIdle=10, MaxOpen=100, MaxLifetime=1h")
			}

			if shouldAutoMigrate() {
				if err := runAutoMigrateWithLock(DB); err != nil {
					panic(fmt.Errorf("auto migrate failed: %w", err))
				}
			} else {
				log.Printf("AutoMigrate disabled by DB_AUTO_MIGRATE")
			}

			// Load settings into memory
			err = models.LoadSettings(DB)
			if err != nil {
				log.Printf("Warning: Failed to load settings: %v", err)
			}

			// Apply sane defaults for default storage pool new fields
			applyStoragePoolDefaults()

			return
		}

		log.Printf("Failed to connect to database (try %d/%d): %v", i+1, maxRetries, err)
		if i < maxRetries-1 {
			log.Printf("Retry number %v...", retryDelay)
			time.Sleep(retryDelay)
		}
	}

	if err != nil {
		panic(err)
	}
}

func shouldAutoMigrate() bool {
	val := strings.ToLower(strings.TrimSpace(env.GetEnv("DB_AUTO_MIGRATE", "1")))
	switch val {
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

func runAutoMigrateWithLock(db *gorm.DB) error {
	lockName := env.GetEnv("DB_AUTO_MIGRATE_LOCK_NAME", defaultAutoMigrateLockName)
	lockTimeoutSec := defaultAutoMigrateLockTimeoutSec
	if timeoutRaw := strings.TrimSpace(env.GetEnv("DB_AUTO_MIGRATE_LOCK_TIMEOUT_SEC", "")); timeoutRaw != "" {
		timeout, err := strconv.Atoi(timeoutRaw)
		if err != nil || timeout < 1 {
			return fmt.Errorf("invalid DB_AUTO_MIGRATE_LOCK_TIMEOUT_SEC value %q", timeoutRaw)
		}
		lockTimeoutSec = timeout
	}

	var lockAcquired int
	if err := db.Raw("SELECT GET_LOCK(?, ?)", lockName, lockTimeoutSec).Scan(&lockAcquired).Error; err != nil {
		return fmt.Errorf("failed to acquire automigrate lock %q: %w", lockName, err)
	}
	if lockAcquired != 1 {
		return fmt.Errorf("failed to acquire automigrate lock %q within %ds", lockName, lockTimeoutSec)
	}
	defer func() {
		var released int
		if err := db.Raw("SELECT RELEASE_LOCK(?)", lockName).Scan(&released).Error; err != nil {
			log.Printf("Warning: failed to release automigrate lock %q: %v", lockName, err)
			return
		}
		if released != 1 {
			log.Printf("Warning: automigrate lock %q was not released cleanly (result=%d)", lockName, released)
		}
	}()

	if err := runAutoMigrate(db); err != nil {
		return err
	}

	return nil
}

func runAutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.ProviderAccount{},
		&models.UserSettings{},
		&models.Image{},
		&models.ImageVariant{},
		&models.ImageMetadata{},
		&models.ImageReport{},
		&models.Album{},
		&models.Tag{},
		&models.Comment{},
		&models.Like{},
		&models.AlbumImage{},
		&models.ImageTag{},
		&models.Notification{},
		&models.News{},
		&models.Page{},
		&models.Setting{},
		&models.StoragePool{},
	)
}

// applyStoragePoolDefaults ensures default pool has public_base_url/upload_api_url/node_id set
func applyStoragePoolDefaults() {
	db := GetDB()
	if db == nil {
		return
	}
	pool, err := models.FindDefaultStoragePool(db)
	if err != nil || pool == nil {
		return
	}
	changed := false
	if pool.PublicBaseURL == "" {
		pool.PublicBaseURL = env.GetEnv("PUBLIC_DOMAIN", "")
		changed = true
	}
	if pool.UploadAPIURL == "" && pool.PublicBaseURL != "" {
		base := pool.PublicBaseURL
		if len(base) > 0 && base[len(base)-1] == '/' {
			base = base[:len(base)-1]
		}
		pool.UploadAPIURL = base + "/api/internal/upload"
		changed = true
	}
	if pool.NodeID == "" {
		pool.NodeID = "local"
		changed = true
	}
	if changed {
		if err := db.Save(pool).Error; err != nil {
			log.Printf("Warning: failed to apply storage pool defaults: %v", err)
		}
	}
}
