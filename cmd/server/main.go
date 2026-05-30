// Command secret-server is the Comax Secrets HTTP server binary.
//
// Boot sequence (Task 12 of M1):
//
//  1. Parse flags / env (no PII).
//  2. Ensure the master key file exists; auto-generate one with mode
//     0600 if missing. Refuse-to-boot if the existing file is wider
//     than 0600 on Unix (FileKeyProvider enforces this).
//  3. Open the SQLite database and apply embedded migrations.
//  4. If no service tokens exist, mint the bootstrap admin token and
//     print the plaintext to stdout exactly once so the operator can
//     copy it from `docker compose logs`.
//  5. Start the HTTP listener with timeouts. SIGINT/SIGTERM trigger a
//     graceful shutdown.
//
// Why log the bootstrap token to stdout instead of writing it to a
// file: the threat model assumes the operator owns the host, so logs
// are an acceptable one-time channel and avoid the "where did the
// installer drop the token?" question.
package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/idenn207/comax-secrets/internal/auth"
	"github.com/idenn207/comax-secrets/internal/crypto"
	"github.com/idenn207/comax-secrets/internal/server"
	"github.com/idenn207/comax-secrets/internal/store"
	"github.com/idenn207/comax-secrets/internal/version"
)

// config is the resolved runtime configuration. Each field has a flag
// and an env fallback; flags win when both are set.
type config struct {
	listenAddr    string
	dbPath        string
	masterKeyPath string
	autoGenKey    bool
	printVersion  bool
	healthCheck   bool
	healthURL     string
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "secret-server:", err)
		os.Exit(1)
	}
}

