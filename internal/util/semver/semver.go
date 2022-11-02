// Package semver provides utility parsing of strings into semver versions.
//
// This package only really exists so that we can pass a string in ldflags and
// convert that to a cf CLI plugin VersionType.
package semver

import (
	"strconv"
	"strings"

	"code.cloudfoundry.org/cli/plugin"
)

// ParseStr returns a cf CLI plugin.VersionType representing s.
//
// s should be of the format "X.X.X", where each X is an integer. Any deviation
// from that format will result in the function returning the default
// VersionType, which is all zeros, and is represented as N/A when using the
// resulting binary as a plugin.
func ParseStr(s string) plugin.VersionType {
	// TODO: remove beginning v if it's there

	sl := strings.Split(s, ".")
	v := plugin.VersionType{}
	if len(sl) == 3 {
		v.Major = convOrZero(sl[0])
		v.Minor = convOrZero(sl[1])
		v.Build = convOrZero(sl[2])
	}
	return v
}

// convOrZero attempts to convert s to an integer, and failing that just returns
// 0.
func convOrZero(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}
