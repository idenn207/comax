package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/idenn207/comax-secrets/internal/store"
)

// dashLogin POSTs the bearer to /dashboard/session and returns the
// session cookie + CSRF plaintext. Tests use this as the "browser
// login" primitive so the call sites stay readable.
func dashLogin(t *testing.T, ts *testServer) (cookie, csrf string) {
	t.Helper()
	bodyBytes, _ := json.Marshal(map[string]string{"token": ts.bearer})
	req, err := http.NewRequest(http.MethodPost, ts.srv.URL+"/api/v1/dashboard/session", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("login request: %v", err)
	}
	resp, err := ts.srv.Client().Do(req)
	if err != nil {
		t.Fatalf("login do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("login status=%d body=%s", resp.StatusCode, raw)
	}
	for _, c := range resp.Cookies() {
		if c.Name == sessionCookieName {
			cookie = c.Value
		}
	}
	if cookie == "" {
		t.Fatalf("login did not set %s cookie", sessionCookieName)
	}
	var env Envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode login body: %v", err)
	}
	m, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("login data is not an object: %T", env.Data)
	}
	csrf, _ = m["csrf"].(string)
	if csrf == "" {
		t.Fatalf("login did not return csrf token")
	}
	return cookie, csrf
}

// doCookie issues a request authenticated by the dashboard cookie,
// attaching the CSRF header when one is supplied. Returns (status, body
// bytes) so tests can assert both.
func doCookie(t *testing.T, ts *testServer, method, path, cookie, csrf string, body any) (int, []byte) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, ts.srv.URL+path, reader)
	if err != nil {
		t.Fatalf("doCookie new request: %v", err)
	}
	req.AddCookie(&http.Cookie{
		Name: sessionCookieName, Value: cookie,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode,
	})
	if csrf != "" {
		req.Header.Set(csrfHeader, csrf)
	}
	resp, err := ts.srv.Client().Do(req)
	if err != nil {
		t.Fatalf("doCookie do: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, raw
}

// ---- POST /api/v1/dashboard/session -----------------------------------

func TestDashboardSession_LoginSetsCookieAndReturnsCSRF(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	cookie, csrf := dashLogin(t, ts)
	if cookie == "" || csrf == "" {
		t.Errorf("login cookie=%q csrf=%q", cookie, csrf)
	}
}

func TestDashboardSession_LoginRejectsBadBearer(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	bodyBytes, _ := json.Marshal(map[string]string{"token": "not-a-real-bearer"})
	req, _ := http.NewRequest(http.MethodPost,
		ts.srv.URL+"/api/v1/dashboard/session", bytes.NewReader(bodyBytes))
	resp, err := ts.srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status=%d; want 401", resp.StatusCode)
	}
}

func TestDashboardSession_LoginRejectsBadJSON(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	status, _ := rawDo(t, ts, http.MethodPost, "/api/v1/dashboard/session", "{not json")
	if status != http.StatusBadRequest {
		t.Errorf("status=%d; want 400", status)
	}
}

// ---- CSRF enforcement on mutating requests via cookie -----------------

func TestDashboardSession_CookieGetWorksWithoutCSRF(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	cookie, _ := dashLogin(t, ts)

	status, _ := doCookie(t, ts, http.MethodGet, "/api/v1/projects", cookie, "", nil)
	if status != http.StatusOK {
		t.Errorf("cookie GET status=%d; want 200 (no CSRF required)", status)
	}
}

func TestDashboardSession_CookiePutRequiresCSRF(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "dev"})
	cookie, csrf := dashLogin(t, ts)

	// Without CSRF header → 403.
	status, body := doCookie(t, ts, http.MethodPut,
		"/api/v1/projects/app/envs/dev/secrets/K", cookie, "",
		map[string]string{"value": "v"})
	if status != http.StatusForbidden {
		t.Errorf("no-csrf PUT status=%d; want 403; body=%s", status, body)
	}

	// With CSRF header → 201 (new secret).
	status, body = doCookie(t, ts, http.MethodPut,
		"/api/v1/projects/app/envs/dev/secrets/K", cookie, csrf,
		map[string]string{"value": "v"})
	if status != http.StatusCreated {
		t.Errorf("with-csrf PUT status=%d; want 201; body=%s", status, body)
	}
}

