// Package crypto provides AES-256-GCM authenticated encryption for the
// Comax Secrets server and the abstraction for loading the master key.
//
// On-disk layout for a sealed value (the BLOB stored in
// secrets.ciphertext):
//
//	[ nonce (12 bytes) ][ ciphertext ][ GCM tag (16 bytes) ]
//
// This matches what Go's crypto/cipher.AEAD.Seal produces when its first
// argument is the nonce prefix, so callers don't have to remember layout
// rules — they just hand the BLOB back to Open.
//
// Keys are always 32 bytes (AES-256). The KeyProvider interface
// (provider.go) is the rotation/abstraction seam — Milestone 1 ships only
// FileKeyProvider, but the API is shaped to host KMS/keyring providers
// later without breaking call sites.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

const (
	// KeySize is the required master-key length in bytes (AES-256).
	KeySize = 32
	// NonceSize is the GCM standard nonce length in bytes.
	NonceSize = 12
	// TagSize is the GCM authentication tag length in bytes.
	TagSize = 16
)

// ErrCiphertextTooShort is returned when Open receives a buffer that
// can't possibly contain the nonce + tag overhead.
var ErrCiphertextTooShort = errors.New("crypto: ciphertext too short")

// ErrInvalidKeySize is returned by Seal/Open when the supplied key is
// not exactly KeySize bytes.
var ErrInvalidKeySize = errors.New("crypto: invalid key size")

// Seal encrypts plaintext under key and returns nonce || ciphertext+tag
// as a single byte slice.
//
// A fresh 12-byte nonce is drawn from crypto/rand on every call. Reusing
// a (key, nonce) pair under GCM is catastrophic, so callers MUST NOT
// supply their own nonce.
func Seal(key, plaintext []byte) ([]byte, error) {
	gcm, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("read nonce: %w", err)
	}
	// AEAD.Seal appends ciphertext+tag to dst (first arg); reusing the
	// nonce slice for dst gives us nonce || ct+tag in one buffer.
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Open decrypts a buffer produced by Seal. The returned slice is the
// original plaintext.
//
// Returns ErrCiphertextTooShort when the input is too small to contain
// nonce + tag, and wraps the GCM authentication error when the tag does
// not verify.
func Open(key, sealed []byte) ([]byte, error) {
	gcm, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	if len(sealed) < gcm.NonceSize()+gcm.Overhead() {
		return nil, ErrCiphertextTooShort
	}
	nonce, ct := sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("gcm open: %w", err)
	}
	return plain, nil
}

// newAEAD validates key size and returns a GCM AEAD. Extracted so Seal
// and Open share the same key-validation error.
func newAEAD(key []byte) (cipher.AEAD, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("%w: got %d bytes, want %d", ErrInvalidKeySize, len(key), KeySize)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}
	return gcm, nil
}
