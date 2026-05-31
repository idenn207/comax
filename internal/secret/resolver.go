package secret

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/idenn207/comax-secrets/internal/crypto"
	"github.com/idenn207/comax-secrets/internal/store"
)

// ResolvedSecret is one fully-resolved secret in an env snapshot: the
// final plaintext after inheritance overlay and reference expansion,
// plus the metadata operators care about.
//
// SecretID identifies the underlying secrets row that contributed this
// value. For an env's own keys this is the row in the same env; for
// inherited keys it is the parent env's row. The dashboard uses this to
// correlate a resolved entry with its version history without relying
// on (env, version) heuristics that collide when two keys share the
// same current version number.
type ResolvedSecret struct {
	SecretID  int64
	Value     string
	Version   int64
	UpdatedAt time.Time
}

// Snapshot is the resolved view of one env's secrets, keyed by secret
// key. Order is not preserved; callers that need ordering sort the
// keys themselves.
type Snapshot map[string]ResolvedSecret

// Resolver computes inherited + reference-expanded snapshots of envs.
//
// Stateless across calls. Internal caches live on the per-call
// resolution context (resolveCtx) so concurrent callers never see each
// other's intermediate state. The trade-off is some duplicated work
// when two simultaneous requests resolve the same env, which is
// acceptable for a single-operator self-host deployment.
type Resolver struct {
	db   *sql.DB
	keys crypto.KeyProvider
}

// NewResolver wires the dependencies.
func NewResolver(db *sql.DB, keys crypto.KeyProvider) *Resolver {
	return &Resolver{db: db, keys: keys}
}

// CycleError signals either an inheritance cycle or a reference cycle.
// Path lists the chain that closed the loop, e.g. ["dev", "shared", "dev"]
// for inheritance or ["dev.A", "shared.B", "dev.A"] for references.
// The HTTP layer maps this to 400 bad_request.
type CycleError struct {
	Kind string // "inheritance" or "reference"
	Path []string
}

func (e *CycleError) Error() string {
	return fmt.Sprintf("%s cycle: %s", e.Kind, strings.Join(e.Path, " -> "))
}

// ErrUnknownReference signals that a ${{ env.KEY }} reference points to
// a missing env or key. We don't conflate with store.ErrNotFound because
// the HTTP layer reports this as 400 (operator's value), not 404.
var ErrUnknownReference = errors.New("secret: unknown reference target")

// Resolve returns the fully-resolved snapshot of (projectID, envID).
func (r *Resolver) Resolve(ctx context.Context, projectID, envID int64) (Snapshot, error) {
	key, err := r.keys.Key(ctx)
	if err != nil {
		return nil, fmt.Errorf("load master key: %w", err)
	}

	rc := &resolveCtx{
		ctx:         ctx,
		db:          r.db,
		projectID:   projectID,
		key:         key,
		envCache:    map[int64]Snapshot{},
		envByNameID: map[string]int64{},
	}
	return rc.snapshot(envID, nil)
}

// resolveCtx carries per-call state through the recursive descent.
// Caches make repeated lookups (e.g. one env referenced from many
// values) constant-time after the first hit.
type resolveCtx struct {
	ctx       context.Context
	db        *sql.DB
	projectID int64
	key       []byte

	// envCache memoises already-built snapshots so a fan-in (many refs
	// to the same env) doesn't rebuild N times.
	envCache map[int64]Snapshot
	// envByNameID memoises name → ID lookups so the same parent env
	// referenced by multiple children only hits the DB once.
	envByNameID map[string]int64
}

