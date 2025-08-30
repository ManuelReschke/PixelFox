package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type UploadTokenClaims struct {
	UserID    uint  `json:"user_id"`
	PoolID    uint  `json:"pool_id"`
	MaxBytes  int64 `json:"max_bytes"`
	ExpiresAt int64 `json:"exp"`
}

func GenerateUploadToken(userID, poolID uint, maxBytes int64, ttl time.Duration, secret string) (string, error) {
	if secret == "" {
		return "", errors.New("secret is required for token generation")
	}
	claims := UploadTokenClaims{
		UserID:    userID,
		PoolID:    poolID,
		MaxBytes:  maxBytes,
		ExpiresAt: time.Now().Add(ttl).Unix(),
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	sig := mac.Sum(nil)
	token := fmt.Sprintf("%s.%s", base64.RawURLEncoding.EncodeToString(payload), base64.RawURLEncoding.EncodeToString(sig))
	return token, nil
}

func VerifyUploadToken(token, secret string) (*UploadTokenClaims, error) {
	if secret == "" {
		return nil, errors.New("secret is required for token verification")
	}
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return nil, errors.New("invalid token format")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("invalid payload encoding")
	}
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("invalid signature encoding")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payloadBytes)
	expected := mac.Sum(nil)
	if !hmac.Equal(sigBytes, expected) {
		return nil, errors.New("invalid token signature")
	}
	var claims UploadTokenClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, errors.New("invalid payload")
	}
	if time.Now().Unix() > claims.ExpiresAt {
		return nil, errors.New("token expired")
	}
	return &claims, nil
}
