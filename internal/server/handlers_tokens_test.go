package server

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/idenn207/comax-secrets/internal/store"
)

// mustIssueToken POSTs /api/v1/tokens as the current (admin) actor and
// returns the issued plaintext + id. It asserts the issued token is
// non-admin — the escalation guard the whole admin-only design rests on.
func mustIssueToken(t *testing.T, ts *testServer, name string) (plaintext string, id int64) {
	t.Helper()
	status, env := ts.do(t, http.MethodPost, "/api/v1/tokens", map[string]string{"name": name})
	if status != http.StatusCreated {
		t.Fatalf("issue token %q: status=%d env=%+v", name, status, env)
	}
	m, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("issue token data not object: %T", env.Data)
	}
	plaintext, _ = m["token"].(string)
	if plaintext == "" {
		t.Fatalf("issue token returned empty plaintext: %+v", env)
	}
	idf, _ := m["id"].(float64)
	if idf == 0 {
		t.Fatalf("issue token returned zero id: %+v", env)
	}
	if admin, _ := m["is_admin"].(bool); admin {
		t.Errorf("issued token is_admin=true; want false (issued tokens must be non-admin)")
	}
	return plaintext, int64(idf)
}

func TestTokens_IssueExposesPlaintextOnceAndAuthenticates(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)

	plain, _ := mustIssueToken(t, ts, "ci")

	// The plaintext returned once authenticates as a bearer for reads.
	admin := ts.bearer
	ts.bearer = plain
	if status, _ := ts.do(t, http.MethodGet, "/api/v1/projects", nil); status != http.StatusOK {
		t.Errorf("issued token read status=%d; want 200", status)
	}
	ts.bearer = admin
}

func TestTokens_ListShowsAdminFlagAndOmitsHash(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustIssueToken(t, ts, "ci")

	status, env := ts.do(t, http.MethodGet, "/api/v1/tokens", nil)
	if status != http.StatusOK {
		t.Fatalf("list status=%d env=%+v", status, env)
	}
	list, ok := env.Data.([]any)
	if !ok || len(list) != 2 {
		t.Fatalf("list len=%d; want 2 (bootstrap + ci)", len(list))
	}
	// id order: bootstrap (admin) first, ci (non-admin) second.
	if admin, _ := list[0].(map[string]any)["is_admin"].(bool); !admin {
		t.Error("bootstrap token is_admin=false; want true")
	}
	if admin, _ := list[1].(map[string]any)["is_admin"].(bool); admin {
		t.Error("ci token is_admin=true; want false")
	}
	// token_hash must never surface in a listing.
	for i, item := range list {
		if _, present := item.(map[string]any)["token_hash"]; present {
			t.Errorf("list[%d] leaked token_hash", i)
		}
	}
}

func TestTokens_NonAdminForbiddenOnAllManagementRoutes(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	plain, id := mustIssueToken(t, ts, "ci")

	// Switch to the non-admin token; every token-management route → 403.
	ts.bearer = plain
	cases := []struct {
		method, path string
		body         any
	}{
		{http.MethodPost, "/api/v1/tokens", map[string]string{"name": "escalate"}},
		{http.MethodGet, "/api/v1/tokens", nil},
		{http.MethodDelete, fmt.Sprintf("/api/v1/tokens/%d", id), nil},
	}
	for _, c := range cases {
		status, env := ts.do(t, c.method, c.path, c.body)
		if status != http.StatusForbidden {
			t.Errorf("%s %s: status=%d; want 403", c.method, c.path, status)
		}
		if env.Error == nil || env.Error.Code != "forbidden" {
			t.Errorf("%s %s: error=%+v; want code=forbidden", c.method, c.path, env.Error)
		}
	}
}

func TestTokens_RejectsInvalidName(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	for _, name := range []string{"", "slash/inside", "space inside"} {
		status, _ := ts.do(t, http.MethodPost, "/api/v1/tokens", map[string]string{"name": name})
		if status != http.StatusBadRequest {
			t.Errorf("name=%q status=%d; want 400", name, status)
		}
	}
}

func TestTokens_RevokeThenBearer401(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	plain, id := mustIssueToken(t, ts, "ci")

	if status, env := ts.do(t, http.MethodDelete, fmt.Sprintf("/api/v1/tokens/%d", id), nil); status != http.StatusNoContent {
		t.Fatalf("revoke status=%d env=%+v", status, env)
	}
	// The revoked bearer no longer authenticates.
	ts.bearer = plain
	if status, _ := ts.do(t, http.MethodGet, "/api/v1/projects", nil); status != http.StatusUnauthorized {
		t.Errorf("revoked bearer read status=%d; want 401", status)
	}
}

func TestTokens_RevokeUnknownOrDoubleIs404(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	_, id := mustIssueToken(t, ts, "ci")

	if status, _ := ts.do(t, http.MethodDelete, "/api/v1/tokens/9999", nil); status != http.StatusNotFound {
		t.Errorf("revoke unknown status=%d; want 404", status)
	}
	if status, _ := ts.do(t, http.MethodDelete, fmt.Sprintf("/api/v1/tokens/%d", id), nil); status != http.StatusNoContent {
		t.Fatalf("first revoke want 204")
	}
	if status, _ := ts.do(t, http.MethodDelete, fmt.Sprintf("/api/v1/tokens/%d", id), nil); status != http.StatusNotFound {
		t.Errorf("double revoke status=%d; want 404", status)
	}
}