// snapshot computes the resolved view of envID, applying inheritance
// then reference expansion. visited is the chain of env IDs already
// being computed in this call stack; entering one twice is an
// inheritance cycle.
func (rc *resolveCtx) snapshot(envID int64, visited []int64) (Snapshot, error) {
	if cached, ok := rc.envCache[envID]; ok {
		return cached, nil
	}
	for _, v := range visited {
		if v == envID {
			return nil, &CycleError{
				Kind: "inheritance",
				Path: rc.envChainNames(append(visited, envID)),
			}
		}
	}
	nextVisited := append(visited, envID)

	env, err := store.NewEnvRepo(rc.db).ByID(rc.ctx, envID)
	if err != nil {
		return nil, fmt.Errorf("load env id=%d: %w", envID, err)
	}

	// 1. Start from the parent snapshot when inherits_from is set.
	merged := Snapshot{}
	if env.InheritsFrom != "" {
		parentID, err := rc.resolveEnvName(env.InheritsFrom)
		if err != nil {
			return nil, fmt.Errorf("env %q inherits_from %q: %w", env.Name, env.InheritsFrom, err)
		}
		parent, err := rc.snapshot(parentID, nextVisited)
		if err != nil {
			return nil, err
		}
		// Copy: callers must not mutate cached entries.
		for k, v := range parent {
			merged[k] = v
		}
	}

	// 2. Overlay this env's own secrets — child wins.
	secrets, err := store.NewSecretRepo(rc.db).ListByEnv(rc.ctx, envID)
	if err != nil {
		return nil, fmt.Errorf("list secrets for env=%d: %w", envID, err)
	}
	for _, s := range secrets {
		plain, err := crypto.Open(rc.key, s.Ciphertext)
		if err != nil {
			return nil, fmt.Errorf("decrypt %s in env=%d: %w", s.Key, envID, err)
		}
		merged[s.Key] = ResolvedSecret{
			SecretID:  s.ID,
			Value:     string(plain),
			Version:   s.Version,
			UpdatedAt: s.UpdatedAt,
		}
	}

	// 3. Cache before expansion so cross-references can read this env's
	// pre-expansion state. Expansion writes back into the same map.
	rc.envCache[envID] = merged

	// 4. Expand ${{ }} references inside every value.
	for k, v := range merged {
		expanded, err := rc.expand(envID, k, v.Value, nil)
		if err != nil {
			return nil, err
		}
		v.Value = expanded
		merged[k] = v
	}
	return merged, nil
}

// expand replaces every ${{ env.KEY }} reference in value with the
// resolved plaintext of that key in that env. visited is the chain of
// (envID, key) pairs already being expanded; entering one twice is a
// reference cycle.
func (rc *resolveCtx) expand(envID int64, key string, value string, visited []refPos) (string, error) {
	refs := findReferences(value)
	if len(refs) == 0 {
		return value, nil
	}
	for _, v := range visited {
		if v.envID == envID && v.key == key {
			return "", &CycleError{
				Kind: "reference",
				Path: rc.refChainNames(append(visited, refPos{envID: envID, key: key})),
			}
		}
	}
	nextVisited := append(visited, refPos{envID: envID, key: key})

	// Walk refs in reverse so byte offsets stay valid as we splice.
	out := value
	for i := len(refs) - 1; i >= 0; i-- {
		ref := refs[i]
		targetEnvID, err := rc.resolveEnvName(ref.envName)
		if err != nil {
			return "", fmt.Errorf("reference ${{ %s.%s }}: %w", ref.envName, ref.keyName, err)
		}
		targetSnap, err := rc.snapshot(targetEnvID, nil)
		if err != nil {
			return "", err
		}
		targetSecret, ok := targetSnap[ref.keyName]
		if !ok {
			return "", fmt.Errorf("reference ${{ %s.%s }}: %w", ref.envName, ref.keyName, ErrUnknownReference)
		}
		// Recurse into the target's value too: it might still contain
		// nested refs (the targetSnap entry is the post-expansion
		// cached value, so this is almost always a no-op — but it's
		// also our cycle-detection hook).
		expanded, err := rc.expand(targetEnvID, ref.keyName, targetSecret.Value, nextVisited)
		if err != nil {
			return "", err
		}
		out = out[:ref.start] + expanded + out[ref.end:]
	}
	return out, nil
}

// resolveEnvName turns an env name into its ID within the current
// project, with a per-call cache.
func (rc *resolveCtx) resolveEnvName(name string) (int64, error) {
	if id, ok := rc.envByNameID[name]; ok {
		return id, nil
	}
	env, err := store.NewEnvRepo(rc.db).ByName(rc.ctx, rc.projectID, name)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return 0, fmt.Errorf("env %q: %w", name, ErrUnknownReference)
		}
		return 0, err
	}
	rc.envByNameID[name] = env.ID
	return env.ID, nil
}

// refPos is one node in the reference-cycle visited list.
type refPos struct {
	envID int64
	key   string
}

// envChainNames best-effort-resolves env IDs to names for error
// messages. If a lookup fails (impossible in practice — the IDs came
// from the chain we just walked) we fall back to "id=N".
func (rc *resolveCtx) envChainNames(ids []int64) []string {
	envRepo := store.NewEnvRepo(rc.db)
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		e, err := envRepo.ByID(rc.ctx, id)
		if err != nil {
			out = append(out, fmt.Sprintf("id=%d", id))
			continue
		}
		out = append(out, e.Name)
	}
	return out
}

// refChainNames turns reference-visited positions into "env.key" strings.
func (rc *resolveCtx) refChainNames(positions []refPos) []string {
	out := make([]string, 0, len(positions))
	for _, p := range positions {
		envName := fmt.Sprintf("id=%d", p.envID)
		if e, err := store.NewEnvRepo(rc.db).ByID(rc.ctx, p.envID); err == nil {
			envName = e.Name
		}
		out = append(out, envName+"."+p.key)
	}
	return out
}
