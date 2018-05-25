package main

import (
	"encoding/json"
	"os"

	"code.cloudfoundry.org/log-cache-cli/pkg/command/k8s"
)

// version is set via ldflags at compile time. It should be JSON encoded
// VersionType.
var version string

type VersionType struct {
	Major int
	Minor int
	Build int
}

func init() {
	// TODO: do something with version information
	var v VersionType
	_ = json.Unmarshal([]byte(version), &v)
}

func main() {
	if k8s.Execute(k8s.WithOutput(os.Stdout)) != nil {
		os.Exit(1)
	}
}
