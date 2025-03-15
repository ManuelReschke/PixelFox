package statistics

import (
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/cache"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
)

const (
	CacheKeyImagesTotal = "statistics:images:total"
	CacheKeyImagesDaily = "statistics:images:daily:%s" // Format with date YYYY-MM-DD
	CacheKeyUsers       = "statistics:users:total"
	CacheExpiration     = 30 * time.Minute
)

// StatisticsData enthu00e4lt die Statistikdaten fu00fcr die Startseite
type StatisticsData struct {
	TodayImages int
	TotalUsers  int
	TotalImages int
}

// Variablen fu00fcr die Cache-Aktualisierungslogik
var (
	lastCacheUpdate     time.Time
	cacheUpdateMutex    sync.Mutex
	cacheUpdateInterval = 5 * time.Minute // Aktualisiere den Cache alle 5 Minuten
)

// ShouldUpdateCache pru00fcft, ob der Cache aktualisiert werden sollte
func ShouldUpdateCache() bool {
	cacheUpdateMutex.Lock()
	defer cacheUpdateMutex.Unlock()

	// Wenn der letzte Update lu00e4nger als das Intervall zuru00fcckliegt, sollte der Cache aktualisiert werden
	return time.Since(lastCacheUpdate) > cacheUpdateInterval
}

// UpdateCacheIfNeeded aktualisiert den Cache, wenn nu00f6tig
func UpdateCacheIfNeeded() {
	if ShouldUpdateCache() {
		cacheUpdateMutex.Lock()
		defer cacheUpdateMutex.Unlock()

		// Aktualisiere den Cache
		log.Println("Aktualisiere Statistik-Cache...")
		if err := UpdateStatisticsCache(); err != nil {
			log.Printf("Fehler beim Aktualisieren des Statistik-Caches: %v", err)
		} else {
			log.Println("Statistik-Cache erfolgreich aktualisiert")
			// Aktualisiere den Zeitstempel des letzten Updates
			lastCacheUpdate = time.Now()
		}
	}
}

// ResetCacheUpdateTimer setzt den Timer fu00fcr die Cache-Aktualisierung zuru00fcck
func ResetCacheUpdateTimer() {
	cacheUpdateMutex.Lock()
	defer cacheUpdateMutex.Unlock()

	lastCacheUpdate = time.Time{} // Setze auf Null-Zeit
}

// UpdateStatisticsCache updates all statistics in the cache
func UpdateStatisticsCache() error {
	// Get database connection
	db := database.GetDB()

	// Count total images
	var totalImages int64
	if err := db.Model(&models.Image{}).Count(&totalImages).Error; err != nil {
		log.Printf("Error counting total images: %v", err)
		return err
	}

	// Count today's images
	var todayImages int64
	today := time.Now().Format("2006-01-02")
	todayStart, _ := time.Parse("2006-01-02", today)
	todayEnd := todayStart.Add(24 * time.Hour)

	if err := db.Model(&models.Image{}).Where("created_at BETWEEN ? AND ?", todayStart, todayEnd).Count(&todayImages).Error; err != nil {
		log.Printf("Error counting today's images: %v", err)
		return err
	}

	// Count total users
	var totalUsers int64
	if err := db.Model(&models.User{}).Count(&totalUsers).Error; err != nil {
		log.Printf("Error counting total users: %v", err)
		return err
	}

	// Store values in cache
	if err := cache.Set(CacheKeyImagesTotal, strconv.FormatInt(totalImages, 10), CacheExpiration); err != nil {
		log.Printf("Error caching total images: %v", err)
		return err
	}

	dailyKey := fmt.Sprintf(CacheKeyImagesDaily, today)
	if err := cache.Set(dailyKey, strconv.FormatInt(todayImages, 10), CacheExpiration); err != nil {
		log.Printf("Error caching today's images: %v", err)
		return err
	}

	if err := cache.Set(CacheKeyUsers, strconv.FormatInt(totalUsers, 10), CacheExpiration); err != nil {
		log.Printf("Error caching total users: %v", err)
		return err
	}

	log.Printf("Statistics updated in cache: Total Images: %d, Today's Images: %d, Total Users: %d",
		totalImages, todayImages, totalUsers)

	return nil
}

// GetTotalImages returns the total number of images from cache or database
func GetTotalImages() int {
	// Try to get from cache first
	val, err := cache.Get(CacheKeyImagesTotal)
	if err != nil {
		// If not in cache, get from database and update cache
		var count int64
		db := database.GetDB()
		if err := db.Model(&models.Image{}).Count(&count).Error; err != nil {
			log.Printf("Error counting total images: %v", err)
			return 0
		}

		// Update cache
		if err := cache.Set(CacheKeyImagesTotal, strconv.FormatInt(count, 10), CacheExpiration); err != nil {
			log.Printf("Error caching total images: %v", err)
		}

		return int(count)
	}

	// Convert string to int
	count, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0
	}

	return int(count)
}

// GetTodayImages returns the number of images uploaded today from cache or database
func GetTodayImages() int {
	today := time.Now().Format("2006-01-02")
	dailyKey := fmt.Sprintf(CacheKeyImagesDaily, today)

	// Try to get from cache first
	val, err := cache.Get(dailyKey)
	if err != nil {
		// If not in cache, get from database and update cache
		var count int64
		db := database.GetDB()
		todayStart, _ := time.Parse("2006-01-02", today)
		todayEnd := todayStart.Add(24 * time.Hour)

		if err := db.Model(&models.Image{}).Where("created_at BETWEEN ? AND ?", todayStart, todayEnd).Count(&count).Error; err != nil {
			log.Printf("Error counting today's images: %v", err)
			return 0
		}

		// Update cache
		if err := cache.Set(dailyKey, strconv.FormatInt(count, 10), CacheExpiration); err != nil {
			log.Printf("Error caching today's images: %v", err)
		}

		return int(count)
	}

	// Convert string to int
	count, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0
	}

	return int(count)
}

// GetTotalUsers returns the total number of users from cache or database
func GetTotalUsers() int {
	// Try to get from cache first
	val, err := cache.Get(CacheKeyUsers)
	if err != nil {
		// If not in cache, get from database and update cache
		var count int64
		db := database.GetDB()
		if err := db.Model(&models.User{}).Count(&count).Error; err != nil {
			log.Printf("Error counting total users: %v", err)
			return 0
		}

		// Update cache
		if err := cache.Set(CacheKeyUsers, strconv.FormatInt(count, 10), CacheExpiration); err != nil {
			log.Printf("Error caching total users: %v", err)
		}

		return int(count)
	}

	// Convert string to int
	count, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0
	}

	return int(count)
}

// GetStatisticsData returns all statistics data as StatisticsData structure
func GetStatisticsData() StatisticsData {
	// Aktualisiere den Cache bei Bedarf
	UpdateCacheIfNeeded()

	return StatisticsData{
		TodayImages: GetTodayImages(),
		TotalUsers:  GetTotalUsers(),
		TotalImages: GetTotalImages(),
	}
}
