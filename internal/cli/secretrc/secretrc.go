// Package secretrc manages the per-worktree .secretrc file.
//
// .secretrc lives at the repository root (gitignored). It binds the
// current checkout to a project + default env, and optionally maps
// branch names to env names. The Task 8 context resolver consults
// this file before falling back to other signals.
package secretrc

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// FileName is the conventional filename. Operators can override the
// path via flag, but everything else assumes this.
const FileName = ".secretrc"

// Config is the on-disk shape. All fields are optional; an empty file
// is technically valid (operators may want only branch mappings, no
// project pin).
type Config struct {
	// Project is the comax-secrets project this worktree binds to.
	// `secret init` writes this; later commands read it.
	Project string `json:"project,omitempty"`
	// DefaultEnv is the env used when --env isn't given and no other
	// resolver signal matches.
	DefaultEnv string `json:"default_env,omitempty"`
	// Branches maps git branch names to env names. Lookups are exact;
	// no glob support in M1.
	Branches map[string]string `json:"branches,omitempty"`
	// Env, when set, pins this worktree to a specific env regardless
	// of branch. Highest-precedence file signal — used by worktrees
	// that should always pull, say, "local".
	Env string `json:"env,omitempty"`
}

// ErrNotFound is returned by Load when the file does not exist. Callers
// distinguish this from a real read error so they can fall through to
// the next precedence layer.
var ErrNotFound = errors.New("secretrc: not found")

// Load reads .secretrc from dir. Returns ErrNotFound when absent so
// callers can decide whether that's fatal.
func Load(dir string) (Config, error) {
	path := filepath.Join(dir, FileName)
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("%w: %s", ErrNotFound, path)
	}
	if err != nil {
		return Config{}, fmt.Errorf("read %q: %w", path, err)
	}
	var c Config
	if len(raw) == 0 {
		return c, nil
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return Config{}, fmt.Errorf("parse %q: %w", path, err)
	}
	return c, nil
}

// Save writes cfg to dir/.secretrc with mode 0644. The file is meant
// to be read by tooling, not protected like the credentials file, so
// we use a normal mode rather than 0600.
func Save(dir string, cfg Config) error {
	path := filepath.Join(dir, FileName)
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal secretrc: %w", err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("write %q: %w", path, err)
	}
	return nil
}
