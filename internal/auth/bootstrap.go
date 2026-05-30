package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/idenn207/comax-secrets/internal/store"
)

// ErrAlreadyBootstrapped is returned by Bootstrap when the server has
// already had its admin token issued. The HTTP layer maps this to 409.
var ErrAlreadyBootstrapped = errors.New("auth: already bootstrapped")

// BootstrapTokenName is the conventional name stamped onto the very
// first admin token. Operators rename or revoke it later via /tokens
// (post-M1); the conventional name keeps the audit log self-explanatory
// without an extra arg on /bootstrap.
const BootstrapTokenName = "bootstrap"

// BootstrapResult is what Bootstrap returns on success. Plaintext is
// shown to the operator exactly once and never persisted; Token is the
// row that landed in service_tokens.
type BootstrapResult struct {
	Plaintext string
	Token     store.ServiceToken
}

// Bootstrap mints the very first admin token, but only when the
// database has zero tokens. The whole operation is one SQLite
// transaction so the token row and the matching audit row commit
// together or not at all.
//
// Race-safety: TokenRepo.BootstrapIfEmpty performs the empty-check and
// the insert in a single conditional-insert statement
// (INSERT ... SELECT ... WHERE (SELECT COUNT(*)) = 0), and SQLite
// serialises writers; two simultaneous /bootstrap calls therefore
// cannot both succeed even if the application-level Count check would
// have raced.
//
// On success the returned plaintext is the exact string the operator
// must paste into `secret login --token`; it never leaves this call
// path again.
func Bootstrap(ctx context.Context, db *sql.DB) (BootstrapResult, error) {
	plain, err := GenerateToken()
	if err != nil {
		return BootstrapResult{}, err
	}
	hash := HashToken(plain)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("begin bootstrap tx: %w", err)
	}
	// Rollback is a no-op after a successful Commit, so the deferred
	// call is safe regardless of which path we take below.
	defer func() { _ = tx.Rollback() }()

	tok, created, err := store.NewTokenRepo(tx).BootstrapIfEmpty(ctx, BootstrapTokenName, hash)
	if err != nil {
		return BootstrapResult{}, err
	}
	if !created {
		return BootstrapResult{}, ErrAlreadyBootstrapped
	}

	if _, err := store.NewAuditRepo(tx).Append(ctx, &tok.ID, "auth.bootstrap", "token="+tok.Name, ""); err != nil {
		return BootstrapResult{}, fmt.Errorf("audit bootstrap: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return BootstrapResult{}, fmt.Errorf("commit bootstrap: %w", err)
	}
	return BootstrapResult{Plaintext: plain, Token: tok}, nil
}
