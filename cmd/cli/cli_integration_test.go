// CLI integration tests. These tests live in package main so they can
// drive the root cobra command directly via rootCmd.SetArgs + Execute,
// which is the same path the binary takes minus os.Args parsing.
//
// We boot a real httptest server backed by an in-memory SQLite + random
// master key (mirror of internal/server's testServer setup), then point
// the CLI at it via --server / --credentials.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/idenn207/comax-secrets/internal/cli/credentials"
	"github.com/idenn207/comax-secrets/internal/cli/secretrc"
	"github.com/idenn207/comax-secrets/internal/crypto"
	"github.com/idenn207/comax-secrets/internal/server"
	"github.com/idenn207/comax-secrets/internal/store"
)

type staticKey []byte

func (k staticKey) Key(_ context.Context) ([]byte, error) { return k, nil }

// startTestServer is a near-clone of internal/server's testServer
// helper, repeated here because the test binary lives in package main
// and can't import _test.go helpers from internal/server.
func startTestServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "cli.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	key := make([]byte, crypto.KeySize)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand: %v", err)
	}
	srv := httptest.NewServer(server.NewServer(server.Options{
		DB: db, Keys: staticKey(key),
	}).Handler())
	t.Cleanup(srv.Close)

	// Bootstrap to obtain a usable token. We hit the endpoint directly
	// via http.Client because the CLI's `login` command would write to
	// the credentials file before we want it to.
	resp, err := srv.Client().Post(srv.URL+"/api/v1/bootstrap", "application/json", nil)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		OK   bool `json:"ok"`
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read bootstrap body: %v", err)
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("bootstrap decode: %v (body=%q)", err, raw)
	}
	if body.Data.Token == "" {
		t.Fatal("bootstrap returned empty token")
	}
	return srv, body.Data.Token
}

func TestLogin_HappyPath(t *testing.T) {
	srv, token := startTestServer(t)
	credPath := filepath.Join(t.TempDir(), "creds.json")

	root := newRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{
		"--credentials", credPath,
		"login",
		"--server", srv.URL,
		"--token", token,
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("login: %v", err)
	}

	got, err := credentials.LoadFrom(credPath)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if got.Server != srv.URL || got.Token != token {
		t.Errorf("creds = %+v; want server=%s token=%s", got, srv.URL, token)
	}
}

func TestLogin_ReadsTokenFromEnv(t *testing.T) {
	srv, token := startTestServer(t)
	credPath := filepath.Join(t.TempDir(), "creds.json")
	// H1: with --token omitted, the CLI reads $COMAX_TOKEN so the plaintext
	// never has to appear on the command line — the GitHub Action relies on
	// this to keep the token out of process listings.
	t.Setenv("COMAX_TOKEN", token)

	root := newRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{
		"--credentials", credPath,
		"login",
		"--server", srv.URL,
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("login via $COMAX_TOKEN: %v", err)
	}
	got, err := credentials.LoadFrom(credPath)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if got.Token != token {
		t.Errorf("saved token = %q; want %q (read from $COMAX_TOKEN)", got.Token, token)
	}
}

func TestLogin_MissingTokenErrors(t *testing.T) {
	srv, _ := startTestServer(t)
	credPath := filepath.Join(t.TempDir(), "creds.json")
	t.Setenv("COMAX_TOKEN", "") // neither --token nor env supplies a token

	root := newRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{
		"--credentials", credPath,
		"login",
		"--server", srv.URL,
	})
	if err := root.Execute(); err == nil {
		t.Fatal("login without --token or $COMAX_TOKEN returned nil; want error")
	}
	if _, err := os.Stat(credPath); err == nil {
		t.Errorf("credentials file exists after tokenless login at %s", credPath)
	}
}

func TestLogin_RejectsBadToken(t *testing.T) {
	srv, _ := startTestServer(t)
	credPath := filepath.Join(t.TempDir(), "creds.json")

	root := newRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{
		"--credentials", credPath,
		"login",
		"--server", srv.URL,
		"--token", "definitely-not-a-real-token",
	})
	if err := root.Execute(); err == nil {
		t.Fatal("login returned nil; want token verification failure")
	}
	// File must not have been written.
	if _, err := os.Stat(credPath); err == nil {
		t.Errorf("credentials file exists after failed login at %s", credPath)
	}
}

func TestInit_CreatesProjectAndSecretrc(t *testing.T) {
	srv, token := startTestServer(t)
	credPath := filepath.Join(t.TempDir(), "creds.json")
	if err := credentials.SaveTo(credPath, credentials.Credentials{
		Server: srv.URL, Token: token,
	}); err != nil {
		t.Fatalf("seed credentials: %v", err)
	}

	cwd := t.TempDir()
	pushd(t, cwd)

	root := newRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{
		"--credentials", credPath,
		"init",
		"--project", "comax",
		"--envs", "local,dev,prod",
		"--default-env", "local",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("init: %v", err)
	}

	cfg, err := secretrc.Load(cwd)
	if err != nil {
		t.Fatalf("secretrc.Load: %v", err)
	}
	if cfg.Project != "comax" {
		t.Errorf("Project = %q; want comax", cfg.Project)
	}
	if cfg.DefaultEnv != "local" {
		t.Errorf("DefaultEnv = %q; want local", cfg.DefaultEnv)
	}
}

func TestInit_IsIdempotent(t *testing.T) {
	srv, token := startTestServer(t)
	credPath := filepath.Join(t.TempDir(), "creds.json")
	if err := credentials.SaveTo(credPath, credentials.Credentials{
		Server: srv.URL, Token: token,
	}); err != nil {
		t.Fatalf("seed credentials: %v", err)
	}
	cwd := t.TempDir()
	pushd(t, cwd)

	for i := 0; i < 2; i++ {
		root := newRootCmd()
		root.SetOut(new(bytes.Buffer))
		root.SetErr(new(bytes.Buffer))
		root.SetArgs([]string{
			"--credentials", credPath,
			"init", "--project", "comax",
		})
		if err := root.Execute(); err != nil {
			t.Fatalf("init pass %d: %v", i+1, err)
		}
	}
}

func TestInit_WithoutLoginFails(t *testing.T) {
	cwd := t.TempDir()
	pushd(t, cwd)
	credPath := filepath.Join(t.TempDir(), "absent.json") // never created

	root := newRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{
		"--credentials", credPath,
		"init", "--project", "comax",
	})
	if err := root.Execute(); err == nil {
		t.Fatal("init without credentials returned nil; want failure")
	}
}

// pushd changes cwd to dir and registers a cleanup to restore the
// original. Required because secret init writes .secretrc to cwd.
func pushd(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir %q: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
}
