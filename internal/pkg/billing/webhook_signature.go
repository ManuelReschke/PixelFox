package billing

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"strings"
)

func VerifyPatreonWebhookSignature(payload []byte, signatureHeader, webhookSecret string) bool {
	sig := strings.TrimSpace(signatureHeader)
	secret := strings.TrimSpace(webhookSecret)
	if sig == "" || secret == "" {
		return false
	}

	decodedSig, err := hex.DecodeString(strings.ToLower(sig))
	if err != nil {
		return false
	}

	// Patreon docs describe HMAC-MD5 for X-Patreon-Signature.
	if verifyHMAC(payload, decodedSig, []byte(secret), md5.New) {
		return true
	}
	// Backward-compatible fallback in case environments were configured for SHA256.
	return verifyHMAC(payload, decodedSig, []byte(secret), sha256.New)
}

func verifyHMAC(payload, expectedSig, secret []byte, hashFunc func() hash.Hash) bool {
	mac := hmac.New(hashFunc, secret)
	mac.Write(payload)
	return hmac.Equal(mac.Sum(nil), expectedSig)
}
