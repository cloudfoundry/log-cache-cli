package main

import (
	"os"

	"code.cloudfoundry.org/log-cache-cli/pkg/command"
)

func main() {
	if command.Execute(command.WithOutput(os.Stdout)) != nil {
		os.Exit(1)
	}
}
