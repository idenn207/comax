package server

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/idenn207/comax-secrets/internal/crypto"
	"github.com/idenn207/comax-secrets/internal/secret"
	"github.com/idenn207/comax-secrets/internal/store"
)

// brokenKey is a KeyProvider that always errors. Used to drive the
// "key unavailable" branch in secrets handlers without monkey-patching
// FileKeyProvider.
type brokenKey struct{}

func (brokenKey) Key(_ context.Context) ([]byte, error) { return nil, errors.New("kms down") }

// cycleResolver always fails Resolve with a fake CycleError. Used to
// drive the bad_reference branch in handleListSecrets / handleGetSecret
// without seeding a real cycle in the database.
type cycleResolver struct{}

func (cycleResolver) Resolve(_ context.Context, _, _ int64) (secret.Snapshot, error) {
	return nil, &secret.CycleError{Kind: "reference", Path: []string{"dev.X", "shared.X", "dev.X"}}
}

// rawDo bypasses the testServer.do JSON-shaping path so we can send
// genuinely broken bodies. Bearer is auto-attached if non-empty.
func rawDo(t *testing.T, ts *testServer, method, path, body string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, ts.srv.URL+path, bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if ts.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+ts.bearer)
	}
	resp, err := ts.srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	buf := make([]byte, 1024)
	n, _ := resp.Body.Read(buf)
	return resp.StatusCode, buf[:n]
}

func TestBadJSON_PostProject(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	status, _ := rawDo(t, ts, http.MethodPost, "/api/v1/projects", "{not json")
	if status != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", status)
	}
}

func TestBadJSON_PostEnv(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "comax"})
	status, _ := rawDo(t, ts, http.MethodPost, "/api/v1/projects/comax/envs", "{")
	if status != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", status)
	}
}

func TestBadJSON_PutSecret(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "comax"})
	mustCreate(t, ts, "/api/v1/projects/comax/envs", map[string]string{"name": "dev"})
	status, _ := rawDo(t, ts, http.MethodPut, "/api/v1/projects/comax/envs/dev/secrets/K", "{")
	if status != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", status)
	}
}

func TestInvalidEnvNameInURL(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "comax"})
	// URL-encoded slash inside the env segment is the easiest way to
	// hit validateName's forbidden-character branch through the router.
	status, _ := ts.do(t, http.MethodGet, "/api/v1/projects/comax/envs/bad%20name/secrets", nil)
	if status != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", status)
	}
}

func TestInvalidKeyNameInURL(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "comax"})
	mustCreate(t, ts, "/api/v1/projects/comax/envs", map[string]string{"name": "dev"})
	status, _ := rawDo(t, ts, http.MethodPut,
		"/api/v1/projects/comax/envs/dev/secrets/bad%20key", `{"value":"v"}`)
	if status != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", status)
	}
}

func TestRejectInvalidEnvCreateName(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "comax"})
	status, _ := ts.do(t, http.MethodPost, "/api/v1/projects/comax/envs",
		map[string]string{"name": "ok", "inherits_from": "bad name"})
	if status != http.StatusBadRequest {
		t.Errorf("status = %d; want 400 on inherits_from with space", status)
	}
}

func TestUnknownRoute(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	status, _ := ts.do(t, http.MethodGet, "/api/v1/no-such-thing", nil)
	if status != http.StatusNotFound {
		t.Errorf("status = %d; want 404", status)
	}
}

// newTestServerWith returns a fresh testServer constructed with
// custom Options (e.g. a broken key provider). Mirrors newTestServer
// but parameterises the bits the error-path tests need to swap.
func newTestServerWith(t *testing.T, mutate func(*Options)) *testServer {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "srv.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatalf("store.Migrate: %v", err)
	}

	keyBytes := make([]byte, crypto.KeySize)
	opts := Options{DB: db, Keys: staticKey(keyBytes)}
	if mutate != nil {
		mutate(&opts)
	}
	srv := httptest.NewServer(NewServer(opts).Handler())
	t.Cleanup(srv.Close)
	return &testServer{srv: srv, db: db}
}

func TestSecretsList_KeyProviderFailure(t *testing.T) {
	ts := newTestServerWith(t, func(o *Options) {
		// Bootstrap doesn't need crypto; we install brokenKey after
		// the project+env are seeded by re-creating the server. Simpler
		// path: use brokenKey from the start, but bootstrap path
		// doesn't hit Keys so it works.
		o.Keys = brokenKey{}
	})
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "comax"})
	mustCreate(t, ts, "/api/v1/projects/comax/envs", map[string]string{"name": "dev"})

	// PUT will fail at the encrypt step.
	status, _ := ts.do(t, http.MethodPut, "/api/v1/projects/comax/envs/dev/secrets/K",
		map[string]string{"value": "v"})
	if status != http.StatusInternalServerError {
		t.Errorf("PUT status = %d; want 500", status)
	}

	// List with broken key also fails. We need *something* to decrypt
	// for ListSecrets to hit the key branch, but PUT already failed —
	// the empty list still returns 200 since there is nothing to
	// decrypt, so this asserts only the PUT path.
}

func TestSecretsGet_ResolverFailure(t *testing.T) {
	// Use newTestServer for plumbing, then swap in a custom resolver
	// that always raises a CycleError. The PUT path doesn't go through
	// the resolver — only the GET path does — so the seed succeeds and
	// the GET surfaces the bad_reference 400.
	ts := newTestServerWith(t, func(o *Options) {
		o.Resolver = cycleResolver{}
	})
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "comax"})
	mustCreate(t, ts, "/api/v1/projects/comax/envs", map[string]string{"name": "dev"})
	if status, env := ts.do(t, http.MethodPut,
		"/api/v1/projects/comax/envs/dev/secrets/K",
		map[string]string{"value": "literal"}); status != http.StatusCreated {
		t.Fatalf("seed PUT: status=%d env=%+v", status, env)
	}

	status, env := ts.do(t, http.MethodGet, "/api/v1/projects/comax/envs/dev/secrets/K", nil)
	if status != http.StatusBadRequest {
		t.Errorf("status = %d; want 400 (resolver cycle)", status)
	}
	if env.Error == nil || env.Error.Code != "bad_reference" {
		t.Errorf("error code = %+v; want bad_reference", env.Error)
	}
}
