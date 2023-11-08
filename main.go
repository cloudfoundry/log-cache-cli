package main

import (
	"code.cloudfoundry.org/cli/plugin"
	"code.cloudfoundry.org/log-cache-cli/v4/internal/logcache"
)

// version is expected to be set via ldflags at compile time to a
// `MAJOR.MINOR.BUILD` version string, e.g. `"1.2.3"`, `"4.0.0`.
var version string

func main() {
	plugin.Start(logcache.New(versionType()))
}
