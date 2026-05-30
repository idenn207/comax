// Package credentials manages the CLI's persistent ~/.config/comax
// credentials file. The file holds the server URL and bearer token so
// every subsequent `secret <subcommand>` doesn't need them on the
// command line.
//
// Security: the file is written 0600 on Unix. On Windows, POSIX-mode
// bits don't reflect NTFS ACLs so we don't enforce them; operators
// running on Windows are documented to lock the file via ACL separately.
package credentials

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Credentials is the on-disk shape.
type Credentials struct {
	Server string `json:"server"`
	Token  string `json:"token"`
}

// ErrNotFound is returned by Load when the file does not exist. CLI
// commands map this to "run `secret login` first" instead of a stat
// error.
var ErrNotFound = errors.New("credentials: not found")

// Path returns the default credentials path:
//
//   - Unix: $XDG_CONFIG_HOME/comax/credentials.json, falling back to
//     ~/.config/comax/credentials.json.
//   - Windows: %AppData%\comax\credentials.json.
//
// os.UserConfigDir already implements the right per-OS lookup so we
// just append the comax/credentials.json suffix.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user config dir: %w", err)
	}
	return filepath.Join(dir, "comax", "credentials.json"), nil
}

// Load reads the credentials file at the default Path(). Returns
// ErrNotFound when the file is missing.
func Load() (Credentials, error) {
	path, err := Path()
	if err != nil {
		return Credentials{}, err
	}
	return LoadFrom(path)
}

// LoadFrom is the explicit-path variant for tests and `--credentials`
// flag callers.
func LoadFrom(path string) (Credentials, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Credentials{}, fmt.Errorf("%w: %s", ErrNotFound, path)
	}
	if err != nil {
		return Credentials{}, fmt.Errorf("read %q: %w", path, err)
	}
	var c Credentials
	if err := json.Unmarshal(raw, &c); err != nil {
		return Credentials{}, fmt.Errorf("parse %q: %w", path, err)
	}
	if c.Server == "" || c.Token == "" {
		return Credentials{}, fmt.Errorf("parse %q: server and token are both required", path)
	}
	return c, nil
}

// Save writes creds to the default Path() with mode 0600 on Unix.
func Save(creds Credentials) error {
	path, err := Path()
	if err != nil {
		return err
	}
	return SaveTo(path, creds)
}

// SaveTo is the explicit-path variant. Creates the enclosing dir at
// mode 0700 if absent.
//
// We write to a tempfile in the same dir and rename to make the write
// atomic — a crash mid-write leaves the prior contents intact rather
// than a truncated file.
func SaveTo(path string, creds Credentials) error {
	if creds.Server == "" || creds.Token == "" {
		return errors.New("credentials: server and token are required")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir %q: %w", dir, err)
	}

	raw, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}
	raw = append(raw, '\n')

	tmp, err := os.CreateTemp(dir, "credentials-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	// On Unix, CreateTemp creates with mode 0600 by default. Belt-and-
	// braces: explicit Chmod so future Go versions can't drift.
	if runtime.GOOS != "windows" {
		if err := tmp.Chmod(0o600); err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
			return fmt.Errorf("chmod temp: %w", err)
		}
	}
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("close temp: %w", err)
	}
	// Windows os.Rename fails if the target exists; remove first.
	if runtime.GOOS == "windows" {
		_ = os.Remove(path)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("rename %q -> %q: %w", tmp.Name(), path, err)
	}
	return nil
}
