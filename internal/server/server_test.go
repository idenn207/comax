package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/idenn207/comax-secrets/internal/crypto"
	"github.com/idenn207/comax-secrets/internal/store"
)

// staticKey is a deterministic master key for tests. Real deployments
// use FileKeyProvider; here we hand the server an in-memory key so the
// suite doesn't depend on the filesystem.
type staticKey []byte

func (k staticKey) Key(_ context.Context) ([]byte, error) { return k, nil }

// testServer spins up an httptest.Server backed by a fresh on-disk
// SQLite DB and an in-memory random master key. The returned helpers
// streamline the call shape every CRUD test needs.
type testServer struct {
	srv    *httptest.Server
	db     *sql.DB
	bearer string // populated after bootstrap()
}

func newTestServer(t *testing.T) *testServer {
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
	if _, err := rand.Read(keyBytes); err != nil {
		t.Fatalf("rand key: %v", err)
	}

	s := NewServer(Options{DB: db, Keys: staticKey(keyBytes)})
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)

	return &testServer{srv: srv, db: db}
}

// do issues a request against the test server. bearer is auto-attached
// when t.bearer is set. Returns the parsed envelope and the raw status.
func (t *testServer) do(testing_ *testing.T, method, path string, body any) (int, Envelope) {
	testing_.Helper()
	var bodyReader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			testing_.Fatalf("marshal body: %v", err)
		}
		bodyReader = bytes.NewReader(buf)
	}
	req, err := http.NewRequest(method, t.srv.URL+path, bodyReader)
	if err != nil {
		testing_.Fatalf("new request: %v", err)
	}
	if t.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+t.bearer)
	}
	resp, err := t.srv.Client().Do(req)
	if err != nil {
		testing_.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	var env Envelope
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &env); err != nil {
			testing_.Fatalf("unmarshal envelope (status=%d, body=%q): %v", resp.StatusCode, raw, err)
		}
	}
	return resp.StatusCode, env
}

// bootstrap calls POST /api/v1/bootstrap and stashes the returned token
// onto t.bearer so subsequent do() calls authenticate automatically.
func (t *testServer) bootstrap(testing_ *testing.T) {
	testing_.Helper()
	status, env := t.do(testing_, http.MethodPost, "/api/v1/bootstrap", nil)
	if status != http.StatusCreated {
		testing_.Fatalf("bootstrap status = %d; want 201; env=%+v", status, env)
	}
	m, ok := env.Data.(map[string]any)
	if !ok {
		testing_.Fatalf("bootstrap data is not an object: %T", env.Data)
	}
	tok, _ := m["token"].(string)
	if tok == "" {
		testing_.Fatalf("bootstrap returned empty token: %+v", env)
	}
	t.bearer = tok
}

// ---- /healthz ---------------------------------------------------------

func TestHealthz(t *testing.T) {
	ts := newTestServer(t)
	status, env := ts.do(t, http.MethodGet, "/healthz", nil)
	if status != http.StatusOK || !env.OK {
		t.Errorf("healthz: status=%d ok=%v", status, env.OK)
	}
}

// ---- bootstrap --------------------------------------------------------

func TestBootstrap_FirstSucceedsSecondConflicts(t *testing.T) {
	ts := newTestServer(t)

	status, env := ts.do(t, http.MethodPost, "/api/v1/bootstrap", nil)
	if status != http.StatusCreated || !env.OK {
		t.Fatalf("first bootstrap: status=%d env=%+v", status, env)
	}

	status, env = ts.do(t, http.MethodPost, "/api/v1/bootstrap", nil)
	if status != http.StatusConflict {
		t.Errorf("second bootstrap status=%d; want 409", status)
	}
	if env.Error == nil || env.Error.Code != "already_bootstrapped" {
		t.Errorf("second bootstrap error code = %+v; want already_bootstrapped", env.Error)
	}
}

// ---- auth gate --------------------------------------------------------

func TestAuth_RejectsMissingBearer(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	ts.bearer = "" // strip bearer to confirm middleware blocks
	status, env := ts.do(t, http.MethodGet, "/api/v1/projects", nil)
	if status != http.StatusUnauthorized {
		t.Errorf("status = %d; want 401", status)
	}
	if env.Error == nil || env.Error.Code != "unauthorized" {
		t.Errorf("error = %+v; want code=unauthorized", env.Error)
	}
}

func TestAuth_RejectsUnknownBearer(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	ts.bearer = "definitely-not-a-real-token"
	status, _ := ts.do(t, http.MethodGet, "/api/v1/projects", nil)
	if status != http.StatusUnauthorized {
		t.Errorf("status = %d; want 401", status)
	}
}

// ---- projects ---------------------------------------------------------

