package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

const (
	argon2Time    = 1
	argon2Memory  = 64 * 1024
	argon2Threads = 4
	argon2KeyLen  = 32
	saltLen       = 16
)

// HashPassword creates an argon2id hash of the password
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)

	// Encode as salt:hash in hex
	return hex.EncodeToString(salt) + ":" + hex.EncodeToString(hash), nil
}

// VerifyPassword checks if a password matches a hash
func VerifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, ":")
	if len(parts) != 2 {
		return false
	}

	salt, err := hex.DecodeString(parts[0])
	if err != nil {
		return false
	}

	expectedHash, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}

	hash := argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)

	return subtle.ConstantTimeCompare(hash, expectedHash) == 1
}

// Auth handles cookie signing for unlocked shares
type Auth struct {
	secret []byte
}

// NewAuth creates a new Auth with the given secret
func NewAuth(secret string) *Auth {
	// If no secret provided, generate a random one (cookies won't survive restart)
	if secret == "" {
		s := make([]byte, 32)
		rand.Read(s)
		return &Auth{secret: s}
	}
	return &Auth{secret: []byte(secret)}
}

// cookieName returns the cookie name for a share
func (a *Auth) cookieName(shareID string) string {
	return "unlock_" + shareID
}

// sign creates an HMAC signature for a value
func (a *Auth) sign(value string) string {
	mac := hmac.New(sha256.New, a.secret)
	mac.Write([]byte(value))
	return base64.URLEncoding.EncodeToString(mac.Sum(nil))
}

// SetUnlockCookie sets a signed cookie indicating the share is unlocked
func (a *Auth) SetUnlockCookie(w http.ResponseWriter, shareID string) {
	// Value is shareID:timestamp
	value := fmt.Sprintf("%s:%d", shareID, time.Now().Unix())
	signature := a.sign(value)
	cookieValue := value + "." + signature

	http.SetCookie(w, &http.Cookie{
		Name:     a.cookieName(shareID),
		Value:    cookieValue,
		Path:     "/",
		MaxAge:   86400, // 24 hours
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// IsUnlocked checks if the share has a valid unlock cookie
func (a *Auth) IsUnlocked(r *http.Request, shareID string) bool {
	cookie, err := r.Cookie(a.cookieName(shareID))
	if err != nil {
		return false
	}

	// Split value and signature
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 {
		return false
	}

	value := parts[0]
	signature := parts[1]

	// Verify signature
	expectedSig := a.sign(value)
	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		return false
	}

	// Verify the value starts with the share ID
	if !strings.HasPrefix(value, shareID+":") {
		return false
	}

	return true
}
