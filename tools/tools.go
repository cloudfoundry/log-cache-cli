//go:build tools

// Package tools imports packages that are used when running go generate, or
// used during the development process but not otherwise depended on by built
// code.
package tools

import (
	_ "github.com/maxbrunsfeld/counterfeiter/v6"
)