func TestDashboardSession_CookieCSRFMismatchIs403(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "dev"})
	cookie, _ := dashLogin(t, ts)

	status, _ := doCookie(t, ts, http.MethodPut,
		"/api/v1/projects/app/envs/dev/secrets/K", cookie, "wrong-csrf",
		map[string]string{"value": "v"})
	if status != http.StatusForbidden {
		t.Errorf("status=%d; want 403 (csrf mismatch)", status)
	}
}

// ---- Bearer arm regression: header-auth requests are unaffected ------

func TestDashboardSession_BearerArmSkipsCSRF(t *testing.T) {
	// Bearer-auth requests must keep working without any CSRF token —
	// the dashboard cookie arm is additive, not a replacement.
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "dev"})

	status, _ := ts.do(t, http.MethodPut,
		"/api/v1/projects/app/envs/dev/secrets/K",
		map[string]string{"value": "v"})
	if status != http.StatusCreated {
		t.Errorf("bearer PUT status=%d; want 201 (no CSRF needed)", status)
	}
}

// ---- Audit attribution under cookie auth -----------------------------

func TestDashboardSession_CookieMutationAttributesToBearerToken(t *testing.T) {
	// Per plan: session is just a vehicle for the same identity. An
	// audit row written under cookie auth must show the same actor_token
	// the underlying bearer represents.
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "dev"})
	cookie, csrf := dashLogin(t, ts)

	if status, _ := doCookie(t, ts, http.MethodPut,
		"/api/v1/projects/app/envs/dev/secrets/K", cookie, csrf,
		map[string]string{"value": "v"}); status != http.StatusCreated {
		t.Fatalf("cookie PUT status=%d", status)
	}

	// Look up audit rows for the upsert and assert the actor matches
	// the bootstrap token (id=1, the only token in this test DB).
	entries, err := store.NewAuditRepo(ts.db).List(context.Background(),
		store.AuditFilter{Action: "secret.upsert"}, 10)
	if err != nil {
		t.Fatalf("audit list: %v", err)
	}
	if len(entries) != 1 || entries[0].ActorToken == nil || *entries[0].ActorToken != 1 {
		t.Errorf("audit attribution = %+v; want actor_token=1", entries)
	}
}

// ---- Revoke flow -----------------------------------------------------

