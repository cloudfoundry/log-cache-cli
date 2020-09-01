package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	"code.cloudfoundry.org/cli/plugin"
	"code.cloudfoundry.org/log-cache-cli/pkg/command/cf"
	"golang.org/x/crypto/ssh/terminal"
)

// version is set via ldflags at compile time. It should be JSON encoded
// plugin.VersionType. If it does not unmarshal, the plugin version will be
// left empty.
var version string

type LogCacheCLI struct{}

var commands = make(map[string]cf.Command)

func (c *LogCacheCLI) Run(conn plugin.CliConnection, args []string) {
	if len(args) == 1 && args[0] == "CLI-MESSAGE-UNINSTALL" {
		// someone's uninstalling the plugin, but we don't need to clean up
		return
	}

	if len(args) < 1 {
		log.Fatalf("Expected at least 1 argument, but got %d.", len(args))
	}

	isTerminal := terminal.IsTerminal(int(os.Stdout.Fd()))

	commands["query"] = func(ctx context.Context, cli plugin.CliConnection, args []string, c cf.HTTPClient, log cf.Logger, tableWriter io.Writer) {
		var opts []cf.QueryOption
		cf.Query(ctx, cli, args, c, log, tableWriter, opts...)
	}

	commands["tail"] = func(ctx context.Context, cli plugin.CliConnection, args []string, c cf.HTTPClient, log cf.Logger, tableWriter io.Writer) {
		var opts []cf.TailOption
		if !isTerminal {
			opts = append(opts, cf.WithTailNoHeaders())
		}
		cf.Tail(ctx, cli, args, c, log, tableWriter, opts...)
	}

	commands["log-meta"] = func(ctx context.Context, cli plugin.CliConnection, args []string, c cf.HTTPClient, log cf.Logger, tableWriter io.Writer) {
		var opts []cf.MetaOption
		if !isTerminal {
			opts = append(opts, cf.WithMetaNoHeaders())
		}
		cf.Meta(
			ctx,
			cli,
			args,
			c,
			log,
			tableWriter,
			opts...,
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
						"-start-time":         "Start of query range in UNIX nanoseconds.",
						"-end-time":           "End of query range in UNIX nanoseconds.",
						"-envelope-type, -t":  "Envelope type filter. Available filters: 'log', 'counter', 'gauge', 'timer', 'event', and 'any'.",
						"-envelope-class, -c": "Envelope class filter. Available filters: 'logs', 'metrics', and 'any'.",
						"-follow, -f":         "Output appended to stdout as logs are egressed.",
						"-json":               "Output envelopes in JSON format.",
						"-lines, -n":          "Number of envelopes to return. Default is 10.",
						"-new-line":           "Character used for new line substition, must be single unicode character. Default is '\\n'.",
						"-name-filter":        "Filters metrics by name.",
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
						"-source-type": "Source type of information to show. Available: 'all', 'application', and 'platform'.",
						"-sort-by":     "Sort by specified column. Available: 'source-id', 'source', 'source-type', 'count', 'expired', 'cache-duration', and 'rate'.",
						"-noise":       "Fetch and display the rate of envelopes per minute for the last minute. WARNING: This is slow...",
						"-guid":        "Display raw source GUIDs",
					},
				},
			},
			{
				Name:     "query",
				HelpText: "Issues a PromQL query against Log Cache",
				UsageDetails: plugin.Usage{
					Usage: `query <promql-query> [options]

ENVIRONMENT VARIABLES:
   LOG_CACHE_ADDR       Overrides the default location of log-cache.
   LOG_CACHE_SKIP_AUTH  Set to 'true' to disable CF authentication.`,
					Options: map[string]string{
						"-time":  "Effective time for query execution of an instant query. Cannont be used with --start, --end, or --step. Can be a unix timestamp or RFC3339.",
						"-start": "Start time for a range query. Cannont be used with --time. Can be a unix timestamp or RFC3339.",
						"-end":   "End time for a range query. Cannont be used with --time. Can be a unix timestamp or RFC3339.",
						"-step":  "Step interval for a range query. Cannot be used with --time.",
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
