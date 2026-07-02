package crypto

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime"
)

// KeyProvider supplies the master key used by Seal/Open. It is the seam
// at which file-on-disk, OS keyring, and cloud-KMS implementations all
// plug in. Milestone 1 ships only FileKeyProvider; later milestones add
// the others without touching the call sites in server/handlers.
//
// Key must be safe for repeated calls; implementations may cache.
type KeyProvider interface {
	Key(ctx context.Context) ([]byte, error)
}

// ErrInsecureKeyFile is returned by NewFileKeyProvider when the file's
// Unix permissions are wider than 0600. Operators relying on this error
// have a misconfigured deployment and must tighten the file before
// retrying.
var ErrInsecureKeyFile = errors.New("crypto: insecure master key file")

// FileKeyProvider loads a 32-byte AES-256 master key from a file on
// disk.
//
// On Unix the file MUST be mode 0600 (owner read/write only) — anything
// wider is treated as a misconfiguration and the constructor refuses,
// returning ErrInsecureKeyFile. This implements the plan's
// "refuse-to-boot" rule for the PRD's #1 named risk.
//
// On Windows the same check is impractical (POSIX-mode bits don't
// reflect the real ACL) so the constructor logs a warning and continues.
// Operators running on Windows are expected to lock the file via NTFS
// ACLs separately; the threat-model doc spells this out.
type FileKeyProvider struct {
	path   string
	logger *slog.Logger
}

// FileKeyProviderOption configures a FileKeyProvider.
type FileKeyProviderOption func(*FileKeyProvider)

// WithLogger overrides the slog logger used for the Windows-only
// permission warning. Default: slog.Default().
func WithLogger(l *slog.Logger) FileKeyProviderOption {
	return func(p *FileKeyProvider) { p.logger = l }
}

// NewFileKeyProvider validates the file's permissions and returns a
// provider that lazy-loads on each Key() call.
//
// We intentionally do NOT cache the key in memory at construction time:
// loading is cheap (32 bytes), and forcing a fresh read each call means
// operator-triggered key rotation (replace the file, restart? — no,
// even without restart) can take effect on the next request without a
// restart-only model.
func NewFileKeyProvider(path string, opts ...FileKeyProviderOption) (*FileKeyProvider, error) {
	p := &FileKeyProvider{path: path, logger: slog.Default()}
	for _, opt := range opts {
		opt(p)
	}

	st, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat key file %q: %w", path, err)
	}
	if err := checkKeyFileMode(st, path, p.logger); err != nil {
		return nil, err
	}
	return p, nil
}

// Key reads and returns the master key. Returns an error if the file is
// missing, unreadable, or not exactly KeySize bytes.
func (p *FileKeyProvider) Key(_ context.Context) ([]byte, error) {
	raw, err := os.ReadFile(p.path)
	if err != nil {
		return nil, fmt.Errorf("read key file %q: %w", p.path, err)
	}
	if len(raw) != KeySize {
		return nil, fmt.Errorf("key file %q: %d bytes; want %d", p.path, len(raw), KeySize)
	}
	return raw, nil
}

// checkKeyFileMode enforces the 0o600 rule on Unix and warns on Windows.
// Extracted so tests can drive both branches deterministically.
func checkKeyFileMode(st os.FileInfo, path string, logger *slog.Logger) error {
	if runtime.GOOS == "windows" {
		logger.Warn(
			"master key file permissions not enforced on Windows; protect via NTFS ACLs",
			slog.String("path", path),
		)
		return nil
	}
	if mode := st.Mode().Perm(); mode != 0o600 {
		return fmt.Errorf("%w: %q has mode %#o, want 0600", ErrInsecureKeyFile, path, mode)
	}
	return nil
}
