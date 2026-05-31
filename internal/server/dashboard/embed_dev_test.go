//go:build !embed_dashboard

package dashboard

import "testing"

// TestDevMode_EmbeddedFalse pins the dev-mode contract: without
// -tags embed_dashboard the package must not pretend a dashboard is
// present. cmd/server's startup log line + the SPA handler both rely
// on this.
func TestDevMode_EmbeddedFalse(t *testing.T) {
	if Embedded() {
		t.Fatal("Embedded() = true in dev-mode build; expected false")
	}
}

// TestDevMode_FSReturnsNil pins the (nil, nil) contract. server.NewServer
// treats nil SPAFS as "no dashboard"; if this ever returns a non-nil FS
// in dev mode the SPA handler would try to read from it and surprise
// contributors who never ran `make dashboard`.
func TestDevMode_FSReturnsNil(t *testing.T) {
	fs, err := FS()
	if err != nil {
		t.Fatalf("FS error: %v", err)
	}
	if fs != nil {
		t.Fatalf("FS = %T; expected nil in dev mode", fs)
	}
}
