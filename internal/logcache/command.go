package logcache

import (
	"code.cloudfoundry.org/cli/plugin"
)

// Map of command names to Commands. Key/value pairs are represent available
// Commands for the plugin.
var commands = make(map[string]Command)

// Command is an interface that each command is required to meet.
type Command interface {
	Run(plugin.CliConnection, []string)
	Metadata() plugin.Command
}

// AddCommand adds a Command to the map of Commands. Making it available to the
// plugin.
func AddCommand(c Command) {
	commands[c.Metadata().Name] = c
}
