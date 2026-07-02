package server

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/idenn207/comax-secrets/internal/store"
)

// seedSecret PUTs a value at /api/v1/projects/{proj}/envs/{env}/secrets/{key}
// and fatals if the request does not succeed. Used to set up scenarios
// where the M2 endpoints exercise existing data.
func seedSecret(t *testing.T, ts *testServer, proj, env, key, value string) {
	t.Helper()
	path := "/api/v1/projects/" + proj + "/envs/" + env + "/secrets/" + key
	status, e := ts.do(t, http.MethodPut, path, map[string]string{"value": value})
	if status != http.StatusOK && status != http.StatusCreated {
		t.Fatalf("seed PUT %s: status=%d env=%+v", path, status, e)
	}
}

// ---- GET .../versions/{v} ---------------------------------------------

func TestGetVersion_ReturnsDecryptedHistoricalValue(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "dev"})
	seedSecret(t, ts, "app", "dev", "DB_URL", "value-v1")
	seedSecret(t, ts, "app", "dev", "DB_URL", "value-v2")

	status, env := ts.do(t, http.MethodGet,
		"/api/v1/projects/app/envs/dev/secrets/DB_URL/versions/1", nil)
	if status != http.StatusOK {
		t.Fatalf("status=%d env=%+v", status, env)
	}
	got := env.Data.(map[string]any)
	if got["value"].(string) != "value-v1" {
		t.Errorf("value=%v; want value-v1", got["value"])
	}
	if got["version"].(float64) != 1 {
		t.Errorf("version=%v; want 1", got["version"])
	}
}

func TestGetVersion_404OnUnknownVersion(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "dev"})
	seedSecret(t, ts, "app", "dev", "K", "v")

	status, e := ts.do(t, http.MethodGet,
		"/api/v1/projects/app/envs/dev/secrets/K/versions/99", nil)
	if status != http.StatusNotFound {
		t.Errorf("status=%d; want 404", status)
	}
	if e.Error == nil || e.Error.Code != "version_not_found" {
		t.Errorf("error=%+v; want code=version_not_found", e.Error)
	}
}

func TestGetVersion_BadVersionInURL(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "dev"})
	seedSecret(t, ts, "app", "dev", "K", "v")

	status, _ := ts.do(t, http.MethodGet,
		"/api/v1/projects/app/envs/dev/secrets/K/versions/abc", nil)
	if status != http.StatusBadRequest {
		t.Errorf("status=%d; want 400", status)
	}
}

// ---- GET .../diff ------------------------------------------------------

func TestDiffEnvs_AddedRemovedChanged(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "local"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "prod"})

	// local has SHARED=local, ONLY_LOCAL=x
	seedSecret(t, ts, "app", "local", "SHARED", "local-value")
	seedSecret(t, ts, "app", "local", "ONLY_LOCAL", "x")
	// prod has SHARED=prod, ONLY_PROD=y
	seedSecret(t, ts, "app", "prod", "SHARED", "prod-value")
	seedSecret(t, ts, "app", "prod", "ONLY_PROD", "y")

	status, env := ts.do(t, http.MethodGet,
		"/api/v1/projects/app/envs/local/diff?against=prod", nil)
	if status != http.StatusOK {
		t.Fatalf("status=%d env=%+v", status, env)
	}
	data := env.Data.(map[string]any)
	if data["lhs"].(string) != "local" || data["rhs"].(string) != "prod" {
		t.Errorf("sides: lhs=%v rhs=%v", data["lhs"], data["rhs"])
	}
	added := data["added"].([]any)
	if len(added) != 1 || added[0].(string) != "ONLY_LOCAL" {
		t.Errorf("added=%v; want [ONLY_LOCAL]", added)
	}
	removed := data["removed"].([]any)
	if len(removed) != 1 || removed[0].(string) != "ONLY_PROD" {
		t.Errorf("removed=%v; want [ONLY_PROD]", removed)
	}
	changed := data["changed"].([]any)
	if len(changed) != 1 {
		t.Fatalf("changed len=%d; want 1", len(changed))
	}
	c := changed[0].(map[string]any)
	if c["key"].(string) != "SHARED" {
		t.Errorf("changed key=%v; want SHARED", c["key"])
	}
}

