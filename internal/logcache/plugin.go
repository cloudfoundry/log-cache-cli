package logcache

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"code.cloudfoundry.org/cli/plugin"

	"code.cloudfoundry.org/log-cache-cli/v4/internal/command"
)

// A Client is implemented by the standard library's http.Client and
// TokenClient.
type Client interface {
	Do(req *http.Request) (*http.Response, error)
}

var (
	DefaultClient Client = http.DefaultClient
)

type LogCache struct {
	version plugin.VersionType
}

func New(version plugin.VersionType) *LogCache {
	return &LogCache{version: version}
}

func (lc *LogCache) Run(conn plugin.CliConnection, args []string) {
	// Plugins are sent this command when they're uninstalled in case they want
	// to print a message or something.
	if args[0] == "CLI-MESSAGE-UNINSTALL" {
		os.Exit(0)
	}

	log.SetFlags(0)
	log.SetOutput(os.Stdout)
	log.SetPrefix("")

	err := command.Run(conn, args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func (lc *LogCache) GetMetadata() plugin.PluginMetadata {
	return plugin.PluginMetadata{
		Name:     "log-cache",
		Version:  lc.version,
		Commands: command.Commands(),
	}
}