// run is the testable entrypoint. It accepts the slice of args minus
// argv[0] so tests can drive it without touching os.Args.
func run(args []string, stdout, stderr io.Writer) error {
	cfg, err := parseFlags(args, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if cfg.printVersion {
		_, err := fmt.Fprintln(stdout, "secret-server", version.String())
		return err
	}
	if cfg.healthCheck {
		return runHealthCheck(cfg.healthURL)
	}

	logger := slog.New(slog.NewJSONHandler(stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if err := ensureMasterKey(cfg, logger); err != nil {
		return err
	}
	keys, err := crypto.NewFileKeyProvider(cfg.masterKeyPath, crypto.WithLogger(logger))
	if err != nil {
		return fmt.Errorf("load master key: %w", err)
	}

	if err := ensureDataDir(cfg.dbPath); err != nil {
		return err
	}
	db, err := store.Open(cfg.dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer func() { _ = db.Close() }()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := store.Migrate(ctx, db); err != nil {
		return fmt.Errorf("migrate db: %w", err)
	}
	if err := autoBootstrap(ctx, db, stdout, logger); err != nil {
		return err
	}

	srv := server.NewServer(server.Options{DB: db, Keys: keys, Logger: logger})
	httpSrv := &http.Server{
		Addr:              cfg.listenAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	logger.Info("server starting",
		slog.String("addr", cfg.listenAddr),
		slog.String("db", cfg.dbPath),
		slog.String("master_key", cfg.masterKeyPath),
		slog.String("version", version.String()),
	)

	errCh := make(chan error, 1)
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("listen: %w", err)
		}
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}

// parseFlags resolves the runtime config. Env vars are read first
// (so they show up as defaults in --help), then flags override.
func parseFlags(args []string, stderr io.Writer) (config, error) {
	cfg := config{
		listenAddr:    envOr("COMAX_LISTEN", ":8080"),
		dbPath:        envOr("COMAX_DB_PATH", "./data/secrets.db"),
		masterKeyPath: envOr("COMAX_MASTER_KEY_FILE", "./keys/master.key"),
		autoGenKey:    envBool("COMAX_AUTO_GENERATE_KEY", true),
	}
	fs := flag.NewFlagSet("secret-server", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&cfg.listenAddr, "listen", cfg.listenAddr, "HTTP listen address (env: COMAX_LISTEN)")
	fs.StringVar(&cfg.dbPath, "db", cfg.dbPath, "SQLite database path (env: COMAX_DB_PATH)")
	fs.StringVar(&cfg.masterKeyPath, "master-key-file", cfg.masterKeyPath, "Path to master key file (env: COMAX_MASTER_KEY_FILE)")
	fs.BoolVar(&cfg.autoGenKey, "auto-generate-key", cfg.autoGenKey, "Generate the master key if missing (env: COMAX_AUTO_GENERATE_KEY)")
	fs.BoolVar(&cfg.printVersion, "version", false, "Print version and exit")
	fs.BoolVar(&cfg.healthCheck, "health", false, "Probe --health-url and exit 0 on HTTP 200")
	fs.StringVar(&cfg.healthURL, "health-url", envOr("COMAX_HEALTH_URL", "http://127.0.0.1:8080/healthz"),
		"URL probed by --health (env: COMAX_HEALTH_URL)")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	return cfg, nil
}

// runHealthCheck performs a single GET against url with a 2s timeout
// and returns nil on HTTP 200. This is the distroless-friendly
// alternative to bundling curl/wget into the image: the same binary
// acts as its own healthcheck client.
func runHealthCheck(url string) error {
	c := &http.Client{Timeout: 2 * time.Second}
	resp, err := c.Get(url)
	if err != nil {
		return fmt.Errorf("health probe: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health status %d", resp.StatusCode)
	}
	return nil
}

// envOr returns the env var value or the fallback when unset.
func envOr(name, fallback string) string {
	if v, ok := os.LookupEnv(name); ok && v != "" {
		return v
	}
	return fallback
}

// envBool parses a truthy env var; empty defers to fallback. Accepts
// "1", "true", "yes" (case-insensitive); everything else is false.
func envBool(name string, fallback bool) bool {
	v, ok := os.LookupEnv(name)
	if !ok || v == "" {
		return fallback
	}
	switch v {
	case "1", "true", "TRUE", "True", "yes", "YES":
		return true
	default:
		return false
	}
}

// ensureMasterKey creates the master key file with mode 0600 when it is
// missing and auto-generation is enabled. If the file exists the
// FileKeyProvider's own validation (in the caller) enforces 0600 on
// Unix — we don't second-guess it here.
func ensureMasterKey(cfg config, logger *slog.Logger) error {
	if _, err := os.Stat(cfg.masterKeyPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat master key %q: %w", cfg.masterKeyPath, err)
	}
	if !cfg.autoGenKey {
		return fmt.Errorf("master key file %q missing and --auto-generate-key=false", cfg.masterKeyPath)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.masterKeyPath), 0o700); err != nil {
		return fmt.Errorf("create key dir: %w", err)
	}
	raw := make([]byte, crypto.KeySize)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Errorf("generate master key: %w", err)
	}
	if err := os.WriteFile(cfg.masterKeyPath, raw, 0o600); err != nil {
		return fmt.Errorf("write master key: %w", err)
	}
	logger.Info("generated new master key",
		slog.String("path", cfg.masterKeyPath),
		slog.Int("bytes", crypto.KeySize),
	)
	return nil
}

// ensureDataDir creates the parent dir of dbPath so SQLite can open it.
// SQLite needs the directory to exist; the file itself is created on
// first open.
func ensureDataDir(dbPath string) error {
	dir := filepath.Dir(dbPath)
	if dir == "" || dir == "." {
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	return nil
}

// autoBootstrap mints the first admin token if the database is empty
// and writes the plaintext to stdout. Operators must capture this from
// the very first boot's logs (`docker compose logs secret-server`); it
// is never re-emitted.
func autoBootstrap(ctx context.Context, db *sql.DB, stdout io.Writer, logger *slog.Logger) error {
	n, err := store.NewTokenRepo(db).Count(ctx)
	if err != nil {
		return fmt.Errorf("count tokens: %w", err)
	}
	if n > 0 {
		return nil
	}
	res, err := auth.Bootstrap(ctx, db)
	if err != nil {
		if errors.Is(err, auth.ErrAlreadyBootstrapped) {
			return nil
		}
		return fmt.Errorf("bootstrap: %w", err)
	}
	fmt.Fprintln(stdout, "==============================================================")
	fmt.Fprintln(stdout, "Comax Secrets: bootstrap admin token (shown ONCE):")
	fmt.Fprintln(stdout, "    "+res.Plaintext)
	fmt.Fprintln(stdout, "Save it via: secret login --server <url> --token <above>")
	fmt.Fprintln(stdout, "==============================================================")
	logger.Info("bootstrap admin token issued",
		slog.String("name", res.Token.Name),
		slog.Int64("token_id", res.Token.ID),
	)
	return nil
}