func TestDiffEnvs_RejectsSelfDiff(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "dev"})

	status, e := ts.do(t, http.MethodGet,
		"/api/v1/projects/app/envs/dev/diff?against=dev", nil)
	if status != http.StatusBadRequest {
		t.Errorf("status=%d; want 400", status)
	}
	if e.Error == nil || e.Error.Code != "bad_request" {
		t.Errorf("error=%+v; want bad_request", e.Error)
	}
}

func TestDiffEnvs_404OnUnknownAgainst(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "dev"})

	status, _ := ts.do(t, http.MethodGet,
		"/api/v1/projects/app/envs/dev/diff?against=ghost", nil)
	if status != http.StatusNotFound {
		t.Errorf("status=%d; want 404", status)
	}
}

// ---- POST .../rollback -------------------------------------------------

func TestRollbackSecret_AppendsNewVersionWithOldValue(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "dev"})
	seedSecret(t, ts, "app", "dev", "DB_URL", "v1")
	seedSecret(t, ts, "app", "dev", "DB_URL", "v2")

	status, env := ts.do(t, http.MethodPost,
		"/api/v1/projects/app/envs/dev/secrets/DB_URL/rollback",
		map[string]int64{"target_version": 1})
	if status != http.StatusOK {
		t.Fatalf("rollback status=%d env=%+v", status, env)
	}
	got := env.Data.(map[string]any)
	if got["version"].(float64) != 3 {
		t.Errorf("rollback version=%v; want 3 (continues sequence)", got["version"])
	}
	if got["value"].(string) != "v1" {
		t.Errorf("rollback value=%v; want v1", got["value"])
	}

	// Subsequent GET returns the rolled-back value at version 3.
	status, env = ts.do(t, http.MethodGet,
		"/api/v1/projects/app/envs/dev/secrets/DB_URL", nil)
	if status != http.StatusOK {
		t.Fatalf("get status=%d", status)
	}
	if env.Data.(map[string]any)["value"].(string) != "v1" {
		t.Errorf("post-rollback value=%v; want v1", env.Data.(map[string]any)["value"])
	}

	// Audit row is appended.
	entries, err := store.NewAuditRepo(ts.db).List(context.Background(),
		store.AuditFilter{Action: "secret.rollback"}, 10)
	if err != nil {
		t.Fatalf("audit list: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("audit rows=%d; want 1", len(entries))
	}
	if !strings.Contains(entries[0].Target, "from_version=2") ||
		!strings.Contains(entries[0].Target, "to_version=1") {
		t.Errorf("audit target=%q; want from_version/to_version", entries[0].Target)
	}
}

func TestRollbackSecret_404OnUnknownTargetVersion(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "dev"})
	seedSecret(t, ts, "app", "dev", "K", "v1")

	status, e := ts.do(t, http.MethodPost,
		"/api/v1/projects/app/envs/dev/secrets/K/rollback",
		map[string]int64{"target_version": 99})
	if status != http.StatusNotFound {
		t.Errorf("status=%d; want 404", status)
	}
	if e.Error == nil || e.Error.Code != "version_not_found" {
		t.Errorf("error=%+v; want version_not_found", e.Error)
	}
}

func TestRollbackSecret_BadBody(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "dev"})
	seedSecret(t, ts, "app", "dev", "K", "v1")

	cases := []struct {
		name string
		body any
	}{
		{"missing target_version", map[string]string{}},
		{"zero target_version", map[string]int64{"target_version": 0}},
	}
	for _, c := range cases {
		status, _ := ts.do(t, http.MethodPost,
			"/api/v1/projects/app/envs/dev/secrets/K/rollback", c.body)
		if status != http.StatusBadRequest {
			t.Errorf("%s: status=%d; want 400", c.name, status)
		}
	}
}