func TestDashboardSession_RevokeKillsSubsequentRequests(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	cookie, _ := dashLogin(t, ts)

	// Revoke.
	req, _ := http.NewRequest(http.MethodDelete, ts.srv.URL+"/api/v1/dashboard/session", nil)
	req.AddCookie(&http.Cookie{
		Name: sessionCookieName, Value: cookie,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode,
	})
	resp, err := ts.srv.Client().Do(req)
	if err != nil {
		t.Fatalf("revoke do: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("revoke status=%d; want 204", resp.StatusCode)
	}
	// Confirm Set-Cookie clears the value.
	var cleared bool
	for _, c := range resp.Cookies() {
		if c.Name == sessionCookieName && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Errorf("revoke did not emit a clearing Set-Cookie header")
	}

	// Subsequent cookie request returns 401.
	status, _ := doCookie(t, ts, http.MethodGet, "/api/v1/projects", cookie, "", nil)
	if status != http.StatusUnauthorized {
		t.Errorf("after-revoke GET status=%d; want 401", status)
	}
}

func TestDashboardSession_RevokeWithoutCookieIs204(t *testing.T) {
	// Logout-when-already-logged-out should not 4xx; the dashboard's
	// "click logout twice" UX needs to stay clean.
	ts := newTestServer(t)
	ts.bootstrap(t)
	req, _ := http.NewRequest(http.MethodDelete, ts.srv.URL+"/api/v1/dashboard/session", nil)
	resp, err := ts.srv.Client().Do(req)
	if err != nil {
		t.Fatalf("revoke do: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("no-cookie revoke status=%d; want 204", resp.StatusCode)
	}
}

func TestDashboardSession_RevokeUnknownCookieIs204AndClears(t *testing.T) {
	// Cookie present but stale (server-side session pruned) — still 204.
	ts := newTestServer(t)
	ts.bootstrap(t)
	req, _ := http.NewRequest(http.MethodDelete, ts.srv.URL+"/api/v1/dashboard/session", nil)
	req.AddCookie(&http.Cookie{
		Name: sessionCookieName, Value: "stale-cookie-value",
		HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode,
	})
	resp, err := ts.srv.Client().Do(req)
	if err != nil {
		t.Fatalf("revoke do: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("stale revoke status=%d; want 204", resp.StatusCode)
	}
}

// ---- GET /api/v1/dashboard/sessions ----------------------------------

func TestDashboardSessions_ListReturnsOnlyOwnTokenSessions(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)

	// Two logins from the actor token. Each call mints a fresh server-
	// side session; both rows are owned by the bootstrap token (id=1).
	cookieA, csrfA := dashLogin(t, ts)
	_, _ = dashLogin(t, ts)

	// Seed a session owned by a *different* token. The list endpoint
	// must not surface it under the actor's listing — that's the whole
	// point of the token_id scope on ListByTokenID.
	secondTok, err := store.NewTokenRepo(ts.db).Create(context.Background(),
		"second", hashBearer("second"), false)
	if err != nil {
		t.Fatalf("seed second token: %v", err)
	}
	if _, err := store.NewSessionRepo(ts.db).Create(context.Background(),
		store.SessionCreateInput{
			TokenID:     secondTok.ID,
			SessionHash: hashSeed("other-session"),
			CSRFHash:    hashSeed("other-csrf"),
			TTL:         time.Hour,
		}); err != nil {
		t.Fatalf("seed second session: %v", err)
	}

	status, body := doCookie(t, ts, http.MethodGet, "/api/v1/dashboard/sessions",
		cookieA, csrfA, nil)
	if status != http.StatusOK {
		t.Fatalf("list status=%d body=%s", status, body)
	}
	var env Envelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	rows, ok := env.Data.([]any)
	if !ok {
		t.Fatalf("data not array: %T", env.Data)
	}
	if len(rows) != 2 {
		t.Fatalf("list returned %d rows; want 2 (no foreign-token rows)", len(rows))
	}

	// Exactly one row must be flagged is_current — the cookie used here.
	var currents int
	for _, r := range rows {
		m := r.(map[string]any)
		if cur, _ := m["is_current"].(bool); cur {
			currents++
		}
	}
	if currents != 1 {
		t.Errorf("is_current flag count=%d; want 1", currents)
	}
}

// ---- DELETE /api/v1/dashboard/sessions/{id} --------------------------

func TestDashboardSessions_RevokeByIDOwnSessionWritesAudit(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	cookie, csrf := dashLogin(t, ts)

	// Mint a second session for the actor so revoking it doesn't kill
	// the very cookie that authenticates the request.
	other, err := store.NewSessionRepo(ts.db).Create(context.Background(),
		store.SessionCreateInput{
			TokenID:     1, // bootstrap token id
			SessionHash: hashSeed("self-other-session"),
			CSRFHash:    hashSeed("self-other-csrf"),
			TTL:         time.Hour,
		})
	if err != nil {
		t.Fatalf("seed own second session: %v", err)
	}

	status, body := doCookie(t, ts, http.MethodDelete,
		fmt.Sprintf("/api/v1/dashboard/sessions/%d", other.ID), cookie, csrf, nil)
	if status != http.StatusNoContent {
		t.Fatalf("revoke own status=%d body=%s", status, body)
	}

	// Audit row recorded under action session.revoke_by_id, attributed
	// to the actor's underlying token.
	entries, err := store.NewAuditRepo(ts.db).List(context.Background(),
		store.AuditFilter{Action: "session.revoke_by_id"}, 10)
	if err != nil {
		t.Fatalf("audit list: %v", err)
	}
	if len(entries) != 1 || entries[0].ActorToken == nil || *entries[0].ActorToken != 1 {
		t.Errorf("audit rows=%+v; want exactly one with actor_token=1", entries)
	}
}

func TestDashboardSessions_RevokeByIDCrossTokenSilentNoAudit(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	cookie, csrf := dashLogin(t, ts)

	// Foreign token + session.
	foreignTok, err := store.NewTokenRepo(ts.db).Create(context.Background(),
		"foreign", hashBearer("foreign"), false)
	if err != nil {
		t.Fatalf("seed foreign token: %v", err)
	}
	foreignSess, err := store.NewSessionRepo(ts.db).Create(context.Background(),
		store.SessionCreateInput{
			TokenID:     foreignTok.ID,
			SessionHash: hashSeed("foreign-session"),
			CSRFHash:    hashSeed("foreign-csrf"),
			TTL:         time.Hour,
		})
	if err != nil {
		t.Fatalf("seed foreign session: %v", err)
	}

	// Actor revokes foreign id → 204, indistinguishable from "unknown id".
	status, body := doCookie(t, ts, http.MethodDelete,
		fmt.Sprintf("/api/v1/dashboard/sessions/%d", foreignSess.ID), cookie, csrf, nil)
	if status != http.StatusNoContent {
		t.Fatalf("cross-token revoke status=%d body=%s", status, body)
	}

	// Foreign session row stays live.
	survivor, err := store.NewSessionRepo(ts.db).ByHash(context.Background(),
		hashSeed("foreign-session"))
	if err != nil {
		t.Fatalf("foreign session vanished: %v", err)
	}
	if survivor.RevokedAt != nil {
		t.Errorf("cross-token attempt mutated foreign row: revoked_at=%v", *survivor.RevokedAt)
	}

	// No audit row — probing foreign ids leaves no trail.
	entries, err := store.NewAuditRepo(ts.db).List(context.Background(),
		store.AuditFilter{Action: "session.revoke_by_id"}, 10)
	if err != nil {
		t.Fatalf("audit list: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("cross-token revoke wrote %d audit row(s); want 0", len(entries))
	}
}

func TestDashboardSessions_RevokeByIDUnknownIs204NoAudit(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	cookie, csrf := dashLogin(t, ts)

	status, body := doCookie(t, ts, http.MethodDelete,
		"/api/v1/dashboard/sessions/999999", cookie, csrf, nil)
	if status != http.StatusNoContent {
		t.Fatalf("unknown revoke status=%d body=%s", status, body)
	}
	entries, err := store.NewAuditRepo(ts.db).List(context.Background(),
		store.AuditFilter{Action: "session.revoke_by_id"}, 10)
	if err != nil {
		t.Fatalf("audit list: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("unknown revoke wrote %d audit row(s); want 0", len(entries))
	}
}

func TestDashboardSessions_RevokeByIDRequiresCSRF(t *testing.T) {
	// CSRF middleware applies to all mutating methods on cookie-auth.
	// This is a wiring regression test, not new logic.
	ts := newTestServer(t)
	ts.bootstrap(t)
	cookie, _ := dashLogin(t, ts)

	status, _ := doCookie(t, ts, http.MethodDelete,
		"/api/v1/dashboard/sessions/1", cookie, "", nil)
	if status != http.StatusForbidden {
		t.Errorf("no-csrf revoke status=%d; want 403", status)
	}
}

// hashSeed and hashBearer are deterministic SHA-256s used to seed test
// rows directly through the store. The repo never inspects the bytes
// beyond equality, so deterministic synthetic hashes are sufficient.
func hashSeed(seed string) []byte {
	h := sha256.Sum256([]byte("test-seed:" + seed))
	return h[:]
}

func hashBearer(seed string) []byte {
	h := sha256.Sum256([]byte("test-bearer:" + seed))
	return h[:]
}

// ---- 401 envelope shape for missing-credential requests --------------

func TestAuth_NoBearerNoCookieIs401(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	ts.bearer = "" // strip both arms
	status, env := ts.do(t, http.MethodGet, "/api/v1/projects", nil)
	if status != http.StatusUnauthorized {
		t.Errorf("status=%d; want 401", status)
	}
	if env.Error == nil || !strings.Contains(env.Error.Code, "unauthorized") {
		t.Errorf("error=%+v; want unauthorized code", env.Error)
	}
}

// ---- bearer arm takes precedence when both are present ---------------

func TestAuth_BearerWinsWhenCookieAlsoPresent(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "dev"})
	cookie, _ := dashLogin(t, ts) // valid cookie

	// Send PUT with bearer AND cookie but NO csrf header. Bearer arm
	// wins, so CSRF is not required and the write succeeds.
	bodyBytes, _ := json.Marshal(map[string]string{"value": "v"})
	req, _ := http.NewRequest(http.MethodPut,
		ts.srv.URL+"/api/v1/projects/app/envs/dev/secrets/K",
		bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+ts.bearer)
	req.AddCookie(&http.Cookie{
		Name: sessionCookieName, Value: cookie,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode,
	})
	resp, err := ts.srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("bearer+cookie PUT status=%d; want 201 (bearer wins, no CSRF needed)", resp.StatusCode)
	}
}
