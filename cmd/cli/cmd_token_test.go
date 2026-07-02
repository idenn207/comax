package main

import (
	"bytes"
	"strings"
	"testing"
)

// runTokenCmd runs a `secret token ...` invocation capturing stdout and
// stderr separately. token create prints the plaintext to stdout and the
// guidance to stderr, so the split matters.
func runTokenCmd(t *testing.T, credPath string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := newRootCmd()
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetArgs(append([]string{"--credentials", credPath}, args...))
	err = root.Execute()
	return out.String(), errb.String(), err
}

// idForName extracts the id column of the tabwriter row whose NAME field
// equals name, or "" if not found.
func idForName(listOut, name string) string {
	for _, line := range strings.Split(listOut, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == name {
			return fields[0]
		}
	}
	return ""
}

func TestToken_CreatePrintsPlaintextOnceWithGuidance(t *testing.T) {
	credPath, _ := loggedInWorktree(t)

	stdout, stderr, err := runTokenCmd(t, credPath, "token", "create", "--name", "ci")
	if err != nil {
		t.Fatalf("token create: %v (stderr=%s)", err, stderr)
	}
	if plain := strings.TrimSpace(stdout); plain == "" {
		t.Fatal("token create did not print a plaintext token to stdout")
	}
	if !strings.Contains(stderr, "shown once") {
		t.Errorf("token create stderr missing guidance; got %q", stderr)
	}
}

func TestToken_CreateRequiresName(t *testing.T) {
	credPath, _ := loggedInWorktree(t)
	if _, _, err := runTokenCmd(t, credPath, "token", "create"); err == nil {
		t.Error("token create without --name returned nil; want error")
	}
}

func TestToken_ListShowsAdminAndIssuedTokens(t *testing.T) {
	credPath, _ := loggedInWorktree(t)
	if _, _, err := runTokenCmd(t, credPath, "token", "create", "--name", "ci"); err != nil {
		t.Fatalf("seed token: %v", err)
	}
	stdout, _, err := runTokenCmd(t, credPath, "token", "list")
	if err != nil {
		t.Fatalf("token list: %v", err)
	}
	if !strings.Contains(stdout, "bootstrap") || !strings.Contains(stdout, "ci") {
		t.Errorf("token list missing bootstrap/ci; got:\n%s", stdout)
	}
	// The bootstrap token is admin=yes; the issued ci token is admin=no.
	if !strings.Contains(stdout, "yes") || !strings.Contains(stdout, "no") {
		t.Errorf("token list missing admin flags; got:\n%s", stdout)
	}
}

func TestToken_RevokeFlipsStatusAndIsNotIdempotent(t *testing.T) {
	credPath, _ := loggedInWorktree(t)
	if _, _, err := runTokenCmd(t, credPath, "token", "create", "--name", "ci"); err != nil {
		t.Fatalf("seed token: %v", err)
	}
	listOut, _, err := runTokenCmd(t, credPath, "token", "list")
	if err != nil {
		t.Fatalf("token list: %v", err)
	}
	ciID := idForName(listOut, "ci")
	if ciID == "" {
		t.Fatalf("could not find ci token id in list:\n%s", listOut)
	}

	// Revoke succeeds.
	if _, stderr, err := runTokenCmd(t, credPath, "token", "revoke", "--id", ciID); err != nil {
		t.Fatalf("token revoke: %v (stderr=%s)", err, stderr)
	}
	// The status flips to revoked on the next listing.
	listOut, _, err = runTokenCmd(t, credPath, "token", "list")
	if err != nil {
		t.Fatalf("token list after revoke: %v", err)
	}
	for _, line := range strings.Split(listOut, "\n") {
		if fields := strings.Fields(line); len(fields) >= 2 && fields[1] == "ci" {
			if !strings.Contains(line, "revoked") {
				t.Errorf("ci row not marked revoked: %q", line)
			}
		}
	}
	// A second revoke of the same id is a 404 (not idempotent by design).
	if _, _, err := runTokenCmd(t, credPath, "token", "revoke", "--id", ciID); err == nil {
		t.Error("double revoke returned nil; want error (404)")
	}
}

func TestToken_RevokeRequiresID(t *testing.T) {
	credPath, _ := loggedInWorktree(t)
	if _, _, err := runTokenCmd(t, credPath, "token", "revoke"); err == nil {
		t.Error("token revoke without --id returned nil; want error")
	}
}
