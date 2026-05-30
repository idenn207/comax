package secret

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/idenn207/comax-secrets/internal/crypto"
	"github.com/idenn207/comax-secrets/internal/store"
)

// staticKey is a deterministic master key for tests.
type staticKey []byte

func (k staticKey) Key(_ context.Context) ([]byte, error) { return k, nil }

// brokenKey always errors. Drives the load-master-key branch in Resolve.
type brokenKey struct{}

func (brokenKey) Key(_ context.Context) ([]byte, error) {
	return nil, errors.New("kms down")
}

// testFixture wires up a fresh DB + project + envs + master key. The
// returned helpers (putSecret, putEnv) hide the store-layer boilerplate.
type testFixture struct {
	db        *sql.DB
	masterKey []byte
	resolver  *Resolver
	project   store.Project
}

func newFixture(t *testing.T) *testFixture {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "resolver.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	key := make([]byte, crypto.KeySize)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand: %v", err)
	}
	p, err := store.NewProjectRepo(db).Create(context.Background(), "app")
	if err != nil {
		t.Fatalf("seed project: %v", err)
	}
	return &testFixture{
		db:        db,
		masterKey: key,
		resolver:  NewResolver(db, staticKey(key)),
		project:   p,
	}
}

func (f *testFixture) putEnv(t *testing.T, name, inherits string) store.Environment {
	t.Helper()
	e, err := store.NewEnvRepo(f.db).Create(context.Background(), f.project.ID, name, inherits)
	if err != nil {
		t.Fatalf("seed env %q: %v", name, err)
	}
	return e
}

func (f *testFixture) putSecret(t *testing.T, envID int64, key, value string) {
	t.Helper()
	ct, err := crypto.Seal(f.masterKey, []byte(value))
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if _, err := store.NewSecretRepo(f.db).Upsert(context.Background(), envID, key, ct); err != nil {
		t.Fatalf("seed secret %q=%q: %v", key, value, err)
	}
}

