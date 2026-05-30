package crypto

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// writeKeyFile creates a file at path containing the given bytes with the
// requested permission mode. Returns the absolute path for convenience.
func writeKeyFile(t *testing.T, dir string, name string, contents []byte, mode os.FileMode) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, contents, mode); err != nil {
		t.Fatalf("write key file: %v", err)
	}
	// os.WriteFile honours umask on Unix; chmod explicitly so we get the
	// exact bits we asked for regardless of umask.
	if err := os.Chmod(p, mode); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	return p
}

func TestFileKeyProvider_LoadValidKey(t *testing.T) {
	key := bytes.Repeat([]byte{0xab}, KeySize)
	path := writeKeyFile(t, t.TempDir(), "master.key", key, 0o600)

	p, err := NewFileKeyProvider(path)
	if err != nil {
		t.Fatalf("NewFileKeyProvider: %v", err)
	}
	got, err := p.Key(context.Background())
	if err != nil {
		t.Fatalf("Key: %v", err)
	}
	if !bytes.Equal(got, key) {
		t.Errorf("Key contents mismatch")
	}
}

func TestFileKeyProvider_MissingFile(t *testing.T) {
	_, err := NewFileKeyProvider(filepath.Join(t.TempDir(), "does-not-exist.key"))
	if err == nil {
		t.Fatal("expected error for missing file; got nil")
	}
}

func TestFileKeyProvider_RefusesInsecureModeOnUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission enforcement is Unix-only; Windows uses warn-and-continue")
	}
	key := bytes.Repeat([]byte{0xcd}, KeySize)
	path := writeKeyFile(t, t.TempDir(), "lax.key", key, 0o644)

	_, err := NewFileKeyProvider(path)
	if !errors.Is(err, ErrInsecureKeyFile) {
		t.Fatalf("err = %v; want ErrInsecureKeyFile", err)
	}
}

func TestFileKeyProvider_WarnAndContinueOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only path")
	}
	key := bytes.Repeat([]byte{0xef}, KeySize)
	// 0o644 would refuse on Unix; on Windows we expect a warning + success.
	path := writeKeyFile(t, t.TempDir(), "win.key", key, 0o644)

	var sink bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&sink, &slog.HandlerOptions{Level: slog.LevelWarn}))

	p, err := NewFileKeyProvider(path, WithLogger(logger))
	if err != nil {
		t.Fatalf("expected success on Windows; got %v", err)
	}
	if !bytes.Contains(sink.Bytes(), []byte("not enforced on Windows")) {
		t.Errorf("expected warning in log; got %q", sink.String())
	}
	if _, err := p.Key(context.Background()); err != nil {
		t.Errorf("Key: %v", err)
	}
}

func TestFileKeyProvider_WrongSize(t *testing.T) {
	// 16 bytes (AES-128 key size) is not acceptable here.
	short := bytes.Repeat([]byte{0x11}, 16)
	path := writeKeyFile(t, t.TempDir(), "short.key", short, 0o600)

	p, err := NewFileKeyProvider(path)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	_, err = p.Key(context.Background())
	if err == nil {
		t.Fatal("expected error for wrong-size key; got nil")
	}
}

func TestFileKeyProvider_UnreadableAfterConstruction(t *testing.T) {
	// Construct succeeds, then we delete the file so Key() fails on read.
	// This drives the os.ReadFile error branch.
	key := bytes.Repeat([]byte{0x22}, KeySize)
	path := writeKeyFile(t, t.TempDir(), "vanishing.key", key, 0o600)

	p, err := NewFileKeyProvider(path)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := p.Key(context.Background()); err == nil {
		t.Fatal("expected error after removing file; got nil")
	}
}

func TestFileKeyProvider_DefaultLoggerIsUsed(t *testing.T) {
	// Smoke test: construct without WithLogger and confirm the provider
	// still works. We don't try to capture slog.Default()'s output here;
	// the explicit-logger test above exercises the formatting.
	key := bytes.Repeat([]byte{0x33}, KeySize)
	path := writeKeyFile(t, t.TempDir(), "default.key", key, 0o600)

	p, err := NewFileKeyProvider(path)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	if _, err := p.Key(context.Background()); err != nil {
		t.Errorf("Key: %v", err)
	}
}
