package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"strings"
)

// ErrCSRFMismatch is returned by VerifyCSRF when the presented token
// does not match the hash bound to the session. HTTP handlers map this
// to a 403 envelope.
var ErrCSRFMismatch = errors.New("auth: csrf mismatch")

// csrfBytes mirrors tokenBytes (32 random bytes → 43-char base64url).
// Same entropy budget as a bearer token because both gate the same
// blast radius — write access to the dashboard surface.
const csrfBytes = 32

// GenerateCSRF returns a fresh base64url CSRF token. Same crypto/rand
// source as bearer tokens; the encoding is URL-safe so the value can be
// shipped through headers and JSON bodies without escaping.
func GenerateCSRF() (string, error) {
	buf := make([]byte, csrfBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate csrf: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashCSRF returns SHA-256(plaintext). The server persists the hash so a
// dashboard_sessions row leak does not hand out usable CSRF credentials.
func HashCSRF(plaintext string) []byte {
	sum := sha256.Sum256([]byte(plaintext))
	return sum[:]
}

// VerifyCSRF constant-time-compares the SHA-256 of presented against the
// expected hash. Returns ErrCSRFMismatch on any difference.
//
// Why we hash the presented value before comparing instead of comparing
// presented to a stored plaintext: the stored value is the hash — the
// plaintext was emitted to the browser exactly once at session creation
// time. A leaked dashboard_sessions row therefore does not expose a
// usable CSRF token.
func VerifyCSRF(presented string, expectedHash []byte) error {
	if presented == "" || len(expectedHash) == 0 {
		return ErrCSRFMismatch
	}
	got := HashCSRF(presented)
	if subtle.ConstantTimeCompare(got, expectedHash) != 1 {
		return ErrCSRFMismatch
	}
	return nil
}

// IPPrefix returns the privacy-preserving prefix of a remote IP address:
// /24 for IPv4 and /48 for IPv6. The full IP is never persisted, so a
// dashboard_sessions dump cannot pinpoint an operator's location. addr
// may include a port (as RemoteAddr usually does) or be empty.
func IPPrefix(addr string) string {
	if addr == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return ""
	}
	if v4 := ip.To4(); v4 != nil {
		return fmt.Sprintf("%d.%d.%d.0/24", v4[0], v4[1], v4[2])
	}
	// IPv6: keep the first three 16-bit groups, mask the rest.
	groups := strings.Split(ip.To16().String(), ":")
	if len(groups) < 3 {
		return ""
	}
	return strings.Join(groups[:3], ":") + "::/48"
}
