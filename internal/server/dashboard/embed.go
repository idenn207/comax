//go:build embed_dashboard

// Package dashboard exposes the embedded SPA filesystem.
//
// With `-tags embed_dashboard` the Vite build output (web/dashboard/dist,
// synced into ./dist by `make dashboard`) is compiled into the binary
// via //go:embed. Without the tag, embed_dev.go is compiled instead and
// FS returns nil so contributors can run `go test ./...` and `go build`
// without Node installed.
package dashboard

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// FS returns the embedded SPA filesystem rooted at the dist directory.
// The error is reserved for future bundling schemes (e.g. a zipfs
// overlay); fs.Sub here cannot fail because dist exists at compile time.
func FS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}

// Embedded reports whether the SPA assets are compiled into this binary.
// Boot logging and the SPA handler both branch on this rather than on a
// build tag so the same handler binary works for both modes.
func Embedded() bool { return true }
