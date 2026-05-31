package version

// Version is set at build time via -ldflags "-X github.com/leofds/conduit/internal/version.Version=<tag>".
// Falls back to "dev" when built without the flag (e.g. go run).
var Version = "dev"
