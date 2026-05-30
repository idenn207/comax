package server

import (
	"context"

	"github.com/idenn207/comax-secrets/internal/secret"
)

// Resolver returns the fully-resolved snapshot of one environment's
// secrets — inheritance applied (inherits_from chain merged with child
// overrides) and ${{ env.KEY }} references expanded.
//
// The default implementation (wired by NewServer when Options.Resolver
// is nil) is *secret.Resolver, which reads from the same *sql.DB and
// KeyProvider the rest of the server uses. Tests can supply a stub that
// returns a hand-built snapshot or an error to exercise specific
// branches without seeding the database.
type Resolver interface {
	Resolve(ctx context.Context, projectID, envID int64) (secret.Snapshot, error)
}
