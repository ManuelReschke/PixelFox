package models

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/go-playground/validator/v10"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	ROLE_USER       = "user"
	ROLE_ADMIN      = "admin"
	STATUS_ACTIVE   = "active"
	STATUS_INACTIVE = "inactive"
	STATUS_DISABLED = "disabled"
)

type User struct {
	ID               uint           `gorm:"primaryKey" json:"id"`
	Name             string         `gorm:"type:varchar(150)" json:"name" validate:"required,min=3,max=150"`
	Email            string         `gorm:"uniqueIndex;type:varchar(200) CHARACTER SET utf8 COLLATE utf8_bin" json:"email" validate:"required,email,min=5,max=200"`
	Password         string         `gorm:"type:text" json:"-" validate:"required,min=6"`
	Role             string         `gorm:"type:varchar(50);default:'user'" json:"role" validate:"oneof=user admin"`
	Status           string         `gorm:"type:varchar(50);default:'active'" json:"status" validate:"oneof=active inactive disabled"`
	Bio              string         `gorm:"type:text;default:null" json:"bio" validate:"max=1000"`
	AvatarURL        string         `gorm:"type:varchar(255);default:null" json:"avatar_url" validate:"max=255"`
	IPv4             string         `gorm:"type:varchar(15);default:null" json:"-"`
	IPv6             string         `gorm:"type:varchar(45);default:null" json:"-"`
	ActivationToken  string         `gorm:"type:varchar(100);index" json:"-"`
	ActivationSentAt *time.Time     `gorm:"type:timestamp;default:null" json:"-"`
	LastLoginAt      *time.Time     `gorm:"type:timestamp;default:null" json:"last_login_at"`
	CreatedAt        time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
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
		Role:     ROLE_USER,
		Status:   STATUS_INACTIVE,
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

// GenerateActivationToken creates a random token and sets ActivationSentAt
func (u *User) GenerateActivationToken() error {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	u.ActivationToken = hex.EncodeToString(b)
	now := time.Now()
	u.ActivationSentAt = &now
	return nil
}

// IsActive reports whether the user status is active
func (u *User) IsActive() bool {
	return u.Status == STATUS_ACTIVE
}
