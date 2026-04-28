package repo

import (
	"crypto/sha256"
	"encoding/hex"
)

// SHA256Hasher is a simple demo hasher; replace with bcrypt or argon2 in production.
type SHA256Hasher struct{}

// NewSHA256Hasher constructs the demo password hasher.
func NewSHA256Hasher() SHA256Hasher {
	return SHA256Hasher{}
}

func (SHA256Hasher) Hash(password string) (string, error) {
	sum := sha256.Sum256([]byte(password))
	return hex.EncodeToString(sum[:]), nil
}

func (SHA256Hasher) Verify(hash string, password string) bool {
	sum := sha256.Sum256([]byte(password))
	return hash == hex.EncodeToString(sum[:])
}
