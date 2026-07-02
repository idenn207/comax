package server

import (
	"context"
	"crypto/rand"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/idenn207/comax-secrets/internal/crypto"
	"github.com/idenn207/comax-secrets/internal/store"
)

// newTestServerWithSPA boots a httptest.Server like newTestServer, but
// attaches a custom SPA FS. Pass nil to simulate dev-mode / dashboard-
// disabled. The helper deliberately mirrors newTestServer's shape so
// adopting it from other tests is trivial.
func newTestServerWithSPA(t *testing.T, spaFS fs.FS) *testServer {
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

	s := NewServer(Options{DB: db, Keys: staticKey(keyBytes), SPAFS: spaFS})
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	return &testServer{srv: srv, db: db}
}

// fakeSPA returns a tiny in-memory dashboard bundle suitable for
// handler tests. index.html carries the literal CSP nonce placeholder so
// the substitution test can prove the swap fires.
func fakeSPA() fstest.MapFS {
	return fstest.MapFS{
		"index.html": &fstest.MapFile{
			Data: []byte(`<!doctype html><html><head>` +
				`<script nonce="__CSP_NONCE__" src="/assets/main.js"></script>` +
				`</head><body><div id="root"></div></body></html>`),
		},
		"assets/main.js":  &fstest.MapFile{Data: []byte("console.log('hi')")},
		"assets/main.css": &fstest.MapFile{Data: []byte("body{}")},
		"favicon.ico":     &fstest.MapFile{Data: []byte("\x00\x00\x01\x00")},
	}
}

// rawGET performs a GET that doesn't expect a JSON envelope so the
// caller can inspect headers and the raw body string.
func rawGET(t *testing.T, url string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, string(body)
}

func TestHandleSPA_DevMode_Returns404Envelope(t *testing.T) {
	ts := newTestServerWithSPA(t, nil)

	// Unknown SPA-style path → envelope 404, matching M1 catch-all behavior.
	status, env := ts.do(t, http.MethodGet, "/anything", nil)
	if status != http.StatusNotFound {
		t.Fatalf("status = %d; want 404", status)
	}
	if env.Error == nil || env.Error.Code != "not_found" {
		t.Fatalf("expected envelope not_found; got %+v", env)
	}
}

func TestHandleSPA_DevMode_HealthAndAPIStillRoute(t *testing.T) {
	ts := newTestServerWithSPA(t, nil)

	// /healthz remains 200 because it has an explicit, more-specific
	// registration that wins over the "/" SPA fallthrough.
	resp, _ := rawGET(t, ts.srv.URL+"/healthz")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/healthz status = %d; want 200", resp.StatusCode)
	}

	// /api/v1/bootstrap accepts the empty body (creates the admin token).
	status, env := ts.do(t, http.MethodPost, "/api/v1/bootstrap", nil)
	if status != http.StatusCreated {
		t.Fatalf("bootstrap status = %d; want 201; env=%+v", status, env)
	}
}

func TestHandleSPA_EmbeddedRootServesIndex(t *testing.T) {
	ts := newTestServerWithSPA(t, fakeSPA())

	resp, body := rawGET(t, ts.srv.URL+"/")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d; want 200", resp.StatusCode)
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "text/html") {
		t.Fatalf("Content-Type = %q; want text/html", resp.Header.Get("Content-Type"))
	}
	if got := resp.Header.Get("Cache-Control"); got != cacheNoStore {
		t.Fatalf("Cache-Control = %q; want %q", got, cacheNoStore)
	}
	if !strings.Contains(body, "<div id=\"root\">") {
		t.Fatalf("body missing SPA shell: %q", body)
	}
	// Nonce placeholder must have been substituted out — it's an
	// invariant the browser depends on for the CSP nonce match.
	if strings.Contains(body, cspNoncePlaceholder) {
		t.Fatalf("CSP nonce placeholder leaked into response body: %q", body)
	}
}

func TestHandleSPA_CSPHeaderCarriesNonce(t *testing.T) {
	ts := newTestServerWithSPA(t, fakeSPA())

	resp, body := rawGET(t, ts.srv.URL+"/")
	csp := resp.Header.Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("Content-Security-Policy header missing")
	}

	// Extract the nonce from the header and confirm the same nonce
	// appears in the substituted body — the round-trip the browser
	// will validate.
	m := regexp.MustCompile(`'nonce-([A-Za-z0-9+/=_-]+)'`).FindStringSubmatch(csp)
	if len(m) != 2 {
		t.Fatalf("CSP has no nonce: %q", csp)
	}
	nonce := m[1]
	if !strings.Contains(body, `nonce="`+nonce+`"`) {
		t.Fatalf("nonce %q from CSP header not present in body: %q", nonce, body)
	}

	// Quick sanity on the rest of the policy — these are load-bearing
	// invariants from buildCSP that the threat model depends on.
	for _, want := range []string{
		"default-src 'self'",
		"frame-src 'none'",
		"object-src 'none'",
		"base-uri 'self'",
	} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP missing %q; got %q", want, csp)
		}
	}
}

