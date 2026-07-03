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
	"io/fs"
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
	"github.com/idenn207/comax-secrets/internal/server/dashboard"
	"github.com/idenn207/comax-secrets/internal/store"
	"github.com/idenn207/comax-secrets/internal/version"
	"github.com/idenn207/comax-secrets/internal/webhook"
)

// config is the resolved runtime configuration. Each field has a flag
// and an env fallback; flags win when both are set.
type config struct {
	listenAddr       string
	dbPath           string
	masterKeyPath    string
	autoGenKey       bool
	dashboardEnabled bool
	printVersion     bool
	healthCheck      bool
	healthURL        string
	webhookPoll      time.Duration
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

	spaFS, err := resolveDashboardFS(cfg, logger)
	if err != nil {
		return err
	}
	webhookPolicy, err := webhook.PolicyFromEnv()
	if err != nil {
		return fmt.Errorf("webhook policy: %w", err)
	}
	srv := server.NewServer(server.Options{
		DB:            db,
		Keys:          keys,
		Logger:        logger,
		SPAFS:         spaFS,
		WebhookPolicy: webhookPolicy,
	})
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

	// Hourly sweeper. cutoff = time.Now() (no grace) so a row becomes
	// eligible for deletion the moment expires_at slips into the past.
	// threat-model.md promises "expired/revoked rows are pruned hourly";
	// that promise is satisfied by *this* goroutine.
	go runPruneSweeper(ctx, store.NewSessionRepo(db), time.Hour, logger)

	// Webhook delivery worker: reclaims stale leases, atomically claims due
	// outbox rows, signs each payload with the webhook's sealed HMAC key, and
	// POSTs through the SSRF-hardened client. The same ctx cancellation that
	// stops the sweeper drives its graceful stop.
	webhookWorker := webhook.NewWorker(webhook.Options{
		DB:     db,
		Keys:   keys,
		Policy: webhookPolicy,
		Logger: logger,
	})
	go webhookWorker.Run(ctx, cfg.webhookPoll)

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
		listenAddr:       envOr("COMAX_LISTEN", ":8080"),
		dbPath:           envOr("COMAX_DB_PATH", "./data/secrets.db"),
		masterKeyPath:    envOr("COMAX_MASTER_KEY_FILE", "./keys/master.key"),
		autoGenKey:       envBool("COMAX_AUTO_GENERATE_KEY", true),
		dashboardEnabled: envBool("COMAX_DASHBOARD_ENABLED", true),
		webhookPoll:      envDurationOr("COMAX_WEBHOOK_POLL", webhook.DefaultPollInterval),
	}
	fs := flag.NewFlagSet("secret-server", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&cfg.listenAddr, "listen", cfg.listenAddr, "HTTP listen address (env: COMAX_LISTEN)")
	fs.StringVar(&cfg.dbPath, "db", cfg.dbPath, "SQLite database path (env: COMAX_DB_PATH)")
	fs.StringVar(&cfg.masterKeyPath, "master-key-file", cfg.masterKeyPath, "Path to master key file (env: COMAX_MASTER_KEY_FILE)")
	fs.BoolVar(&cfg.autoGenKey, "auto-generate-key", cfg.autoGenKey, "Generate the master key if missing (env: COMAX_AUTO_GENERATE_KEY)")
	fs.BoolVar(&cfg.dashboardEnabled, "dashboard-enabled", cfg.dashboardEnabled,
		"Serve the embedded dashboard SPA at / (env: COMAX_DASHBOARD_ENABLED). "+
			"Has no effect on /api or /healthz. Always off when the binary was built without -tags embed_dashboard.")
	fs.DurationVar(&cfg.webhookPoll, "webhook-poll-interval", cfg.webhookPoll,
		"How often the webhook delivery worker polls the outbox (env: COMAX_WEBHOOK_POLL)")
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

// envDurationOr parses a Go duration env var (e.g. "10s", "1m"); an empty or
// unparseable value defers to fallback.
func envDurationOr(name string, fallback time.Duration) time.Duration {
	v, ok := os.LookupEnv(name)
	if !ok || v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

// resolveDashboardFS returns the embedded SPA filesystem when both
// conditions hold: (1) the binary was built with -tags embed_dashboard
// so dashboard.Embedded() reports true, and (2) the operator did not
// disable the dashboard via --dashboard-enabled=false /
// COMAX_DASHBOARD_ENABLED=false.
//
// The two conditions are surfaced separately in the log line so the
// operator can tell why the dashboard is not responding — "built
// without the embed tag" and "disabled by config" are very different
// fixes.
func resolveDashboardFS(cfg config, logger *slog.Logger) (fs.FS, error) {
	if !cfg.dashboardEnabled {
		logger.Info("dashboard: disabled by config",
			slog.Bool("embedded", dashboard.Embedded()),
		)
		return nil, nil
	}
	if !dashboard.Embedded() {
		logger.Info("dashboard: dev mode, /api only (rebuild with -tags embed_dashboard to serve the SPA)")
		return nil, nil
	}
	spaFS, err := dashboard.FS()
	if err != nil {
		return nil, fmt.Errorf("dashboard fs: %w", err)
	}
	logger.Info("dashboard: enabled, serving embedded SPA at /")
	return spaFS, nil
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

// runPruneSweeper drives SessionRepo.Prune on a fixed interval until ctx
// is cancelled. cutoff is always time.Now() — the threat model promises
// "1-hour pruning of expired/revoked rows", and any grace window past
// expires_at would silently weaken that promise.
//
// Extracted out of run() so tests can drive it at millisecond intervals
// against an in-memory repo without booting the HTTP listener.
func runPruneSweeper(ctx context.Context, repo *store.SessionRepo, interval time.Duration, logger *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := repo.Prune(ctx, time.Now())
			if err != nil {
				// Don't tear the process down for a transient SQLite hiccup —
				// the next tick will retry. A persistent error will spam the
				// log at hourly cadence which is its own signal.
				logger.Warn("session prune failed", slog.String("err", err.Error()))
				continue
			}
			if n > 0 {
				logger.Info("sessions pruned", slog.Int64("rows", n))
			}
		}
	}
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
