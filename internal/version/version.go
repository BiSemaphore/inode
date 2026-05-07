package version

// Populated at build time via -ldflags.
// See .goreleaser.yml and Makefile.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)
