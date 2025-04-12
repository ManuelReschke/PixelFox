package models

import (
	"time"

	"github.com/go-playground/validator/v10"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type User struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"type:varchar(150)" json:"name" validate:"required,min=3,max=150"`
	Email       string         `gorm:"uniqueIndex;type:varchar(150) CHARACTER SET utf8 COLLATE utf8_bin" json:"email" validate:"required,email,min=6,max=150"`
	Password    string         `gorm:"type:text" json:"-" validate:"required,min=6"`
	Role        string         `gorm:"type:varchar(50);default:'user'" json:"role" validate:"oneof=user admin"`
	Status      string         `gorm:"type:varchar(50);default:'active'" json:"status" validate:"oneof=active inactive disabled"`
	LastLoginAt *time.Time     `gorm:"type:timestamp;default:null" json:"last_login_at"`
	CreatedAt   time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (u *User) Validate() error {
	v := validator.New()

	return v.Struct(u)
}

func CreateUser(username string, email string, password string) (*User, error) {
	pw, err := HashPassword(password)
	if err != nil {
		return nil, err
	}

	u := &User{
		Name:     username,
		Email:    email,
		Password: pw,
		Role:     "user",
		Status:   "active",
	}

	err = u.Validate()
	if err != nil {
		return nil, err
	}

	return u, nil
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	return string(bytes), err
}

// CheckPasswordHash compares the given password with the stored hash.
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))

	return err == nil
}
