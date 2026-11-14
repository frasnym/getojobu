package version

import (
	_ "embed"
	"strings"
)

//go:embed version.txt
var rawBuild string

// Build is the commit SHA or version string the binary was compiled with.
var Build = strings.TrimSpace(rawBuild)

// Short returns the commit SHA or version string the binary was built from.
func Short() string { return Build }
