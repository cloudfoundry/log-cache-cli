package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"code.cloudfoundry.org/cli/plugin"
	logcache "code.cloudfoundry.org/go-log-cache"
)

var (
	MetaName     = "log-meta"
	MetaHelpText = "Show all available meta information"
	MetaUsage    = plugin.Usage{
		Usage: `log-meta [--source-type TYPE] [--sort-by COLUMN] [--noise] [--guid]`,
		Options: map[string]string{
			"--source-type": "Source type of information to show. Available: 'all', 'application', 'service', 'platform', and 'unknown'. Excludes unknown sources unless 'all' or 'unknown' is selected, or `--guid` is used. To receive information on platform or unknown source id's, you must have the doppler.firehose, or logs.admin scope.",
			"--sort-by":     "Sort by specified column. Available: 'source-id', 'source', 'source-type', 'count', 'expired', 'cache-duration', and 'rate'.",
			"--noise":       "Fetch and display the rate of envelopes per minute for the last minute. WARNING: This is slow...",
			"--guid":        "Display raw source GUIDs with no source Names. Incompatible with 'source' and 'source-type' for --sort-by. Only allows 'platform' for --source-type",
		},
	}
)

var (
	writer io.Writer
)

type MetaOpts struct {
	Filter       SourceOpt
	SortBy       SortOpt
	DisplayNoise bool
	DisplayGuids bool
}

func Meta(conn plugin.CliConnection, opts *MetaOpts) {
	e, err := conn.ApiEndpoint()
	if err != nil {
		fmt.Fprintln(writer, "Could not retrieve Log-Cache endpoint.")
		fmt.Fprintf(writer, "Error: %s\n", err)
		return
	}
	e = strings.Replace(e, "api", "log-cache", 1)

	client := logcache.NewClient(e)

	_, err = client.Meta(context.TODO())
	if err != nil {
		fmt.Fprintln(writer, "Could not retrieve meta information from Log-Cache.")
		fmt.Fprintf(writer, "Error: %s\n", err)
		return
	}

	tw := tabwriter.NewWriter(writer, 0, 2, 2, ' ', 0)
	fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", "Source", "Source Type", "Count", "Expired", "Cache Duration", "Rate/minute")
	tw.Flush()
}
