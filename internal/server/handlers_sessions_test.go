package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

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
