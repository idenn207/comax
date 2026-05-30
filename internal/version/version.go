// Package version exposes the build-time version string for the
// secret-server and secret CLI binaries. It is intentionally tiny so
// that ldflags injection stays trivial:
//
//	go build -ldflags "-X 'github.com/idenn207/comax-secrets/internal/version.Version=v0.1.0'"
package version

// Version is the build version. Overridden via -ldflags at build time;
// defaults to "dev" for local non-release builds.
var Version = "dev"

// String returns the current build version.
func String() string {
	return Version
}
