package server

import (
	"net/http"
	"testing"
)

// The tests in this file go after the genuinely-rare error branches that
// happy-path tests can't reach: tx commit failure, audit append failure,
// store-write failure, and decrypt failure. The lever they pull is to
// tamper with the *sql.DB underneath a live server — either by dropping
// a table (audit_log / secrets) so the handler's last write step fails,
// or by overwriting a stored ciphertext so the next decrypt rejects it.
//
// Why this is safe: each test owns its own t.TempDir() database, so the
// tamper never escapes the test. The auth middleware reads only the
// service_tokens table, which we leave intact, so the handlers still
// reach the targeted branch instead of being blocked at auth.

// ---- audit append failure (DROP TABLE audit_log) ----------------------
//
// Every mutating handler ends with `appendAudit` inside its transaction.
// Dropping audit_log after bootstrap+seed forces that step to fail and
// the request to return the "audit failed" 500 envelope.

func dropAudit(t *testing.T, ts *testServer) {
	t.Helper()
	if _, err := ts.db.Exec("DROP TABLE audit_log"); err != nil {
		t.Fatalf("drop audit_log: %v", err)
	}
}

func TestCreateProject_AuditFailureIs500(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	dropAudit(t, ts)

	status, _ := ts.do(t, http.MethodPost, "/api/v1/projects",
		map[string]string{"name": "app"})
	if status != http.StatusInternalServerError {
		t.Errorf("status=%d; want 500 (audit failed)", status)
	}
}

func TestCreateEnv_AuditFailureIs500(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	dropAudit(t, ts)

	status, _ := ts.do(t, http.MethodPost, "/api/v1/projects/app/envs",
		map[string]string{"name": "dev"})
	if status != http.StatusInternalServerError {
		t.Errorf("status=%d; want 500 (audit failed)", status)
	}
}

func TestPutSecret_AuditFailureIs500(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "dev"})
	dropAudit(t, ts)

	status, _ := ts.do(t, http.MethodPut,
		"/api/v1/projects/app/envs/dev/secrets/K",
		map[string]string{"value": "v"})
	if status != http.StatusInternalServerError {
		t.Errorf("status=%d; want 500 (audit failed)", status)
	}
}

func TestRollbackSecret_AuditFailureIs500(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "dev"})
	seedSecret(t, ts, "app", "dev", "K", "v1")
	seedSecret(t, ts, "app", "dev", "K", "v2")
	dropAudit(t, ts)

	status, _ := ts.do(t, http.MethodPost,
		"/api/v1/projects/app/envs/dev/secrets/K/rollback",
		map[string]int64{"target_version": 1})
	if status != http.StatusInternalServerError {
		t.Errorf("status=%d; want 500 (audit failed)", status)
	}
}

func TestDeleteSecret_AuditFailureIs500(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "dev"})
	seedSecret(t, ts, "app", "dev", "K", "v1")
	dropAudit(t, ts)

	status, _ := ts.do(t, http.MethodDelete,
		"/api/v1/projects/app/envs/dev/secrets/K", nil)
	if status != http.StatusInternalServerError {
		t.Errorf("status=%d; want 500 (audit failed)", status)
	}
}

// ---- store write failure (DROP TABLE secret_versions) ----------------
//
// handlePutSecret's append-version step depends on secret_versions
// existing. Dropping it after the secrets row is created lets us drive
// the "version append failed" 500 branch independently of audit.

func TestPutSecret_VersionAppendFailureIs500(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "dev"})

	if _, err := ts.db.Exec("DROP TABLE secret_versions"); err != nil {
		t.Fatalf("drop secret_versions: %v", err)
	}

	status, _ := ts.do(t, http.MethodPut,
		"/api/v1/projects/app/envs/dev/secrets/K",
		map[string]string{"value": "v"})
	if status != http.StatusInternalServerError {
		t.Errorf("status=%d; want 500 (version append failed)", status)
	}
}

// ---- decrypt failure (tamper stored ciphertext) ----------------------
//
// handleGetVersion and handleRollbackSecret both call crypto.Open on the
// ciphertext fetched from secret_versions. Overwriting that ciphertext
// with random bytes forces AEAD authentication to fail and the handler
// to take its "decrypt failed" 500 branch — without us needing to swap
// the master key out from under a live server.

func tamperCiphertext(t *testing.T, ts *testServer, version int64) {
	t.Helper()
	res, err := ts.db.Exec(
		"UPDATE secret_versions SET ciphertext = ? WHERE version = ?",
		[]byte("not-a-valid-AEAD-blob"), version,
	)
	if err != nil {
		t.Fatalf("tamper ciphertext: %v", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		t.Fatalf("tamper ciphertext: no rows affected (version=%d not found)", version)
	}
}

func TestGetVersion_DecryptFailureIs500(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "dev"})
	seedSecret(t, ts, "app", "dev", "K", "v1")

	tamperCiphertext(t, ts, 1)

	status, env := ts.do(t, http.MethodGet,
		"/api/v1/projects/app/envs/dev/secrets/K/versions/1", nil)
	if status != http.StatusInternalServerError {
		t.Errorf("status=%d; want 500 (decrypt failed)", status)
	}
	if env.Error == nil || env.Error.Code != "internal" {
		t.Errorf("error=%+v; want code=internal", env.Error)
	}
}

func TestRollbackSecret_DecryptFailureIs500(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "dev"})
	seedSecret(t, ts, "app", "dev", "K", "v1")
	seedSecret(t, ts, "app", "dev", "K", "v2")

	// Tamper with version 1, which is the rollback target.
	tamperCiphertext(t, ts, 1)

	status, _ := ts.do(t, http.MethodPost,
		"/api/v1/projects/app/envs/dev/secrets/K/rollback",
		map[string]int64{"target_version": 1})
	if status != http.StatusInternalServerError {
		t.Errorf("status=%d; want 500 (decrypt failed)", status)
	}
}

// ---- handleListProjects / handleListEnvs list error -----------------
//
// Dropping the underlying table forces ProjectRepo.List / EnvRepo.List
// to surface a "no such table" error, hitting the handler's 500 path.

func TestListProjects_ListErrorIs500(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	// auth middleware doesn't touch the projects table, so it still
	// passes — only the handler's List call breaks.
	if _, err := ts.db.Exec("DROP TABLE projects"); err != nil {
		t.Fatalf("drop projects: %v", err)
	}

	status, _ := ts.do(t, http.MethodGet, "/api/v1/projects", nil)
	if status != http.StatusInternalServerError {
		t.Errorf("status=%d; want 500", status)
	}
}

func TestListEnvs_ListErrorIs500(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})

	if _, err := ts.db.Exec("DROP TABLE environments"); err != nil {
		t.Fatalf("drop environments: %v", err)
	}

	status, _ := ts.do(t, http.MethodGet, "/api/v1/projects/app/envs", nil)
	if status != http.StatusInternalServerError {
		t.Errorf("status=%d; want 500", status)
	}
}
