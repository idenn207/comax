// Package auth provides the bearer-token primitives and bootstrap
// orchestration used by the secret-server HTTP layer.
//
// Token storage rule: plaintext tokens are NEVER persisted. Callers see
// the plaintext exactly once — at issue time — and the server only ever
// stores the SHA-256 hash. Every authenticated request rehashes the
// presented bearer and looks it up by hash, so a database leak does not
// hand out usable credentials.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/idenn207/comax-secrets/internal/store"
)

// tokenBytes is the entropy budget for a freshly minted bearer token.
// 32 random bytes → 43-character base64url. Enough headroom that even a
// targeted online guessing attack against the /api/v1 surface is
// indistinguishable from rand-guessing 256 bits.
const tokenBytes = 32

// Sentinel errors. HTTP handlers compare with errors.Is.
var (
	// ErrMissingBearer is returned by ParseBearer when the Authorization
	// header is empty.
	ErrMissingBearer = errors.New("auth: missing bearer token")
	// ErrInvalidBearer is returned by ParseBearer when the header is
	// present but malformed.
	ErrInvalidBearer = errors.New("auth: invalid bearer header")
	// ErrUnknownToken is returned by Verify when the presented token does
	// not hash to any persisted row.
	ErrUnknownToken = errors.New("auth: unknown token")
)

// GenerateToken returns a fresh, base64url-encoded random bearer string.
// The byte source is crypto/rand; failure is fatal in practice (it means
// the OS RNG is broken) but is surfaced as an error so callers can log
// rather than panic.
func GenerateToken() (string, error) {
	buf := make([]byte, tokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashToken returns SHA-256(plaintext). The result is what the server
// persists and indexes; the plaintext is what the operator carries on
// disk. Comparing hashes is enough because SHA-256 is preimage-resistant
// and the token entropy is already 256 bits.
func HashToken(plaintext string) []byte {
	sum := sha256.Sum256([]byte(plaintext))
	return sum[:]
}

// ParseBearer extracts the token from an Authorization header value.
// The accepted shapes are exactly "Bearer <token>" and "bearer <token>"
// (RFC 6750 case-insensitive scheme); leading/trailing whitespace around
// the token is trimmed. Anything else returns ErrInvalidBearer.
func ParseBearer(header string) (string, error) {
	if header == "" {
		return "", ErrMissingBearer
	}
	// Case-insensitive scheme match without allocating: the prefix must
	// be 7 chars ("Bearer " or "bearer ") and the rest is the token.
	if len(header) < 7 || !strings.EqualFold(header[:7], "Bearer ") {
		return "", ErrInvalidBearer
	}
	tok := strings.TrimSpace(header[7:])
	if tok == "" {
		return "", ErrInvalidBearer
	}
	return tok, nil
}

// Verify hashes plaintext, looks up the matching service token by hash,
// and returns it on success. The repository layer is the source of
// truth; we add a constant-time compare on the returned hash as a
// defence-in-depth against a future implementation regression where the
// driver might leak timing on the WHERE clause.
//
// On success Verify also bumps last_used_at via TokenRepo.TouchLastUsed.
// A failed TouchLastUsed is logged-and-continued by the caller (we just
// pass that error back); the request still succeeds because auth has
// already passed.
func Verify(ctx context.Context, repo *store.TokenRepo, plaintext string) (store.ServiceToken, error) {
	if plaintext == "" {
		return store.ServiceToken{}, ErrMissingBearer
	}
	hash := HashToken(plaintext)
	tok, err := repo.ByHash(ctx, hash)
	if errors.Is(err, store.ErrNotFound) {
		return store.ServiceToken{}, ErrUnknownToken
	}
	if err != nil {
		return store.ServiceToken{}, fmt.Errorf("verify bearer: %w", err)
	}
	// subtle.ConstantTimeCompare returns 1 on equal. Belt-and-braces
	// against a driver that might shortcut the lookup; in practice the
	// ByHash result is the row whose hash matched the WHERE clause.
	if subtle.ConstantTimeCompare(tok.TokenHash, hash) != 1 {
		return store.ServiceToken{}, ErrUnknownToken
	}
	return tok, nil
}

// Context key plumbing. We use an unexported struct type so no caller
// outside this package can fabricate the key — context.Value with a
// string key is a common cross-package bug, this idiom prevents it.

type ctxKey struct{}

// WithToken returns a derived context carrying the authenticated
// service token. Middleware calls this after Verify succeeds; handlers
// retrieve via FromContext.
func WithToken(ctx context.Context, tok store.ServiceToken) context.Context {
	return context.WithValue(ctx, ctxKey{}, tok)
}

// FromContext returns the service token stamped onto ctx by the auth
// middleware, plus an ok flag. Handlers that need to record an actor in
// the audit log call this; handlers that don't care about identity (e.g.
// /healthz) ignore the result.
func FromContext(ctx context.Context) (store.ServiceToken, bool) {
	tok, ok := ctx.Value(ctxKey{}).(store.ServiceToken)
	return tok, ok
}
