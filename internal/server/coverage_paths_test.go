package server

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/idenn207/comax-secrets/internal/secret"
)

// The tests in this file fill the coverage gaps the M1+M2 suites leave
// behind — the rarely-walked branches of resolverErrorToHTTP, the
// recoverMiddleware panic path, and the edge cases of paginated list
// endpoints. Each test pairs a one-method stub with one HTTP call so a
// future reader can find the branch from the test name alone.

// noRefResolver always raises secret.ErrUnknownReference so handlers
// that consult the resolver flip into the "bad_reference" 400 branch
// via resolverErrorToHTTP.
type noRefResolver struct{}

func (noRefResolver) Resolve(_ context.Context, _, _ int64) (secret.Snapshot, error) {
	return nil, secret.ErrUnknownReference
}

// errResolver raises an opaque error so the resolverErrorToHTTP default
// branch (500 "internal") is exercised — neither CycleError nor
// ErrUnknownReference.
type errResolver struct{}

func (errResolver) Resolve(_ context.Context, _, _ int64) (secret.Snapshot, error) {
	return nil, errors.New("resolver fell over")
}

// panicResolver panics inside Resolve so recoverMiddleware's recover()
// branch fires. The panic must escape the handler — never the goroutine
// — and the client must still receive a JSON 500 envelope.
type panicResolver struct{}

func (panicResolver) Resolve(_ context.Context, _, _ int64) (secret.Snapshot, error) {
	panic("resolver deliberately exploded")
}

// ---- resolverErrorToHTTP: ErrUnknownReference branch -------------------

func TestListSecrets_UnknownReferenceIs400(t *testing.T) {
	ts := newTestServerWith(t, func(o *Options) { o.Resolver = noRefResolver{} })
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "dev"})

	status, env := ts.do(t, http.MethodGet, "/api/v1/projects/app/envs/dev/secrets", nil)
	if status != http.StatusBadRequest {
		t.Fatalf("status=%d; want 400 (ErrUnknownReference)", status)
	}
	if env.Error == nil || env.Error.Code != "bad_reference" {
		t.Errorf("error=%+v; want code=bad_reference", env.Error)
	}
}

func TestDiffEnvs_UnknownReferenceIs400(t *testing.T) {
	ts := newTestServerWith(t, func(o *Options) { o.Resolver = noRefResolver{} })
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "dev"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "prod"})

	status, env := ts.do(t, http.MethodGet,
		"/api/v1/projects/app/envs/dev/diff?against=prod", nil)
	if status != http.StatusBadRequest {
		t.Fatalf("status=%d; want 400", status)
	}
	if env.Error == nil || env.Error.Code != "bad_reference" {
		t.Errorf("error=%+v; want bad_reference", env.Error)
	}
}

// ---- resolverErrorToHTTP: default 500 branch --------------------------

func TestListSecrets_GenericResolverErrorIs500(t *testing.T) {
	ts := newTestServerWith(t, func(o *Options) { o.Resolver = errResolver{} })
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "dev"})

	status, env := ts.do(t, http.MethodGet, "/api/v1/projects/app/envs/dev/secrets", nil)
	if status != http.StatusInternalServerError {
		t.Fatalf("status=%d; want 500", status)
	}
	if env.Error == nil || env.Error.Code != "internal" {
		t.Errorf("error=%+v; want code=internal", env.Error)
	}
}

func TestDiffEnvs_GenericResolverErrorIs500(t *testing.T) {
	ts := newTestServerWith(t, func(o *Options) { o.Resolver = errResolver{} })
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "dev"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "prod"})

	status, env := ts.do(t, http.MethodGet,
		"/api/v1/projects/app/envs/dev/diff?against=prod", nil)
	if status != http.StatusInternalServerError {
		t.Fatalf("status=%d; want 500", status)
	}
	if env.Error == nil || env.Error.Code != "internal" {
		t.Errorf("error=%+v; want internal", env.Error)
	}
}

// ---- recoverMiddleware: panic in handler -------------------------------

