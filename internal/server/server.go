package server

import (
	"database/sql"
	"io"
	"log/slog"

	"github.com/idenn207/comax-secrets/internal/crypto"
	"github.com/idenn207/comax-secrets/internal/secret"
)

// Server holds the dependencies the HTTP handlers need. Construct via
// NewServer; expose the assembled handler via Handler().
//
// Design notes:
//
//   - The *sql.DB is the source of truth for transaction scope; the
//     server never caches repos because they are thin wrappers over
//     a DBTX and would otherwise pin a specific *sql.DB|*sql.Tx.
//   - KeyProvider is an interface so future KMS/keyring impls plug in
//     without touching this struct. M1 wires FileKeyProvider.
//   - Resolver is the seam for inline ${{ env.KEY }} expansion. M1 Task 5
//     wires noopResolver; Task 6 replaces it.
//   - Logger is slog so structured JSON propagates through middleware,
//     handlers, and downstream layers consistently.
type Server struct {
	db       *sql.DB
	keys     crypto.KeyProvider
	resolver Resolver
	logger   *slog.Logger
}

// Options configures NewServer. All fields are optional except DB and
// Keys; defaults are applied for the rest.
type Options struct {
	DB       *sql.DB
	Keys     crypto.KeyProvider
	Resolver Resolver
	Logger   *slog.Logger
}

// NewServer wires the dependencies. It does not start a network
// listener; callers do that themselves so they can swap http.Server
// configuration (timeouts, TLS) without this package growing flags.
//
// When Options.Resolver is nil we construct the production resolver
// (*secret.Resolver) over Options.DB and Options.Keys. Tests that want
// a stub supply it explicitly; the resulting Server never touches the
// crypto layer directly — all decryption flows through the resolver.
func NewServer(opts Options) *Server {
	if opts.Logger == nil {
		opts.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if opts.Resolver == nil {
		opts.Resolver = secret.NewResolver(opts.DB, opts.Keys)
	}
	return &Server{
		db:       opts.DB,
		keys:     opts.Keys,
		resolver: opts.Resolver,
		logger:   opts.Logger,
	}
}
