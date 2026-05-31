package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// These tests pull the dashboard-session handlers through their rare
// error branches — audit append failure, commit failure, oversized
// metadata — using the same DROP-TABLE / oversized-input tricks that
// covered the M1 mutation handlers.

func TestDashboardSession_LoginAuditFailureIs500(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	if _, err := ts.db.Exec("DROP TABLE audit_log"); err != nil {
		t.Fatalf("drop audit_log: %v", err)
	}

	bodyBytes, _ := json.Marshal(map[string]string{"token": ts.bearer})
	req, _ := http.NewRequest(http.MethodPost,
		ts.srv.URL+"/api/v1/dashboard/session", bytes.NewReader(bodyBytes))
	resp, err := ts.srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status=%d; want 500 (audit failed)", resp.StatusCode)
	}
}

func TestDashboardSession_RevokeAuditFailureIs500(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	cookie, _ := dashLogin(t, ts)
	if _, err := ts.db.Exec("DROP TABLE audit_log"); err != nil {
		t.Fatalf("drop audit_log: %v", err)
	}

	req, _ := http.NewRequest(http.MethodDelete,
		ts.srv.URL+"/api/v1/dashboard/session", nil)
	req.AddCookie(&http.Cookie{
		Name: sessionCookieName, Value: cookie,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode,
	})
	resp, err := ts.srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status=%d; want 500 (audit failed)", resp.StatusCode)
	}
}

func TestTruncateUserAgent_CapsAt512(t *testing.T) {
	long := strings.Repeat("a", 700)
	got := truncateUserAgent(long)
	if len(got) != 512 {
		t.Errorf("len=%d; want 512 (truncated)", len(got))
	}
	if got != strings.Repeat("a", 512) {
		t.Errorf("content not preserved up to cap")
	}

	// Below-cap UAs pass through unchanged.
	if got := truncateUserAgent("short"); got != "short" {
		t.Errorf("short UA got mangled: %q", got)
	}
}

func TestDashboardSession_AuthSessionTokenGoneIs401(t *testing.T) {
	// Underlying token disappears while the session is still live — the
	// cookie must stop working, not panic. We simulate this by pointing
	// the session row at a non-existent token id; FK constraints block a
	// direct DELETE so we PRAGMA them off for the bare update.
	ts := newTestServer(t)
	ts.bootstrap(t)
	cookie, _ := dashLogin(t, ts)

	if _, err := ts.db.Exec("PRAGMA foreign_keys = OFF"); err != nil {
		t.Fatalf("disable fk: %v", err)
	}
	if _, err := ts.db.Exec("UPDATE dashboard_sessions SET token_id = 9999"); err != nil {
		t.Fatalf("rewrite token_id: %v", err)
	}

	status, _ := doCookie(t, ts, http.MethodGet, "/api/v1/projects", cookie, "", nil)
	if status != http.StatusUnauthorized {
		t.Errorf("status=%d; want 401 (underlying token revoked)", status)
	}
}
