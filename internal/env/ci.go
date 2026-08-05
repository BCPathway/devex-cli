package env

import (
	"os"
)

var isCI bool

// Init initializes the CI context based on environment variables.
func Init() {
	if os.Getenv("CI") == "true" || os.Getenv("CI") == "1" ||
		os.Getenv("GITHUB_ACTIONS") == "true" ||
		os.Getenv("GITLAB_CI") == "true" {
		isCI = true
	}
}

// SetCI manually sets the CI mode, overriding environment detection.
func SetCI(ci bool) {
	isCI = ci
}

// IsCI returns true if the CLI is running in a Continuous Integration environment.
func IsCI() bool {
	return isCI
}
