package logcache

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{Use: "log-cache"}

// Map of command names to Commands. Key/value pairs are represent available
// Commands for the plugin.
var commands = make(map[string]*cobra.Command)

// AddCommand adds a Command to the map of Commands. Making it available to the
// plugin.
func AddCommand(c *cobra.Command) {
	commands[c.Use] = c
}
