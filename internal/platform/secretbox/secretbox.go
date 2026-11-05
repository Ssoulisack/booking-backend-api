// Package secretbox encrypts small secrets (a bot token, a chat id) before they are stored, so the database never
// holds a usable credential. AES-256-GCM with a key made from the server's own secret; every value gets its own
// random nonce, so equal secrets look different. A changed server secret makes old values unreadable (they must
// be entered again): Open then fails instead of returning garbage.
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
)

const prefix = "v1:"

// ErrUnreadable: the value is not one this box sealed (wrong key, damaged, or not encrypted at all)
var ErrUnreadable = errors.New("secretbox: value cannot be opened")

// Box seals and opens values with one key
type Box struct{ gcm cipher.AEAD }

// New makes a box from secret material (a long server secret). The 32-byte key is derived from it with HMAC-SHA256
// and a fixed label, so the same material gives the same key and the material itself is never used as the key.
func New(material, purpose string) (*Box, error) {
	if len(strings.TrimSpace(material)) < 16 {
		return nil, errors.New("secretbox: the server secret is too short to encrypt with")
	}
	mac := hmac.New(sha256.New, []byte(material))
	mac.Write([]byte("secretbox:v1:" + purpose))
	block, err := aes.NewCipher(mac.Sum(nil))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{gcm: gcm}, nil
}

// Seal encrypts plain text into "v1:<base64 of nonce+ciphertext>"; "" stays ""
func (b *Box) Seal(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	nonce := make([]byte, b.gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	out := b.gcm.Seal(nonce, nonce, []byte(plain), nil)
	return prefix + base64.RawURLEncoding.EncodeToString(out), nil
}

// Open decrypts a value made by Seal; "" stays ""
func (b *Box) Open(sealed string) (string, error) {
	if sealed == "" {
		return "", nil
	}
	if !strings.HasPrefix(sealed, prefix) {
		return "", ErrUnreadable
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(sealed, prefix))
	if err != nil || len(raw) < b.gcm.NonceSize() {
		return "", ErrUnreadable
	}
	plain, err := b.gcm.Open(nil, raw[:b.gcm.NonceSize()], raw[b.gcm.NonceSize():], nil)
	if err != nil {
		return "", ErrUnreadable
	}
	return string(plain), nil
}
