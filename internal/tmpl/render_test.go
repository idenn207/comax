package tmpl

import (
	"strings"
	"testing"
)

func TestReferences(t *testing.T) {
	refs := References("a ${{ self.X }} b ${{ shared.Y }} c")
	if len(refs) != 2 {
		t.Fatalf("want 2 refs, got %d", len(refs))
	}
	if refs[0].Env != "self" || refs[0].Key != "X" {
		t.Errorf("ref0 = %v; want self.X", refs[0])
	}
	if refs[1].Env != "shared" || refs[1].Key != "Y" {
		t.Errorf("ref1 = %v; want shared.Y", refs[1])
	}
	if References("no refs here $host literal") != nil {
		t.Error("expected nil for a template with no ${{ }} refs")
	}
}

func TestEnvs_ResolvesSelf(t *testing.T) {
	got := Envs("${{ self.A }} ${{ shared.B }} ${{ self.C }}", "prod")
	// self -> prod, plus shared, distinct + sorted.
	want := []string{"prod", "shared"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("Envs = %v; want %v", got, want)
	}
}

// TestRender_SingleTemplateAcrossEnvs is the F1 core evidence: ONE template
// renders to different output per env via the self alias, with no env
// hardcoded in the template.
func TestRender_SingleTemplateAcrossEnvs(t *testing.T) {
	tmpl := "maxmemory ${{ self.MAXMEM }}\n"
	snaps := map[string]map[string]string{
		"local": {"MAXMEM": "64mb"},
		"dev":   {"MAXMEM": "256mb"},
		"prod":  {"MAXMEM": "2gb"},
	}
	cases := map[string]string{"local": "64mb", "dev": "256mb", "prod": "2gb"}
	for env, want := range cases {
		out, missing, err := Render(tmpl, env, map[string]map[string]string{env: snaps[env]})
		if err != nil || missing != nil {
			t.Fatalf("env=%s: err=%v missing=%v", env, err, missing)
		}
		if got := strings.TrimSpace(out); got != "maxmemory "+want {
			t.Errorf("env=%s rendered %q; want maxmemory %s", env, got, want)
		}
	}
}

func TestRender_CrossEnv(t *testing.T) {
	out, missing, err := Render(
		"upstream ${{ shared.HOST }} pass ${{ self.PW }}", "dev",
		map[string]map[string]string{
			"dev":    {"PW": "s3cret"},
			"shared": {"HOST": "10.0.0.1"},
		})
	if err != nil || missing != nil {
		t.Fatalf("err=%v missing=%v", err, missing)
	}
	if out != "upstream 10.0.0.1 pass s3cret" {
		t.Errorf("rendered %q", out)
	}
}

func TestRender_LiteralDollarPreserved(t *testing.T) {
	// nginx config idioms must survive verbatim — only ${{ }} is a ref.
	in := "log_format main '$remote_addr $host';\nssl ${{ self.CERT }};"
	out, _, err := Render(in, "prod", map[string]map[string]string{"prod": {"CERT": "/etc/tls/x.pem"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "$remote_addr $host") {
		t.Errorf("literal $ not preserved: %q", out)
	}
	if !strings.Contains(out, "ssl /etc/tls/x.pem;") {
		t.Errorf("ref not substituted: %q", out)
	}
}

func TestRender_MultilineValue(t *testing.T) {
	cert := "-----BEGIN CERT-----\nline1\nline2\n-----END CERT-----"
	out, _, err := Render("cert ${{ self.PEM }} end", "prod",
		map[string]map[string]string{"prod": {"PEM": cert}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, cert) {
		t.Errorf("multiline value mangled: %q", out)
	}
}

func TestRender_NoRefsPassThrough(t *testing.T) {
	in := "plain redis.conf\nmaxmemory 64mb\n"
	out, missing, err := Render(in, "dev", nil)
	if err != nil || missing != nil {
		t.Fatalf("err=%v missing=%v", err, missing)
	}
	if out != in {
		t.Errorf("passthrough changed content: %q", out)
	}
}

func TestRender_MissingKeyFailsClosed(t *testing.T) {
	out, missing, err := Render("${{ self.PRESENT }} ${{ self.ABSENT }}", "dev",
		map[string]map[string]string{"dev": {"PRESENT": "ok"}})
	if err == nil {
		t.Fatal("expected fail-closed error for missing key")
	}
	if out != "" {
		t.Errorf("expected empty output on failure, got %q", out)
	}
	if len(missing) != 1 || missing[0].Key != "ABSENT" {
		t.Errorf("missing = %v; want [self.ABSENT]", missing)
	}
}

func TestRender_MissingEnvFailsClosed(t *testing.T) {
	_, missing, err := Render("${{ other.K }}", "dev",
		map[string]map[string]string{"dev": {}})
	if err == nil {
		t.Fatal("expected fail-closed error for missing env")
	}
	if len(missing) != 1 || missing[0].Env != "other" {
		t.Errorf("missing = %v; want [other.K]", missing)
	}
}

func TestRender_CurrentEnvNamedSelfRejected(t *testing.T) {
	_, _, err := Render("${{ self.K }}", "self", nil)
	if err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("expected reserved-alias error, got %v", err)
	}
}

// TestRender_NoSecretsInErrorMessages enforces CLAUDE.md's hard rule that
// secrets never appear in logs/errors (sec#2/#10). A canary value seeded
// under a present key must never surface even when the render fails on a
// sibling missing key.
func TestRender_NoSecretsInErrorMessages(t *testing.T) {
	const canary = "SUPER-SECRET-CANARY-9d3f"
	_, _, err := Render(
		"pass ${{ self.PW }} host ${{ self.MISSING }}", "prod",
		map[string]map[string]string{"prod": {"PW": canary}})
	if err == nil {
		t.Fatal("expected error (missing key)")
	}
	if strings.Contains(err.Error(), canary) {
		t.Fatalf("secret value leaked into error: %q", err.Error())
	}
	// The error should still name the offending reference.
	if !strings.Contains(err.Error(), "self.MISSING") {
		t.Errorf("error should name the missing ref: %q", err.Error())
	}
}
