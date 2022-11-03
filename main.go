/*
log-cache allows for better interactions with Cloud Foundry's Log-Cache.

It is a cf CLI plugin that must be installed by your local cf CLI to be used.
The latest version can generally be installed with:

	cf install-plugin log-cache
*/
package main

import "code.cloudfoundry.org/log-cache-cli/v4/internal/plugin"

// version is set at compile time with ldflags. See
// https://github.com/cloudfoundry/log-cache-cli/blob/main/internal/util/semver/semver.go#L14
// for details about how it will be parsed.
var version string

func main() {
	plugin.New(version).Start()
}