func TestHandleSPA_CSPNonceUniquePerRequest(t *testing.T) {
	ts := newTestServerWithSPA(t, fakeSPA())

	nonceFor := func() string {
		resp, _ := rawGET(t, ts.srv.URL+"/")
		csp := resp.Header.Get("Content-Security-Policy")
		m := regexp.MustCompile(`'nonce-([^']+)'`).FindStringSubmatch(csp)
		if len(m) != 2 {
			t.Fatalf("CSP nonce missing in %q", csp)
		}
		return m[1]
	}

	if a, b := nonceFor(), nonceFor(); a == b {
		t.Fatalf("nonce reused across requests: %q", a)
	}
}

func TestHandleSPA_AssetServedWithImmutableCache(t *testing.T) {
	ts := newTestServerWithSPA(t, fakeSPA())

	resp, body := rawGET(t, ts.srv.URL+"/assets/main.js")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d; want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Cache-Control"); got != cacheImmutable {
		t.Fatalf("Cache-Control = %q; want %q", got, cacheImmutable)
	}
	if body != "console.log('hi')" {
		t.Fatalf("body = %q; want main.js content", body)
	}
}

func TestHandleSPA_RootFileShortTTL(t *testing.T) {
	ts := newTestServerWithSPA(t, fakeSPA())

	resp, _ := rawGET(t, ts.srv.URL+"/favicon.ico")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d; want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Cache-Control"); got != cacheShortTTL {
		t.Fatalf("Cache-Control = %q; want %q", got, cacheShortTTL)
	}
}

func TestHandleSPA_UnknownRouteFallsBackToIndex(t *testing.T) {
	ts := newTestServerWithSPA(t, fakeSPA())

	// /login is not a file in the FS but is a valid client-side route.
	// SPA fallback: serve index.html so React Router takes it from there.
	resp, body := rawGET(t, ts.srv.URL+"/login")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d; want 200", resp.StatusCode)
	}
	if !strings.Contains(body, "<div id=\"root\">") {
		t.Fatalf("SPA fallback did not serve index; body = %q", body)
	}
}

func TestHandleSPA_RejectsNonGETMethods(t *testing.T) {
	ts := newTestServerWithSPA(t, fakeSPA())

	// POST to an SPA path → envelope 404 (not 405), matching M1's
	// "no such route" behavior. Method-Not-Allowed would imply the
	// route exists, which is a needless signal to probes.
	status, env := ts.do(t, http.MethodPost, "/", nil)
	if status != http.StatusNotFound {
		t.Fatalf("status = %d; want 404", status)
	}
	if env.Error == nil || env.Error.Code != "not_found" {
		t.Fatalf("expected envelope not_found; got %+v", env)
	}
}

func TestHandleSPA_TraversalCannotLeakOutsideFS(t *testing.T) {
	// fakeSPA() contains only index.html + a couple of assets. A request
	// for a name outside the FS must NOT return arbitrary host content;
	// it must return the SPA shell (the standard fallback for any
	// non-existent path). embed.FS / fs.Sub sandbox this by design —
	// the test pins the invariant so a future custom FS implementation
	// can't regress it.
	ts := newTestServerWithSPA(t, fakeSPA())

	resp, body := rawGET(t, ts.srv.URL+"/this-path-does-not-exist.txt")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d; want 200 (SPA fallback)", resp.StatusCode)
	}
	if !strings.Contains(body, "<div id=\"root\">") {
		t.Fatalf("expected SPA shell; got %q", body)
	}
	// The response must look like the SPA shell, not something else
	// pulled from the OS filesystem.
	if strings.Contains(strings.ToLower(body), "root:x:") {
		t.Fatalf("response leaked OS passwd content: %q", body)
	}
}

func TestHandleSPA_NoIndexHTMLReturns404(t *testing.T) {
	// FS containing only assets (no index.html) — represents the
	// .gitkeep-only state before `make dashboard` has run.
	spa := fstest.MapFS{
		"assets/main.js": &fstest.MapFile{Data: []byte("x")},
	}
	ts := newTestServerWithSPA(t, spa)

	// Asset still serves (it exists).
	if resp, _ := rawGET(t, ts.srv.URL+"/assets/main.js"); resp.StatusCode != http.StatusOK {
		t.Fatalf("asset status = %d; want 200", resp.StatusCode)
	}
	// Root falls through serveSPAIndex → no index.html → envelope 404.
	status, env := ts.do(t, http.MethodGet, "/", nil)
	if status != http.StatusNotFound {
		t.Fatalf("status = %d; want 404", status)
	}
	if env.Error == nil || env.Error.Code != "not_found" {
		t.Fatalf("expected envelope not_found; got %+v", env)
	}
}

func TestHandleSPA_HEADReturnsHeadersNoBody(t *testing.T) {
	ts := newTestServerWithSPA(t, fakeSPA())

	req, err := http.NewRequest(http.MethodHead, ts.srv.URL+"/", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("HEAD status = %d; want 200", resp.StatusCode)
	}
	if resp.Header.Get("Content-Security-Policy") == "" {
		t.Fatal("HEAD response missing CSP header")
	}
	body, _ := io.ReadAll(resp.Body)
	if len(body) != 0 {
		t.Fatalf("HEAD body length = %d; want 0", len(body))
	}
}
