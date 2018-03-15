package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"code.cloudfoundry.org/cli/plugin"
	"code.cloudfoundry.org/log-cache-cli/internal/command"
)

// version is set via ldflags at compile time. It should be JSON encoded
// plugin.VersionType. If it does not unmarshal, the plugin version will be
// left empty.
var version string

type LogCacheCLI struct{}

var commands = map[string]command.Command{
	"tail": command.Tail,
}

func (c *LogCacheCLI) Run(conn plugin.CliConnection, args []string) {
	if len(args) == 1 && args[0] == "CLI-MESSAGE-UNINSTALL" {
		// someone's uninstalling the plugin, but we don't need to clean up
		return
	}

	if len(args) < 1 {
		log.Fatalf("Expected at least 1 argument, but got %d.", len(args))
	}

	commands["log-meta"] = func(ctx context.Context, cli plugin.CliConnection, args []string, c command.HTTPClient, log command.Logger, tableWriter io.Writer) {
		command.Meta(
			ctx,
			cli,
			func(sourceID string, start, end time.Time) []string {
				var buf linesWriter

				args := []string{
					sourceID,
					"--start-time",
					strconv.FormatInt(start.UnixNano(), 10),
					"--end-time",
					strconv.FormatInt(end.UnixNano(), 10),
					"--json",
					"--lines", "1000",
				}

				command.Tail(
					ctx,
					cli,
					args,
					c,
					log,
					&buf,
				)

				return buf.lines
			},
			args,
			c,
			log,
			tableWriter,
		)
	}

	skipSSL, err := conn.IsSSLDisabled()
	if err != nil {
		log.Fatalf("%s", err)
	}
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{
		InsecureSkipVerify: skipSSL,
	}

	op, ok := commands[args[0]]
	if !ok {
		log.Fatalf("Unknown Log Cache command: %s", args[0])
	}
	op(context.Background(), conn, args[1:], http.DefaultClient, log.New(os.Stderr, "", 0), os.Stdout)
}

func (c *LogCacheCLI) GetMetadata() plugin.PluginMetadata {
	var v plugin.VersionType
	// Ignore the error. If this doesn't unmarshal, then we want the default
	// VersionType.
	_ = json.Unmarshal([]byte(version), &v)

	return plugin.PluginMetadata{
		Name:    "log-cache",
		Version: v,
		Commands: []plugin.Command{
			{
				Name:     "tail",
				HelpText: "Output logs for a source-id/app",
				UsageDetails: plugin.Usage{
					Usage: `tail [options] <source-id/app>

ENVIRONMENT VARIABLES:
   LOG_CACHE_ADDR       Overrides the default location of log-cache.
   LOG_CACHE_SKIP_AUTH  Set to 'true' to disable CF authentication.`,
					Options: map[string]string{
						"end-time":      "End of query range in UNIX nanoseconds.",
						"envelope-type": "Envelope type filter. Available filters: 'log', 'counter', 'gauge', 'timer', and 'event'.",
						"follow, -f":    "Output appended to stdout as logs are egressed.",
						"json":          "Output envelopes in JSON format.",
						"lines, -n":     "Number of envelopes to return. Default is 10.",
						"start-time":    "Start of query range in UNIX nanoseconds.",
						"counter-name":  "Counter name filter (implies --envelope-type=counter).",
						"gauge-name":    "Gauge name filter (implies --envelope-type=gauge).",
					},
				},
			},
			{
				Name:     "log-meta",
				HelpText: "Show all available meta information",
				UsageDetails: plugin.Usage{
					Usage: `log-meta [options]

ENVIRONMENT VARIABLES:
   LOG_CACHE_ADDR       Overrides the default location of log-cache.
   LOG_CACHE_SKIP_AUTH  Set to 'true' to disable CF authentication.`,
					Options: map[string]string{
						"scope": "Scope of meta information to show. Available: 'all', 'applications', and 'platform'.",
						"noise": "Fetch and display the rate of envelopes per minute for the last minute. WARNING: This is slow...",
					},
				},
			},
		},
	}
}

func main() {
	plugin.Start(&LogCacheCLI{})
}

type linesWriter struct {
	lines []string
}

func (w *linesWriter) Write(data []byte) (int, error) {
	w.lines = append(w.lines, string(data))
	return len(data), nil
}
