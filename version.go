package main

import (
	"strconv"
	"strings"

	"code.cloudfoundry.org/cli/plugin"
)

// versionType parses version into a plugin.VersionType. If version cannot be
// parsed correctly, then the default plugin.VersionType is returned, which
// results in the plugin being marked as "dev".
func versionType() plugin.VersionType {
	s := strings.Split(version, ".")
	if len(s) != 3 {
		return plugin.VersionType{}
	}

	var (
		err error
		vt  plugin.VersionType
	)
	vt.Major, err = strconv.Atoi(s[0])
	if err != nil {
		return plugin.VersionType{}
	}
	vt.Minor, err = strconv.Atoi(s[1])
	if err != nil {
		return plugin.VersionType{}
	}
	vt.Build, err = strconv.Atoi(s[2])
	if err != nil {
		return plugin.VersionType{}
	}

	return vt
}
