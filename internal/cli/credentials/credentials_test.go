package credentials

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSaveAndLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.json")
	in := Credentials{Server: "https://comax.local:8443", Token: "abc123"}

	if err := SaveTo(path, in); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}
	got, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if got != in {
		t.Errorf("got %+v; want %+v", got, in)
	}
}

func TestLoad_MissingFileIsErrNotFound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.json")
	_, err := LoadFrom(path)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v; want wraps ErrNotFound", err)
	}
}

func TestSave_RejectsEmptyFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.json")
	cases := []Credentials{
		{Server: "", Token: "t"},
		{Server: "s", Token: ""},
		{},
	}
	for i, c := range cases {
		if err := SaveTo(path, c); err == nil {
			t.Errorf("case %d %+v: SaveTo returned nil; want error", i, c)
		}
	}
}

func TestLoad_RejectsPartialFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.json")
	if err := os.WriteFile(path, []byte(`{"server":"https://x","token":""}`), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := LoadFrom(path); err == nil {
		t.Error("LoadFrom returned nil on partial file; want error")
	}
}

func TestLoad_RejectsCorruptJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.json")
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := LoadFrom(path); err == nil {
		t.Error("LoadFrom returned nil on corrupt file; want error")
	}
}

func TestSave_FileIs0600OnUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX mode bits don't reflect NTFS ACLs")
	}
	path := filepath.Join(t.TempDir(), "credentials.json")
	if err := SaveTo(path, Credentials{Server: "s", Token: "t"}); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if mode := st.Mode().Perm(); mode != 0o600 {
		t.Errorf("mode = %#o; want 0600", mode)
	}
}

func TestSave_CreatesMissingDir(t *testing.T) {
	// Save into a path whose parent doesn't exist yet — Save should
	// MkdirAll the chain.
	dir := filepath.Join(t.TempDir(), "deep", "nested", "config")
	path := filepath.Join(dir, "credentials.json")
	if err := SaveTo(path, Credentials{Server: "s", Token: "t"}); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file missing after Save: %v", err)
	}
}

func TestPath_NoError(t *testing.T) {
	// Path is platform-dependent; just assert it returns *something*
	// non-empty and ends in our suffix.
	p, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if filepath.Base(p) != "credentials.json" {
		t.Errorf("Path base = %q; want credentials.json", filepath.Base(p))
	}
}

// TestSave_DefaultPath_RoundTripsViaLoad exercises the production-path
// Save() / Load() variants (no explicit path arg). We isolate the
// filesystem via XDG_CONFIG_HOME on Unix and APPDATA on Windows so the
// test never touches the operator's real ~/.config/comax/.
func TestSave_DefaultPath_RoundTripsViaLoad(t *testing.T) {
	dir := t.TempDir()
	// os.UserConfigDir honours both XDG_CONFIG_HOME (Unix) and APPDATA
	// (Windows); setting both is harmless on the wrong platform.
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("APPDATA", dir)
	t.Setenv("HOME", dir) // macOS fallback when XDG isn't set

	in := Credentials{Server: "https://comax.local", Token: "tok"}
	if err := Save(in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != in {
		t.Errorf("got %+v; want %+v", got, in)
	}
}

// TestSave_OverwritesExisting verifies the atomic-rename path works
// when the target file already exists. Bug a regression here would
// leave operators with stale credentials after a re-login.
func TestSave_OverwritesExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.json")
	if err := SaveTo(path, Credentials{Server: "s1", Token: "t1"}); err != nil {
		t.Fatalf("first Save: %v", err)
	}
	if err := SaveTo(path, Credentials{Server: "s2", Token: "t2"}); err != nil {
		t.Fatalf("second Save: %v", err)
	}
	got, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Server != "s2" || got.Token != "t2" {
		t.Errorf("got %+v; want server=s2 token=t2 (atomic overwrite failed)", got)
	}
}