// TestTokens_CannotRevokeLastLiveAdmin guards the lockout footgun: the
// sole live admin cannot revoke itself. A soft revoke keeps is_admin=1, so
// neither the migration backfill nor BootstrapIfEmpty could restore issuing
// rights — the operator would be stranded with no API path back.
func TestTokens_CannotRevokeLastLiveAdmin(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t) // the bootstrap token is the sole admin (id 1)

	status, env := ts.do(t, http.MethodDelete, "/api/v1/tokens/1", nil)
	if status != http.StatusConflict {
		t.Fatalf("revoke last admin status=%d; want 409 (env=%+v)", status, env)
	}
	if env.Error == nil || env.Error.Code != "conflict" {
		t.Errorf("revoke last admin error=%+v; want code=conflict", env.Error)
	}
	// The admin is still live — an admin-only route still works.
	if status, _ := ts.do(t, http.MethodGet, "/api/v1/tokens", nil); status != http.StatusOK {
		t.Errorf("admin still live: list status=%d; want 200", status)
	}
}

// TestTokens_RevokeAdminAllowedWhenAnotherAdminLives proves the guard is
// scoped to the LAST admin: with a second live admin present, revoking the
// first is allowed.
func TestTokens_RevokeAdminAllowedWhenAnotherAdminLives(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)

	// The API only ever issues non-admins, so seed a second admin directly.
	if _, err := store.NewTokenRepo(ts.db).Create(
		context.Background(), "admin2", []byte("0123456789abcdef0123456789abcdef"), true,
	); err != nil {
		t.Fatalf("seed second admin: %v", err)
	}
	// With two live admins, revoking the bootstrap admin (id 1) is allowed.
	if status, env := ts.do(t, http.MethodDelete, "/api/v1/tokens/1", nil); status != http.StatusNoContent {
		t.Fatalf("revoke admin with a backup status=%d; want 204 (env=%+v)", status, env)
	}
}

func TestTokens_CreateAndRevokeAudited(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)                                                        // audit: auth.bootstrap
	_, id := mustIssueToken(t, ts, "ci")                                   // audit: auth.token.create
	ts.do(t, http.MethodDelete, fmt.Sprintf("/api/v1/tokens/%d", id), nil) // audit: auth.token.revoke

	entries, err := store.NewAuditRepo(ts.db).ListRecent(context.Background(), 100)
	if err != nil {
		t.Fatalf("audit list: %v", err)
	}
	if len(entries) < 3 {
		t.Fatalf("audit rows = %d; want >= 3", len(entries))
	}
	if entries[0].Action != "auth.token.revoke" {
		t.Errorf("latest audit = %q; want auth.token.revoke", entries[0].Action)
	}
	if entries[1].Action != "auth.token.create" {
		t.Errorf("2nd audit = %q; want auth.token.create", entries[1].Action)
	}
	// Attribution: the admin actor (bootstrap token, id 1) is stamped.
	if entries[0].ActorToken == nil || *entries[0].ActorToken != 1 {
		t.Errorf("revoke actor = %v; want admin token id 1", entries[0].ActorToken)
	}
}

// TestTokens_RevokeTerminatesLiveSession is the R2-1 proof: a soft revoke
// must kill a live dashboard session, not only the bearer arm. The session
// arm re-hydrates via ByID (which returns revoked rows) and 401s at the
// middleware — before the handler's own authz would even run.
func TestTokens_RevokeTerminatesLiveSession(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	plain, id := mustIssueToken(t, ts, "ci")

	// Log a dashboard session in as the ci token (swap bearer around the
	// login primitive, then restore the admin bearer).
	admin := ts.bearer
	ts.bearer = plain
	cookie, csrf := dashLogin(t, ts)
	ts.bearer = admin

	// The session authenticates a read before revoke.
	if status, _ := doCookie(t, ts, http.MethodGet, "/api/v1/projects", cookie, csrf, nil); status != http.StatusOK {
		t.Fatalf("pre-revoke session read status=%d; want 200", status)
	}
	// A non-admin session hitting an admin route is 403 (handler authz).
	if status, _ := doCookie(t, ts, http.MethodGet, "/api/v1/tokens", cookie, csrf, nil); status != http.StatusForbidden {
		t.Fatalf("pre-revoke admin-route status=%d; want 403", status)
	}

	// Revoke the ci token as admin.
	if status, _ := ts.do(t, http.MethodDelete, fmt.Sprintf("/api/v1/tokens/%d", id), nil); status != http.StatusNoContent {
		t.Fatalf("revoke status=%d; want 204", status)
	}

	// The live session is now rejected at the middleware — 401, before the
	// handler's own authz runs.
	if status, _ := doCookie(t, ts, http.MethodGet, "/api/v1/projects", cookie, csrf, nil); status != http.StatusUnauthorized {
		t.Errorf("post-revoke session read status=%d; want 401 (R2-1)", status)
	}
}
