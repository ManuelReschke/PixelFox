package models

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// UserSettings stores per-user preferences and plan info
type UserSettings struct {
	ID                uint           `gorm:"primaryKey" json:"id"`
	UserID            uint           `gorm:"uniqueIndex" json:"user_id"`
	Plan              string         `gorm:"type:varchar(50);default:'free'" json:"plan"`
	PrefThumbOriginal bool           `gorm:"default:true" json:"pref_thumb_original"`
	PrefThumbWebP     bool           `gorm:"default:false" json:"pref_thumb_webp"`
	PrefThumbAVIF     bool           `gorm:"default:false" json:"pref_thumb_avif"`
	APIKeyHash        string         `gorm:"type:char(64);default:''" json:"-"`
	APIKeyPrefix      string         `gorm:"type:varchar(20);default:''" json:"api_key_prefix"`
	APIKeyCreatedAt   *time.Time     `json:"api_key_created_at"`
	APIKeyLastUsedAt  *time.Time     `json:"api_key_last_used_at"`
	APIKeyRevokedAt   *time.Time     `json:"api_key_revoked_at"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`
}

var apiKeyEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

const apiKeyPrefix = "pxl_"

// GetOrCreateUserSettings returns existing settings or creates defaults
func GetOrCreateUserSettings(db *gorm.DB, userID uint) (*UserSettings, error) {
	var us UserSettings
	if err := db.Where("user_id = ?", userID).First(&us).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			us = UserSettings{UserID: userID, Plan: "free", PrefThumbOriginal: true}
			if err := db.Create(&us).Error; err != nil {
				return nil, err
			}
			return &us, nil
		}
		return nil, err
	}
	return &us, nil
}

// HasActiveAPIKey reports whether the user has an active API key configured
func (us *UserSettings) HasActiveAPIKey() bool {
	return us != nil && us.APIKeyHash != "" && us.APIKeyRevokedAt == nil
}

// IssueAPIKey generates a new API key, persists metadata on the struct, and returns the raw secret.
// Callers must persist the struct via the database after invoking this method.
func (us *UserSettings) IssueAPIKey() (string, error) {
	rawKey, prefix, hash, err := generateAPIKeyMaterial()
	if err != nil {
		return "", err
	}
	now := time.Now()
	us.APIKeyHash = hash
	us.APIKeyPrefix = prefix
	us.APIKeyCreatedAt = &now
	us.APIKeyRevokedAt = nil
	us.APIKeyLastUsedAt = nil
	return rawKey, nil
}

// RevokeAPIKey clears the stored API key metadata without deleting the record.
func (us *UserSettings) RevokeAPIKey() {
	us.APIKeyHash = ""
	us.APIKeyPrefix = ""
	now := time.Now()
	us.APIKeyRevokedAt = &now
	us.APIKeyLastUsedAt = nil
}

// TouchAPIKeyUsage updates the last-used timestamp metadata.
func (us *UserSettings) TouchAPIKeyUsage() {
	now := time.Now()
	us.APIKeyLastUsedAt = &now
}

// HashAPIKey returns the SHA-256 hash for the provided API key.
func HashAPIKey(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:])
}

func generateAPIKeyMaterial() (string, string, string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", "", err
	}
	encoded := apiKeyEncoding.EncodeToString(b)
	encoded = strings.ToLower(encoded)
	rawKey := apiKeyPrefix + encoded
	if len(rawKey) < 12 {
		return "", "", "", fmt.Errorf("api key generation failed: key too short")
	}
	prefix := rawKey[:min(len(rawKey), 16)]
	hash := HashAPIKey(rawKey)
	return rawKey, prefix, hash, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
