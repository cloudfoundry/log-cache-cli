package main

import "code.cloudfoundry.org/log-cache-cli/v4/internal/plugin"

// semver version is set via ldflags at compile time
var version string

func main() {
	p := plugin.New(version)
	p.Start()
}
