// Package version exposes build-time metadata injected via ldflags.
// Displayed in `:about` (ARCHITECTURE §12.2).
package version

// Values are overridden at link time:
//
//	go build -ldflags "-X github.com/tedilabs/ota/internal/version.Tag=v0.1.0 \
//	                  -X github.com/tedilabs/ota/internal/version.Commit=<sha> \
//	                  -X github.com/tedilabs/ota/internal/version.BuildTime=<iso>"
var (
	Tag       = "v0.1.1"
	Commit    = "unknown"
	BuildTime = "unknown"
)
