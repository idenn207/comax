package secretrc

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSaveAndLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	in := Config{
		Project:    "comax",
		DefaultEnv: "local",
		Branches:   map[string]string{"main": "prod", "dev": "dev"},
	}
	if err := Save(dir, in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Project != "comax" || got.DefaultEnv != "local" {
		t.Errorf("Load returned %+v; want project=comax default_env=local", got)
	}
	if got.Branches["main"] != "prod" || got.Branches["dev"] != "dev" {
		t.Errorf("Branches = %+v; want main->prod, dev->dev", got.Branches)
	}
}

func TestLoad_MissingIsErrNotFound(t *testing.T) {
	_, err := Load(t.TempDir())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v; want wraps ErrNotFound", err)
	}
}

func TestLoad_EmptyFileIsZeroConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), nil, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(got, Config{}) {
		t.Errorf("Load returned %+v; want zero value", got)
	}
}

func TestLoad_CorruptJSONErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("{ not json"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := Load(dir); err == nil {
		t.Error("Load returned nil on corrupt JSON; want error")
	}
}
