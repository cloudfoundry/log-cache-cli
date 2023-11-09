package logcache

import (
	"context"

	"code.cloudfoundry.org/cli/plugin"
)

type LogCache struct {
	version plugin.VersionType
}

func New(version plugin.VersionType) *LogCache {
	return &LogCache{version: version}
}

func (lc *LogCache) Run(conn plugin.CliConnection, args []string) {
	ctx := context.WithValue(context.Background(), "conn", conn)
	rootCmd.SetArgs(args)
	rootCmd.ExecuteContext(ctx)
}

func (lc *LogCache) GetMetadata() plugin.PluginMetadata {
	cmds := rootCmd.Commands()
	for _, cmd := range cmds {
		cmd.Flags().FlagUsages()
	}
	return plugin.PluginMetadata{
		Name:    "log-cache",
		Version: lc.version,
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
						"-source-type": "Source type of information to show. Available: 'all', 'application', 'service', 'platform', and 'unknown'. Excludes unknown sources unless 'all' or 'unknown' is selected, or `--guid` is used. To receive information on platform or unknown source id's, you must have the doppler.firehose, or logs.admin scope.",
						"-sort-by":     "Sort by specified column. Available: 'source-id', 'source', 'source-type', 'count', 'expired', 'cache-duration', and 'rate'.",
						"-noise":       "Fetch and display the rate of envelopes per minute for the last minute. WARNING: This is slow...",
						"-guid":        "Display raw source GUIDs with no source Names. Incompatible with 'source' and 'source-type' for --sort-by. Only allows 'platform' for --source-type",
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
