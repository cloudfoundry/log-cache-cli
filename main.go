package main

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"os"

	"code.cloudfoundry.org/cli/plugin"
	"code.cloudfoundry.org/log-cache-cli/internal/command"
)

type LogCacheCLI struct{}

var commands = map[string]command.Command{
	"tail": command.LogCache,
	"meta": command.Meta,
}

func (c *LogCacheCLI) Run(conn plugin.CliConnection, args []string) {
	if len(args) < 2 {
		log.Fatalf("Expected at least 2 argument, but got %d.", len(args))
	}

	skipSSL, err := conn.IsSSLDisabled()
	if err != nil {
		log.Fatalf("%s", err)
	}
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{
		InsecureSkipVerify: skipSSL,
	}

	op := commands[args[1]]
	op(context.Background(), conn, args[2:], http.DefaultClient, log.New(os.Stderr, "", 0), os.Stdout)
}

func (c *LogCacheCLI) GetMetadata() plugin.PluginMetadata {
	return plugin.PluginMetadata{
		Name: "Log Cache CLI Plugin",
		Commands: []plugin.Command{
			{
				Name: "log-cache",
				UsageDetails: plugin.Usage{
					Usage: `log-cache <meta | tail [options] <app-guid>]>

COMMANDS:
    tail: Output logs for an app
        --end-time           End of query range in UNIX nanoseconds.
        --envelope-type      Envelope type filter. Available filters: 'log', 'counter', 'gauge', 'timer', and 'event'.
        --follow             Output appended to stdout as logs are egressed.
        --json               Output envelopes in JSON format.
        --lines              Number of envelopes to return. Default is 10.
        --start-time         Start of query range in UNIX nanoseconds.
    meta: Get meta information from Log Cache`,
				},
			},
		},
	}
}

func main() {
	plugin.Start(&LogCacheCLI{})
}