func TestRecoverMiddleware_PanicReturns500Envelope(t *testing.T) {
	ts := newTestServerWith(t, func(o *Options) { o.Resolver = panicResolver{} })
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "dev"})

	// handleListSecrets calls Resolver.Resolve before any other branch, so
	// the panic fires deep enough to prove the recover is wired around
	// the *handler chain*, not just around the mux.
	status, env := ts.do(t, http.MethodGet, "/api/v1/projects/app/envs/dev/secrets", nil)
	if status != http.StatusInternalServerError {
		t.Fatalf("status=%d; want 500 (recovered panic)", status)
	}
	if env.Error == nil || env.Error.Code != "internal" {
		t.Errorf("error=%+v; want code=internal", env.Error)
	}
}

// ---- handleListVersions: empty env -------------------------------------

func TestListVersions_EmptyEnvReturnsEmptyList(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "dev"})

	status, env := ts.do(t, http.MethodGet,
		"/api/v1/projects/app/envs/dev/versions", nil)
	if status != http.StatusOK {
		t.Fatalf("status=%d", status)
	}
	if env.Data == nil {
		return // nil == empty in JSON envelope; both are acceptable
	}
	rows, ok := env.Data.([]any)
	if !ok {
		t.Fatalf("data type=%T; want []any", env.Data)
	}
	if len(rows) != 0 {
		t.Errorf("rows=%d; want 0", len(rows))
	}
}

// ---- handleListAudit: limit > max → clamps to auditMaxLimit ----------

func TestListAudit_LimitClampsToMax(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	// limit=500 is above auditMaxLimit (200); meta.limit must show 200.
	status, env := ts.do(t, http.MethodGet, "/api/v1/audit?limit=500", nil)
	if status != http.StatusOK {
		t.Fatalf("status=%d", status)
	}
	meta, ok := env.Meta.(map[string]any)
	if !ok {
		t.Fatalf("meta type=%T; want map", env.Meta)
	}
	got, _ := meta["limit"].(float64)
	if int(got) != auditMaxLimit {
		t.Errorf("limit=%v; want %d (clamped)", got, auditMaxLimit)
	}
}

// ---- handleListProjects: with a project (covers the rows loop) -------
//
// handleListProjects shows up at 60% because the existing tests either
// list *zero* projects (immediately after bootstrap) or list *one* — the
// branch that iterates more than one row goes uncovered. Two projects in
// one call exercises the slice-grow path and a deterministic order.

func TestListProjects_MultipleEntries(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "alpha"})
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "beta"})
	// One env under alpha, none under beta. The list response must carry
	// the env_count field so the Projects grid can render its configs chip
	// without a second round-trip per card.
	mustCreate(t, ts, "/api/v1/projects/alpha/envs", map[string]string{"name": "dev"})

	status, env := ts.do(t, http.MethodGet, "/api/v1/projects", nil)
	if status != http.StatusOK {
		t.Fatalf("status=%d", status)
	}
	rows := env.Data.([]any)
	if len(rows) != 2 {
		t.Fatalf("rows=%d; want 2", len(rows))
	}
	counts := map[string]float64{}
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("row type=%T; want map", raw)
		}
		name, _ := row["name"].(string)
		count, ok := row["env_count"].(float64)
		if !ok {
			t.Fatalf("row %q missing env_count: %+v", name, row)
		}
		counts[name] = count
	}
	if counts["alpha"] != 1 {
		t.Errorf("alpha env_count = %v; want 1", counts["alpha"])
	}
	if counts["beta"] != 0 {
		t.Errorf("beta env_count = %v; want 0 (LEFT JOIN surfaces zero-env projects)", counts["beta"])
	}
}

// ---- handleCreateEnv: missing-name validation -----------------------

func TestCreateEnv_RejectsEmptyName(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})

	status, _ := ts.do(t, http.MethodPost, "/api/v1/projects/app/envs",
		map[string]string{"name": ""})
	if status != http.StatusBadRequest {
		t.Errorf("status=%d; want 400 (empty name)", status)
	}
}

// ---- handleCreateEnv: duplicate name → 409 conflict ----------------

func TestCreateEnv_DuplicateNameIs409(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "dev"})

	status, env := ts.do(t, http.MethodPost, "/api/v1/projects/app/envs",
		map[string]string{"name": "dev"})
	if status != http.StatusConflict {
		t.Errorf("status=%d; want 409 (duplicate env)", status)
	}
	if env.Error == nil {
		t.Errorf("env error nil; want conflict envelope")
	}
}
