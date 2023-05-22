package test

import (
	"os"
	"strings"
)

// IsDevEnvironment returns true if the STARLIGHT_ENV environment variable starts with "DEV".
// export STARLIGHT_ENV=dev
func IsDevEnvironment() bool {
	return strings.HasPrefix(strings.ToUpper(os.Getenv("STARLIGHT_ENV")), "DEV")
}