func TestResolver_NoReferencesNoInheritance(t *testing.T) {
	f := newFixture(t)
	dev := f.putEnv(t, "dev", "")
	f.putSecret(t, dev.ID, "DB_HOST", "localhost")
	f.putSecret(t, dev.ID, "DB_PORT", "5432")

	snap, err := f.resolver.Resolve(context.Background(), f.project.ID, dev.ID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(snap) != 2 {
		t.Fatalf("snap size = %d; want 2", len(snap))
	}
	if snap["DB_HOST"].Value != "localhost" {
		t.Errorf("DB_HOST = %q; want localhost", snap["DB_HOST"].Value)
	}
	if snap["DB_PORT"].Version != 1 {
		t.Errorf("DB_PORT.Version = %d; want 1", snap["DB_PORT"].Version)
	}
}

func TestResolver_Inheritance_ChildOverridesParent(t *testing.T) {
	f := newFixture(t)
	shared := f.putEnv(t, "shared", "")
	dev := f.putEnv(t, "dev", "shared")

	f.putSecret(t, shared.ID, "DB_HOST", "shared-host")
	f.putSecret(t, shared.ID, "API_KEY", "shared-key")
	f.putSecret(t, dev.ID, "DB_HOST", "dev-host") // override

	snap, err := f.resolver.Resolve(context.Background(), f.project.ID, dev.ID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if snap["DB_HOST"].Value != "dev-host" {
		t.Errorf("DB_HOST = %q; want dev-host (child wins)", snap["DB_HOST"].Value)
	}
	if snap["API_KEY"].Value != "shared-key" {
		t.Errorf("API_KEY = %q; want shared-key (inherited)", snap["API_KEY"].Value)
	}
	if len(snap) != 2 {
		t.Errorf("snap size = %d; want 2 (one inherited + one overridden)", len(snap))
	}
}

func TestResolver_Inheritance_TwoLevelChain(t *testing.T) {
	f := newFixture(t)
	base := f.putEnv(t, "base", "")
	shared := f.putEnv(t, "shared", "base")
	dev := f.putEnv(t, "dev", "shared")

	f.putSecret(t, base.ID, "LEVEL", "from-base")
	f.putSecret(t, shared.ID, "LEVEL", "from-shared")
	f.putSecret(t, dev.ID, "OWN", "dev-only")

	snap, err := f.resolver.Resolve(context.Background(), f.project.ID, dev.ID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if snap["LEVEL"].Value != "from-shared" {
		t.Errorf("LEVEL = %q; want from-shared (deeper child wins)", snap["LEVEL"].Value)
	}
	if snap["OWN"].Value != "dev-only" {
		t.Errorf("OWN = %q; want dev-only", snap["OWN"].Value)
	}
}

func TestResolver_References_SimpleExpansion(t *testing.T) {
	f := newFixture(t)
	shared := f.putEnv(t, "shared", "")
	dev := f.putEnv(t, "dev", "")

	f.putSecret(t, shared.ID, "DB_HOST", "db.example.com")
	f.putSecret(t, dev.ID, "DB_URL", "postgres://${{ shared.DB_HOST }}:5432/app")

	snap, err := f.resolver.Resolve(context.Background(), f.project.ID, dev.ID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := "postgres://db.example.com:5432/app"
	if snap["DB_URL"].Value != want {
		t.Errorf("DB_URL = %q; want %q", snap["DB_URL"].Value, want)
	}
}

func TestResolver_References_MultipleInOneValue(t *testing.T) {
	f := newFixture(t)
	shared := f.putEnv(t, "shared", "")
	dev := f.putEnv(t, "dev", "")

	f.putSecret(t, shared.ID, "USER", "alice")
	f.putSecret(t, shared.ID, "HOST", "h.local")
	f.putSecret(t, dev.ID, "DSN", "${{ shared.USER }}@${{ shared.HOST }}")

	snap, err := f.resolver.Resolve(context.Background(), f.project.ID, dev.ID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if snap["DSN"].Value != "alice@h.local" {
		t.Errorf("DSN = %q; want alice@h.local", snap["DSN"].Value)
	}
}

func TestResolver_References_UnknownEnvIs400(t *testing.T) {
	f := newFixture(t)
	dev := f.putEnv(t, "dev", "")
	f.putSecret(t, dev.ID, "X", "${{ nope.K }}")

	_, err := f.resolver.Resolve(context.Background(), f.project.ID, dev.ID)
	if !errors.Is(err, ErrUnknownReference) {
		t.Errorf("err = %v; want wraps ErrUnknownReference", err)
	}
}

func TestResolver_References_UnknownKeyIs400(t *testing.T) {
	f := newFixture(t)
	shared := f.putEnv(t, "shared", "")
	dev := f.putEnv(t, "dev", "")
	f.putSecret(t, shared.ID, "DEFINED", "yes")
	f.putSecret(t, dev.ID, "X", "${{ shared.MISSING }}")

	_, err := f.resolver.Resolve(context.Background(), f.project.ID, dev.ID)
	if !errors.Is(err, ErrUnknownReference) {
		t.Errorf("err = %v; want wraps ErrUnknownReference", err)
	}
}

func TestResolver_Cycle_Inheritance(t *testing.T) {
	f := newFixture(t)
	// dev inherits from shared; shared inherits from dev (cycle).
	dev := f.putEnv(t, "dev", "shared")
	shared := f.putEnv(t, "shared", "dev")
	// Add a secret so the resolver actually descends.
	f.putSecret(t, dev.ID, "X", "1")
	_ = shared

	_, err := f.resolver.Resolve(context.Background(), f.project.ID, dev.ID)
	var cyc *CycleError
	if !errors.As(err, &cyc) {
		t.Fatalf("err = %v; want *CycleError", err)
	}
	if cyc.Kind != "inheritance" {
		t.Errorf("cyc.Kind = %q; want inheritance", cyc.Kind)
	}
	if len(cyc.Path) < 2 {
		t.Errorf("cyc.Path = %v; want at least 2 entries", cyc.Path)
	}
}

func TestResolver_KeyLoadFailureSurfaces(t *testing.T) {
	f := newFixture(t)
	f.resolver = NewResolver(f.db, brokenKey{})
	dev := f.putEnv(t, "dev", "")
	f.putSecret(t, dev.ID, "X", "1") // seed using f.masterKey, not the broken one

	_, err := f.resolver.Resolve(context.Background(), f.project.ID, dev.ID)
	if err == nil {
		t.Fatal("Resolve returned nil error; want load-key failure")
	}
}

func TestResolver_NoSecretsReturnsEmptySnapshot(t *testing.T) {
	f := newFixture(t)
	dev := f.putEnv(t, "dev", "")
	snap, err := f.resolver.Resolve(context.Background(), f.project.ID, dev.ID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(snap) != 0 {
		t.Errorf("snap size = %d; want 0", len(snap))
	}
}