func TestRollbackSecret_RefusesSameAsCurrent(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "dev"})
	seedSecret(t, ts, "app", "dev", "K", "v1")

	// current version is 1; asking to roll back to 1 is a no-op.
	status, _ := ts.do(t, http.MethodPost,
		"/api/v1/projects/app/envs/dev/secrets/K/rollback",
		map[string]int64{"target_version": 1})
	if status != http.StatusBadRequest {
		t.Errorf("status=%d; want 400 (already current)", status)
	}
}

// ---- DELETE .../secrets/{k} -------------------------------------------

func TestDeleteSecret_RemovesFromReadsButKeepsHistory(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "dev"})
	seedSecret(t, ts, "app", "dev", "K", "v1")
	seedSecret(t, ts, "app", "dev", "K", "v2")

	status, _ := ts.do(t, http.MethodDelete,
		"/api/v1/projects/app/envs/dev/secrets/K", nil)
	if status != http.StatusNoContent {
		t.Fatalf("delete status=%d; want 204", status)
	}

	// GET secret returns 404.
	status, _ = ts.do(t, http.MethodGet,
		"/api/v1/projects/app/envs/dev/secrets/K", nil)
	if status != http.StatusNotFound {
		t.Errorf("post-delete GET status=%d; want 404", status)
	}

	// Versions list endpoint still surfaces the timeline.
	status, env := ts.do(t, http.MethodGet,
		"/api/v1/projects/app/envs/dev/versions", nil)
	if status != http.StatusOK {
		t.Fatalf("versions list status=%d", status)
	}
	versions := env.Data.([]any)
	if len(versions) != 2 {
		t.Errorf("versions after delete = %d; want 2 (history preserved)", len(versions))
	}

	// Per-version endpoint still serves the decrypted historical value.
	status, env = ts.do(t, http.MethodGet,
		"/api/v1/projects/app/envs/dev/secrets/K/versions/1", nil)
	if status != http.StatusOK {
		t.Fatalf("get version after delete status=%d", status)
	}
	if env.Data.(map[string]any)["value"].(string) != "v1" {
		t.Errorf("historical value lost after delete")
	}

	// Audit row appended for the delete itself.
	entries, err := store.NewAuditRepo(ts.db).List(context.Background(),
		store.AuditFilter{Action: "secret.delete"}, 10)
	if err != nil {
		t.Fatalf("audit list: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("delete audit rows=%d; want 1", len(entries))
	}
}

func TestDeleteSecret_404OnUnknownKey(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "dev"})

	status, _ := ts.do(t, http.MethodDelete,
		"/api/v1/projects/app/envs/dev/secrets/ghost", nil)
	if status != http.StatusNotFound {
		t.Errorf("status=%d; want 404", status)
	}
}

func TestDeleteSecret_ReactivatedByPut(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "dev"})
	seedSecret(t, ts, "app", "dev", "K", "v1")

	if status, _ := ts.do(t, http.MethodDelete,
		"/api/v1/projects/app/envs/dev/secrets/K", nil); status != http.StatusNoContent {
		t.Fatalf("delete status=%d", status)
	}
	// Re-PUT reactivates without operator needing to know about deleted_at.
	seedSecret(t, ts, "app", "dev", "K", "v2-after-delete")

	status, env := ts.do(t, http.MethodGet,
		"/api/v1/projects/app/envs/dev/secrets/K", nil)
	if status != http.StatusOK {
		t.Fatalf("get after re-PUT status=%d", status)
	}
	got := env.Data.(map[string]any)
	if got["value"].(string) != "v2-after-delete" {
		t.Errorf("post-reactivation value=%v", got["value"])
	}
	if got["version"].(float64) != 2 {
		t.Errorf("post-reactivation version=%v; want 2 (sequence continues)", got["version"])
	}
}

