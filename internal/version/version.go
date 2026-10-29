// Package version exposes the build version of the birdROMs handheld client.
package version

// Version is overridden at build time via -ldflags "-X .../version.Version=...".
var Version = "dev"

// String returns the human-readable version banner.
func String() string {
	return "birdROMs " + Version
}
