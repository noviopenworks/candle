package version

// version is the build version of the server. It defaults to a dev marker and
// is overridden at release time via the linker (`-ldflags "-X ...version.version=..."`),
// which GoReleaser sets from the git tag.
var version = "1.0.0-dev"

// String returns the build version of the server.
func String() string { return version }