// ---- GET /api/v1/audit -------------------------------------------------

func TestListAudit_ReturnsEntriesNewestFirst(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)                                                                  // +1 audit
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})          // +1
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "dev"}) // +1
	seedSecret(t, ts, "app", "dev", "K", "v")                                        // +1

	status, env := ts.do(t, http.MethodGet, "/api/v1/audit", nil)
	if status != http.StatusOK {
		t.Fatalf("status=%d env=%+v", status, env)
	}
	data := env.Data.([]any)
	if len(data) != 4 {
		t.Errorf("audit rows=%d; want 4", len(data))
	}
	// Newest first — secret.upsert was the last write.
	if data[0].(map[string]any)["action"].(string) != "secret.upsert" {
		t.Errorf("newest action=%v; want secret.upsert", data[0].(map[string]any)["action"])
	}
}

func TestListAudit_FiltersByAction(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "dev"})
	seedSecret(t, ts, "app", "dev", "K", "v")

	status, env := ts.do(t, http.MethodGet,
		"/api/v1/audit?action=secret.upsert", nil)
	if status != http.StatusOK {
		t.Fatalf("status=%d", status)
	}
	data := env.Data.([]any)
	if len(data) != 1 || data[0].(map[string]any)["action"].(string) != "secret.upsert" {
		t.Errorf("filter result=%+v", data)
	}
}

func TestListAudit_LimitAndCursor(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "app"})
	mustCreate(t, ts, "/api/v1/projects/app/envs", map[string]string{"name": "dev"})
	for i := 0; i < 5; i++ {
		seedSecret(t, ts, "app", "dev", "K", "v")
	}

	status, env := ts.do(t, http.MethodGet, "/api/v1/audit?limit=2", nil)
	if status != http.StatusOK {
		t.Fatalf("status=%d", status)
	}
	page1 := env.Data.([]any)
	if len(page1) != 2 {
		t.Errorf("page1 len=%d; want 2", len(page1))
	}
	meta := env.Meta.(map[string]any)
	cursor, ok := meta["next_before"].(float64)
	if !ok || cursor == 0 {
		t.Fatalf("next_before missing: %+v", meta)
	}

	// Fetch next page using the cursor.
	status, env = ts.do(t, http.MethodGet,
		"/api/v1/audit?limit=2&before="+formatInt64(int64(cursor)), nil)
	if status != http.StatusOK {
		t.Fatalf("page2 status=%d", status)
	}
	page2 := env.Data.([]any)
	if len(page2) != 2 {
		t.Errorf("page2 len=%d; want 2", len(page2))
	}
	// Pages must not overlap.
	last := int64(page1[len(page1)-1].(map[string]any)["id"].(float64))
	first := int64(page2[0].(map[string]any)["id"].(float64))
	if first >= last {
		t.Errorf("page2[0].id=%d >= page1.last=%d (cursor leaked or reverse)", first, last)
	}
}

func TestListAudit_RejectsBadParams(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	cases := []struct {
		name, qs string
	}{
		{"non-integer actor", "actor=abc"},
		{"negative before", "before=-1"},
		{"zero limit", "limit=0"},
		{"invalid project name", "project=bad/name"},
	}
	for _, c := range cases {
		status, _ := ts.do(t, http.MethodGet, "/api/v1/audit?"+c.qs, nil)
		if status != http.StatusBadRequest {
			t.Errorf("%s: status=%d; want 400", c.name, status)
		}
	}
}

// formatInt64 avoids pulling strconv into the test file just for one call.
func formatInt64(v int64) string {
	if v == 0 {
		return "0"
	}
	digits := []byte{}
	for v > 0 {
		digits = append([]byte{byte('0' + v%10)}, digits...)
		v /= 10
	}
	return string(digits)
}
