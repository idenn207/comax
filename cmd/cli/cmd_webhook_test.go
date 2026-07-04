package main

import (
	"bytes"
	"strings"
	"testing"
)

// runWebhookCmd runs a `secret webhook ...` invocation capturing stdout and
// stderr separately — create prints the signing secret to stdout and guidance
// to stderr, so the split matters.
func runWebhookCmd(t *testing.T, credPath string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := newRootCmd()
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetArgs(append([]string{"--credentials", credPath}, args...))
	err = root.Execute()
	return out.String(), errb.String(), err
}

// webhookIDForURL scans a `webhook list` table for the row whose URL column
// matches url and returns its id column.
func webhookIDForURL(listOut, url string) string {
	for _, line := range strings.Split(listOut, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[2] == url {
			return fields[0]
		}
	}
	return ""
}

func TestWebhook_CreatePrintsSecretOnceWithGuidance(t *testing.T) {
	credPath, _ := loggedInWorktree(t)

	stdout, stderr, err := runWebhookCmd(t, credPath,
		"webhook", "create", "--project", "comax", "--env", "dev",
		"--url", "http://10.1.2.3/hook", "--events", "secret.upsert")
	if err != nil {
		t.Fatalf("webhook create: %v (stderr=%s)", err, stderr)
	}
	if secret := strings.TrimSpace(stdout); secret == "" {
		t.Fatal("webhook create did not print a signing secret to stdout")
	}
	if !strings.Contains(stderr, "shown once") {
		t.Errorf("stderr missing guidance; got %q", stderr)
	}
}

func TestWebhook_CreateRequiresProjectAndURL(t *testing.T) {
	credPath, _ := loggedInWorktree(t)
	if _, _, err := runWebhookCmd(t, credPath, "webhook", "create", "--url", "http://10.0.0.1/h"); err == nil {
		t.Error("create without --project returned nil; want error")
	}
	if _, _, err := runWebhookCmd(t, credPath, "webhook", "create", "--project", "comax"); err == nil {
		t.Error("create without --url returned nil; want error")
	}
}

func TestWebhook_CreateRejectsBlockedURL(t *testing.T) {
	credPath, _ := loggedInWorktree(t)
	// The server SSRF-rejects the metadata IP → the CLI surfaces the error.
	if _, _, err := runWebhookCmd(t, credPath,
		"webhook", "create", "--project", "comax", "--url", "http://169.254.169.254/latest"); err == nil {
		t.Error("create with metadata URL returned nil; want error")
	}
}

// webhookEnabledForURL scans a `webhook list` table for the row whose URL
// column matches url and returns its ENABLED column (field 4).
func webhookEnabledForURL(listOut, url string) string {
	for _, line := range strings.Split(listOut, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 5 && fields[2] == url {
			return fields[4]
		}
	}
	return ""
}

func TestWebhook_EnableDisableRoundTrip(t *testing.T) {
	credPath, _ := loggedInWorktree(t)
	const url = "http://10.7.8.9/hook"
	if _, _, err := runWebhookCmd(t, credPath, "webhook", "create", "--project", "comax", "--url", url); err != nil {
		t.Fatalf("seed webhook: %v", err)
	}
	listOut, _, _ := runWebhookCmd(t, credPath, "webhook", "list")
	id := webhookIDForURL(listOut, url)
	if id == "" {
		t.Fatalf("could not find webhook id in list:\n%s", listOut)
	}
	if got := webhookEnabledForURL(listOut, url); got != "yes" {
		t.Fatalf("new webhook ENABLED = %q; want yes", got)
	}

	// Disable → stderr confirms, listing flips to no.
	_, stderr, err := runWebhookCmd(t, credPath, "webhook", "disable", "--id", id)
	if err != nil {
		t.Fatalf("webhook disable: %v (stderr=%s)", err, stderr)
	}
	if !strings.Contains(stderr, "Disabled webhook") {
		t.Errorf("disable stderr = %q", stderr)
	}
	listOut, _, _ = runWebhookCmd(t, credPath, "webhook", "list")
	if got := webhookEnabledForURL(listOut, url); got != "no" {
		t.Errorf("ENABLED after disable = %q; want no", got)
	}

	// Re-enable → stderr confirms, listing flips back to yes.
	_, stderr, err = runWebhookCmd(t, credPath, "webhook", "enable", "--id", id)
	if err != nil {
		t.Fatalf("webhook enable: %v (stderr=%s)", err, stderr)
	}
	if !strings.Contains(stderr, "Enabled webhook") {
		t.Errorf("enable stderr = %q", stderr)
	}
	listOut, _, _ = runWebhookCmd(t, credPath, "webhook", "list")
	if got := webhookEnabledForURL(listOut, url); got != "yes" {
		t.Errorf("ENABLED after enable = %q; want yes", got)
	}

	// enable/disable require --id.
	if _, _, err := runWebhookCmd(t, credPath, "webhook", "disable"); err == nil {
		t.Error("disable without --id returned nil; want error")
	}
}

func TestWebhook_ListAndDelete(t *testing.T) {
	credPath, _ := loggedInWorktree(t)
	if _, _, err := runWebhookCmd(t, credPath,
		"webhook", "create", "--project", "comax", "--url", "http://10.4.5.6/hook"); err != nil {
		t.Fatalf("seed webhook: %v", err)
	}

	listOut, _, err := runWebhookCmd(t, credPath, "webhook", "list")
	if err != nil {
		t.Fatalf("webhook list: %v", err)
	}
	if !strings.Contains(listOut, "http://10.4.5.6/hook") {
		t.Fatalf("list missing the created webhook:\n%s", listOut)
	}
	id := webhookIDForURL(listOut, "http://10.4.5.6/hook")
	if id == "" {
		t.Fatalf("could not find webhook id in list:\n%s", listOut)
	}

	// Deliveries endpoint is reachable (empty until the worker runs).
	if _, _, err := runWebhookCmd(t, credPath, "webhook", "deliveries", "--id", id); err != nil {
		t.Errorf("webhook deliveries: %v", err)
	}

	_, stderr, err := runWebhookCmd(t, credPath, "webhook", "delete", "--id", id)
	if err != nil {
		t.Fatalf("webhook delete: %v", err)
	}
	if !strings.Contains(stderr, "Deleted webhook") {
		t.Errorf("delete stderr = %q", stderr)
	}

	// After deletion it is gone from the list.
	listOut, _, _ = runWebhookCmd(t, credPath, "webhook", "list")
	if strings.Contains(listOut, "http://10.4.5.6/hook") {
		t.Errorf("webhook still listed after delete:\n%s", listOut)
	}
}
