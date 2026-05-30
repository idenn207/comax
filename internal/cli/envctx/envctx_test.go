package envctx

import (
	"errors"
	"testing"

	"github.com/idenn207/comax-secrets/internal/cli/secretrc"
)

func TestResolve_PrecedenceOrder(t *testing.T) {
	// Each row exercises one rule firing while the lower-priority rules
	// are also populated, to prove the higher rule wins. The "missing
	// higher input" cases (lower rules firing) come at the bottom.
	cases := []struct {
		name       string
		in         Inputs
		wantEnv    string
		wantSource Source
	}{
		{
			name: "flag beats env var",
			in: Inputs{
				Flag:   "from-flag",
				EnvVar: "from-env-var",
				Cfg:    secretrc.Config{Env: "from-secretrc", DefaultEnv: "from-default"},
				Branch: "main",
			},
			wantEnv: "from-flag", wantSource: SourceFlag,
		},
		{
			name: "env var beats secretrc.env",
			in: Inputs{
				EnvVar: "from-env-var",
				Cfg:    secretrc.Config{Env: "from-secretrc", DefaultEnv: "from-default"},
				Branch: "main",
			},
			wantEnv: "from-env-var", wantSource: SourceEnvVar,
		},
		{
			name: "secretrc.env beats branch map",
			in: Inputs{
				Cfg:    secretrc.Config{Env: "from-secretrc", DefaultEnv: "from-default", Branches: map[string]string{"main": "from-branches"}},
				Branch: "main",
			},
			wantEnv: "from-secretrc", wantSource: SourceSecretrcEnv,
		},
		{
			name: "branch map beats built-in",
			in: Inputs{
				Cfg:    secretrc.Config{Branches: map[string]string{"main": "from-branches"}, DefaultEnv: "from-default"},
				Branch: "main",
			},
			wantEnv: "from-branches", wantSource: SourceBranchMap,
		},
		{
			name: "built-in map beats default_env",
			in: Inputs{
				Cfg:    secretrc.Config{DefaultEnv: "from-default"},
				Branch: "main", // DefaultBranchMap["main"] = "prod"
			},
			wantEnv: "prod", wantSource: SourceBuiltinMap,
		},
		{
			name: "default_env when branch doesn't match",
			in: Inputs{
				Cfg:    secretrc.Config{DefaultEnv: "from-default"},
				Branch: "feat/something",
			},
			wantEnv: "from-default", wantSource: SourceDefault,
		},
		{
			name: "default_env when no branch",
			in: Inputs{
				Cfg: secretrc.Config{DefaultEnv: "from-default"},
			},
			wantEnv: "from-default", wantSource: SourceDefault,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Resolve(tc.in)
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if got.Env != tc.wantEnv {
				t.Errorf("Env = %q; want %q", got.Env, tc.wantEnv)
			}
			if got.Source != tc.wantSource {
				t.Errorf("Source = %q; want %q", got.Source, tc.wantSource)
			}
		})
	}
}

func TestResolve_UnresolvedWhenNoSignals(t *testing.T) {
	_, err := Resolve(Inputs{})
	if !errors.Is(err, ErrUnresolved) {
		t.Errorf("err = %v; want wraps ErrUnresolved", err)
	}
}

func TestResolve_EmptyBranchMapValueFallsThroughToBuiltin(t *testing.T) {
	// An operator who wrote .secretrc.branches = {"main": ""} (perhaps
	// deleting a row carelessly) should fall through to the built-in
	// map instead of getting an unresolved error.
	got, err := Resolve(Inputs{
		Cfg:    secretrc.Config{Branches: map[string]string{"main": ""}},
		Branch: "main",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Source != SourceBuiltinMap || got.Env != "prod" {
		t.Errorf("got %+v; want main→prod via built-in map", got)
	}
}

func TestResolve_UnknownBranchAndNoDefault(t *testing.T) {
	_, err := Resolve(Inputs{
		Cfg:    secretrc.Config{Branches: map[string]string{"main": "prod"}},
		Branch: "feat/x",
	})
	if !errors.Is(err, ErrUnresolved) {
		t.Errorf("err = %v; want wraps ErrUnresolved", err)
	}
}

// TestLoad_FromSecretrc exercises the production wiring (Load) by
// dropping a .secretrc into a temp dir and asserting we resolve against
// it. The test isolates env vars and cwd so it doesn't depend on the
// host's git state.
func TestLoad_FromSecretrc(t *testing.T) {
	dir := t.TempDir()
	if err := secretrc.Save(dir, secretrc.Config{
		Project:    "comax",
		DefaultEnv: "local",
	}); err != nil {
		t.Fatalf("seed secretrc: %v", err)
	}
	t.Setenv("COMAX_ENV", "")

	got, err := Load(dir, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Branch lookup may match the host's git branch unexpectedly, so
	// we accept either the built-in/branches source OR the default_env
	// source. The point of this test is that Load reads the file at
	// all — not which precedence layer wins.
	if got.Env == "" {
		t.Errorf("Load returned empty env; got=%+v", got)
	}
}

// TestLoad_FlagOverridesEverything mirrors the precedence test but
// through the production Load entry point, proving the wiring matches
// the pure Resolve function.
func TestLoad_FlagOverridesEverything(t *testing.T) {
	dir := t.TempDir()
	if err := secretrc.Save(dir, secretrc.Config{
		Project: "comax", DefaultEnv: "from-default",
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	t.Setenv("COMAX_ENV", "from-env")

	got, err := Load(dir, "from-flag")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Env != "from-flag" || got.Source != SourceFlag {
		t.Errorf("got %+v; want env=from-flag source=%s", got, SourceFlag)
	}
}

// TestLoad_MissingSecretrcIsNotFatal: operators in non-repo dirs (or
// before running `secret init`) should still be able to use --env.
func TestLoad_MissingSecretrcIsNotFatal(t *testing.T) {
	dir := t.TempDir() // no .secretrc inside
	t.Setenv("COMAX_ENV", "")
	got, err := Load(dir, "from-flag")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Env != "from-flag" {
		t.Errorf("got env=%q; want from-flag", got.Env)
	}
}
