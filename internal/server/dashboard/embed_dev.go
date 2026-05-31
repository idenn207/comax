//go:build !embed_dashboard

// Package dashboard. Dev-mode shim: without `-tags embed_dashboard`
// the SPA is not compiled in. Contributors can build and test the
// server (and run `go test ./...`) without Node or a Vite build.
//
// The server boot logs "dashboard: dev mode, /api only" so the operator
// knows the browser UI will not respond at "/".
package dashboard

import "io/fs"

// FS returns (nil, nil). The SPA handler treats nil FS as "no dashboard"
// and returns the same envelope-shaped 404 as any other unknown path.
func FS() (fs.FS, error) { return nil, nil }

// Embedded always reports false in dev mode.
func Embedded() bool { return false }
