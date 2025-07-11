package models

import (
	"time"

	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

type Page struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Title     string         `gorm:"type:varchar(255);not null" json:"title" validate:"required,min=1,max=255"`
	Slug      string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"slug" validate:"required,min=1,max=255"`
	Content   string         `gorm:"type:longtext;not null" json:"content" validate:"required,min=1"`
	IsActive  bool           `gorm:"default:true" json:"is_active"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (p *Page) Validate() error {
	v := validator.New()
	return v.Struct(p)
}

func FindPageBySlug(db *gorm.DB, slug string) (*Page, error) {
	var page Page
	err := db.Where("slug = ? AND is_active = ?", slug, true).First(&page).Error
	if err != nil {
		return nil, err
	}
	return &page, nil
}

func FindPageByID(db *gorm.DB, id uint) (*Page, error) {
	var page Page
	err := db.First(&page, id).Error
	if err != nil {
		return nil, err
	}
	return &page, nil
}

func GetAllPages(db *gorm.DB) ([]Page, error) {
	var pages []Page
	err := db.Order("created_at DESC").Find(&pages).Error
	return pages, err
}

func GetActivePages(db *gorm.DB) ([]Page, error) {
	var pages []Page
	err := db.Where("is_active = ?", true).Order("created_at DESC").Find(&pages).Error
	return pages, err
}
