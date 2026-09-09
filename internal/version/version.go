package version

// Version is the canonical SemVer 2.0.0 version string for novus-installer.
// Value is injected via ldflags during CI/CD release builds:
//   -X github.com/sgc-novus/novus-installer/internal/version.Version=v0.1.0
//   -X github.com/sgc-novus/novus-installer/internal/version.GitCommit=$(git rev-parse --short HEAD)
//   -X github.com/sgc-novus/novus-installer/internal/version.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)
var (
	Version   = "0.1.0-dev"
	GitCommit = "none"
	BuildDate = "unknown"
)

// Info returns full build signature.
func Info() string {
	if GitCommit == "none" {
		return Version
	}
	return Version + " (" + GitCommit + ", " + BuildDate + ")"
}