func TestProjects_CreateAndList(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)

	status, env := ts.do(t, http.MethodPost, "/api/v1/projects", map[string]string{"name": "comax"})
	if status != http.StatusCreated {
		t.Fatalf("create status = %d env=%+v", status, env)
	}

	status, _ = ts.do(t, http.MethodPost, "/api/v1/projects", map[string]string{"name": "comax"})
	if status != http.StatusConflict {
		t.Errorf("duplicate create status = %d; want 409", status)
	}

	status, env = ts.do(t, http.MethodGet, "/api/v1/projects", nil)
	if status != http.StatusOK {
		t.Fatalf("list status = %d", status)
	}
	projects, ok := env.Data.([]any)
	if !ok || len(projects) != 1 {
		t.Errorf("list returned %d projects; want 1 (data=%+v)", len(projects), env.Data)
	}
}

func TestProjects_RejectsInvalidName(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	cases := []struct {
		name string
		want string
	}{
		{"", "empty name should be rejected"},
		{"slash/inside", "slash should be rejected"},
		{"space inside", "space should be rejected"},
	}
	for _, c := range cases {
		status, env := ts.do(t, http.MethodPost, "/api/v1/projects", map[string]string{"name": c.name})
		if status != http.StatusBadRequest {
			t.Errorf("%s: status=%d env=%+v", c.want, status, env)
		}
	}
}

// ---- envs -------------------------------------------------------------

func TestEnvs_CreateAndList(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "comax"})

	mustCreate(t, ts, "/api/v1/projects/comax/envs", map[string]string{"name": "dev"})
	mustCreate(t, ts, "/api/v1/projects/comax/envs", map[string]string{"name": "prod", "inherits_from": "dev"})

	status, env := ts.do(t, http.MethodGet, "/api/v1/projects/comax/envs", nil)
	if status != http.StatusOK {
		t.Fatalf("list envs status = %d", status)
	}
	envs := env.Data.([]any)
	if len(envs) != 2 {
		t.Errorf("got %d envs; want 2", len(envs))
	}
}

func TestEnvs_404OnUnknownProject(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	status, _ := ts.do(t, http.MethodGet, "/api/v1/projects/no-such-project/envs", nil)
	if status != http.StatusNotFound {
		t.Errorf("status = %d; want 404", status)
	}
}

// ---- secrets ----------------------------------------------------------

func TestSecrets_FullRoundTrip(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "comax"})
	mustCreate(t, ts, "/api/v1/projects/comax/envs", map[string]string{"name": "dev"})

	// Initial PUT → 201.
	status, env := ts.do(t, http.MethodPut, "/api/v1/projects/comax/envs/dev/secrets/DB_URL",
		map[string]string{"value": "postgres://localhost/dev"})
	if status != http.StatusCreated {
		t.Fatalf("first put status=%d env=%+v", status, env)
	}
	if got := env.Data.(map[string]any)["version"].(float64); got != 1 {
		t.Errorf("version = %v; want 1", got)
	}

	// Update → 200 and version bumps.
	status, env = ts.do(t, http.MethodPut, "/api/v1/projects/comax/envs/dev/secrets/DB_URL",
		map[string]string{"value": "postgres://localhost/dev?sslmode=disable"})
	if status != http.StatusOK {
		t.Fatalf("update status=%d", status)
	}
	if got := env.Data.(map[string]any)["version"].(float64); got != 2 {
		t.Errorf("version after update = %v; want 2", got)
	}

	// Single GET returns the latest decrypted value.
	status, env = ts.do(t, http.MethodGet, "/api/v1/projects/comax/envs/dev/secrets/DB_URL", nil)
	if status != http.StatusOK {
		t.Fatalf("get status=%d", status)
	}
	got := env.Data.(map[string]any)
	if got["value"].(string) != "postgres://localhost/dev?sslmode=disable" {
		t.Errorf("decrypted value mismatch: %v", got["value"])
	}

	// List returns one entry.
	status, env = ts.do(t, http.MethodGet, "/api/v1/projects/comax/envs/dev/secrets", nil)
	if status != http.StatusOK {
		t.Fatalf("list status=%d", status)
	}
	if list, _ := env.Data.([]any); len(list) != 1 {
		t.Errorf("list len=%d; want 1", len(list))
	}

	// Versions endpoint returns both history rows.
	status, env = ts.do(t, http.MethodGet, "/api/v1/projects/comax/envs/dev/versions", nil)
	if status != http.StatusOK {
		t.Fatalf("versions status=%d", status)
	}
	versions := env.Data.([]any)
	if len(versions) != 2 {
		t.Errorf("versions = %d; want 2", len(versions))
	}
}

