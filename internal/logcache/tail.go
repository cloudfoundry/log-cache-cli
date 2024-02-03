package logcache

import (
	"context"
	"fmt"
	"log"
	"time"

	"code.cloudfoundry.org/cli/plugin"
	logcache "code.cloudfoundry.org/go-log-cache/v2"
	"code.cloudfoundry.org/go-loggregator/v9/rpc/loggregator_v2"

	"code.cloudfoundry.org/log-cache-cli/v4/internal/command"
	"code.cloudfoundry.org/log-cache-cli/v4/internal/source"
)

var (
	follow bool
)

func init() {
	tailCmd.Flags().BoolVarP(&follow, "follow", "f", false, "")
	command.Add(tailCmd)
}

var tailCmd = &command.Command{
	Name:     "tail",
	HelpText: "Output envelopes for a source",
	UsageDetails: plugin.Usage{
		Usage: `tail [options] SOURCE_ID/APP_NAME`,
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
	PositionalArgs: 1,
	Run: func(cmd *command.Command, c command.LogCacheClient, args []string) error {
		src := source.Get(cmd.Conn(), args[0])
		if err := printHeaders(cmd.Conn(), src); err != nil {
			return err
		}

		ctx := context.Background()
		envelopes, err := c.Read(ctx, src.ID, time.Unix(0, 0))
		if err != nil {
			return fmt.Errorf("could not read from Log Cache: %w", err)
		}
		for _, e := range envelopes {
			log.Println(e)
		}
		if follow {
			v := func(envelopes []*loggregator_v2.Envelope) bool {
				for _, e := range envelopes {
					log.Println(e)
				}
				return true
			}
			logcache.Walk(ctx, src.ID, v, c.Read)
		}

		return nil
	},
}

func printHeaders(conn plugin.CliConnection, s source.Source) error {
	username, err := conn.Username()
	if err != nil {
		return fmt.Errorf("could not retrieve username: %w", err)
	}
	switch s.Type {
	case source.ApplicationType, source.ServiceType:
		org, err := conn.GetCurrentOrg()
		if err != nil {
			return fmt.Errorf("could not retrieve current org: %w", err)
		}
		space, err := conn.GetCurrentSpace()
		if err != nil {
			return fmt.Errorf("could not retrieve current space: %w", err)
		}
		log.Printf(`Retrieving envelopes for app "%s" in org "%s" / space "%s" as "%s"...`, s.Name, org.Name, space.Name, username)
	case source.UnknownType:
		log.Printf(`Retrieving envelopes for source "%s" as "%s"...`, s.ID, username)
	}
	log.Println("")
	return nil
}
