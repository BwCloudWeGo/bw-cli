package repo

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

// SHA256Hasher stores salted SHA-256 hashes in the form salt:hash.
type SHA256Hasher struct{}

// NewSHA256Hasher constructs the demo password hasher.
func NewSHA256Hasher() SHA256Hasher {
	return SHA256Hasher{}
}

func (SHA256Hasher) Hash(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	return encodeSaltedHash(salt, password), nil
}

func (SHA256Hasher) Verify(hash string, password string) bool {
	saltText, hashText, ok := strings.Cut(hash, ":")
	if !ok || saltText == "" || hashText == "" {
		return false
	}
	salt, err := base64.RawURLEncoding.DecodeString(saltText)
	if err != nil {
		return false
	}
	expected := encodeSaltedHash(salt, password)
	return subtle.ConstantTimeCompare([]byte(hash), []byte(expected)) == 1
}

func encodeSaltedHash(salt []byte, password string) string {
	sum := sha256.Sum256(append(append([]byte{}, salt...), []byte(password)...))
	return base64.RawURLEncoding.EncodeToString(salt) + ":" + hex.EncodeToString(sum[:])
}
