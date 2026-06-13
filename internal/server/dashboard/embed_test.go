//go:build embed_dashboard

package dashboard

import (
	"io/fs"
	"testing"
)

// TestEmbedMode_EmbeddedTrue pins the embed-mode contract: when the
// build tag is on, Embedded() must report true so cmd/server logs
// "enabled" instead of "dev mode" and the SPA handler actually gets a
// usable FS.
func TestEmbedMode_EmbeddedTrue(t *testing.T) {
	if !Embedded() {
		t.Fatal("Embedded() = false in embed-mode build; expected true")
	}
}

// TestEmbedMode_FSResolves pins that FS() returns a non-nil FS rooted
// at dist/. The contents may be only .gitkeep before `make dashboard`
// runs — we only verify the root exists and is iterable.
func TestEmbedMode_FSResolves(t *testing.T) {
	dist, err := FS()
	if err != nil {
		t.Fatalf("FS error: %v", err)
	}
	if dist == nil {
		t.Fatal("FS returned nil in embed mode")
	}
	entries, err := fs.ReadDir(dist, ".")
	if err != nil {
		t.Fatalf("read dist root: %v", err)
	}
	// At minimum .gitkeep should be present (committed into the repo to
	// keep //go:embed happy before `npm run build` produces anything else).
	if len(entries) == 0 {
		t.Fatal("dist root has no entries; .gitkeep sentinel missing?")
	}
}
