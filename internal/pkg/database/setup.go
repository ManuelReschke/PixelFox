package database

import (
	"fmt"
	"log"
	"time"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const maxRetries = 5
const retryDelay = 5 * time.Second

func SetupDatabase() {
	var err error
	// "user:pass@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local"
	// dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		env.GetEnv("DB_USER", ""),
		env.GetEnv("DB_PASSWORD", ""),
		env.GetEnv("DB_HOST", "127.0.0.1"),
		"3306", // env.GetEnv("DB_PORT", "5432"),
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
			DB.AutoMigrate(
				&models.User{},
				&models.Image{},
				&models.ImageMetadata{},
				&models.Album{},
				&models.Tag{},
				&models.Comment{},
				&models.Like{},
				&models.AlbumImage{},
				&models.ImageTag{},
				&models.Notification{},
				&models.News{},
				&models.Page{},
			)

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