func TestSecrets_404OnMissingKey(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "comax"})
	mustCreate(t, ts, "/api/v1/projects/comax/envs", map[string]string{"name": "dev"})

	status, _ := ts.do(t, http.MethodGet, "/api/v1/projects/comax/envs/dev/secrets/NOPE", nil)
	if status != http.StatusNotFound {
		t.Errorf("status = %d; want 404", status)
	}
}

// ---- audit trail ------------------------------------------------------

// TestAuditWritten checks that every state-changing endpoint appends to
// audit_log inside the same transaction. We verify the count after each
// mutation rather than peeking at individual rows — the existence check
// is what proves the audit ring is wired.
func TestAuditWritten(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t) // +1 audit row
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "comax"}) // +1
	mustCreate(t, ts, "/api/v1/projects/comax/envs", map[string]string{"name": "dev"}) // +1
	ts.do(t, http.MethodPut, "/api/v1/projects/comax/envs/dev/secrets/K",
		map[string]string{"value": "v"}) // +1

	entries, err := store.NewAuditRepo(ts.db).ListRecent(context.Background(), 100)
	if err != nil {
		t.Fatalf("audit list: %v", err)
	}
	if len(entries) != 4 {
		t.Errorf("audit rows = %d; want 4 (bootstrap + project + env + secret)", len(entries))
	}
	// Spot-check that the latest one is the secret upsert.
	if entries[0].Action != "secret.upsert" {
		t.Errorf("latest audit action = %q; want secret.upsert", entries[0].Action)
	}
}

// mustCreate POSTs body to path and fails the test if the status isn't
// 201. Helper for tests that don't care about the response body.
func mustCreate(t *testing.T, ts *testServer, path string, body any) {
	t.Helper()
	status, env := ts.do(t, http.MethodPost, path, body)
	if status != http.StatusCreated {
		t.Fatalf("POST %s: status=%d env=%+v", path, status, env)
	}
}

// TestEndToEnd_InheritanceAndReferences exercises the Task 6 features
// through the public HTTP surface: a child env inherits from "shared",
// then a third env references a key in "shared" via ${{ shared.KEY }}.
// This is the integration analogue of the unit-level coverage in
// internal/secret/resolver_test.go.
func TestEndToEnd_InheritanceAndReferences(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "comax"})
	mustCreate(t, ts, "/api/v1/projects/comax/envs", map[string]string{"name": "shared"})
	mustCreate(t, ts, "/api/v1/projects/comax/envs",
		map[string]string{"name": "dev", "inherits_from": "shared"})

	// Seed shared.DB_HOST and shared.API_KEY.
	if status, _ := ts.do(t, http.MethodPut,
		"/api/v1/projects/comax/envs/shared/secrets/DB_HOST",
		map[string]string{"value": "db.shared.local"}); status != http.StatusCreated {
		t.Fatalf("seed shared.DB_HOST: status=%d", status)
	}
	if status, _ := ts.do(t, http.MethodPut,
		"/api/v1/projects/comax/envs/shared/secrets/API_KEY",
		map[string]string{"value": "sk_shared"}); status != http.StatusCreated {
		t.Fatalf("seed shared.API_KEY: status=%d", status)
	}
	// Dev overrides DB_HOST and pulls in DB_URL via ${{ ... }}.
	if status, _ := ts.do(t, http.MethodPut,
		"/api/v1/projects/comax/envs/dev/secrets/DB_HOST",
		map[string]string{"value": "db.dev.local"}); status != http.StatusCreated {
		t.Fatalf("seed dev.DB_HOST: status=%d", status)
	}
	if status, _ := ts.do(t, http.MethodPut,
		"/api/v1/projects/comax/envs/dev/secrets/DB_URL",
		map[string]string{"value": "postgres://${{ shared.DB_HOST }}/app"}); status != http.StatusCreated {
		t.Fatalf("seed dev.DB_URL: status=%d", status)
	}

	// Pull dev: expect DB_HOST=dev-override (child wins), API_KEY
	// inherited verbatim, DB_URL with reference expanded to the
	// *shared* DB_HOST (resolver looks up shared's snapshot, not dev's).
	status, env := ts.do(t, http.MethodGet, "/api/v1/projects/comax/envs/dev/secrets", nil)
	if status != http.StatusOK {
		t.Fatalf("list dev: status=%d env=%+v", status, env)
	}
	views := env.Data.([]any)
	got := map[string]string{}
	for _, v := range views {
		m := v.(map[string]any)
		got[m["key"].(string)] = m["value"].(string)
	}
	wantValues := map[string]string{
		"DB_HOST": "db.dev.local",
		"API_KEY": "sk_shared",
		"DB_URL":  "postgres://db.shared.local/app",
	}
	for k, v := range wantValues {
		if got[k] != v {
			t.Errorf("dev[%s] = %q; want %q", k, got[k], v)
		}
	}
}
