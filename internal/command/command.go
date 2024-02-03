// Package command provides commands for the log-cache cf CLI plugin, inspired
// by cobra.
package command

import (
	"errors"
	"fmt"
	"strings"

	"code.cloudfoundry.org/cli/plugin"
	logcache "code.cloudfoundry.org/go-log-cache/v2"
	flag "github.com/spf13/pflag"
)

var (
	// Registry of commands.
	commands map[string]*Command = make(map[string]*Command)
)

type Command struct {
	// The name of the command, e.g. "tail" would represent the command `cf
	// tail`.
	Name string

	// Short, one-sentence description of what the command does.
	HelpText string

	// Usage details to report to the cf CLI for this command.
	UsageDetails plugin.Usage

	// The actual work function.
	Run func(cmd *Command, c LogCacheClient, args []string) error

	// The number of expected arguments.
	PositionalArgs int

	// The set of flags for this command.
	fs *flag.FlagSet

	// The cf CLI connection.
	conn plugin.CliConnection
}

func (c *Command) Flags() *flag.FlagSet {
	if c.fs == nil {
		c.fs = flag.NewFlagSet(c.Name, flag.ContinueOnError)
	}
	return c.fs
}

func (c *Command) Conn() plugin.CliConnection {
	if c.conn == nil {
		panic("expected a plugin.CliConnection to be set")
	}
	return c.conn
}

func Add(c *Command) {
	commands[c.Name] = c
}

func Run(conn plugin.CliConnection, args []string) error {
	cmd := commands[args[0]]

	cmd.conn = conn

	if err := cmd.Flags().Parse(args[1:]); err != nil {
		return fmt.Errorf("incorrect usage: %w", err)
	}

	if err := validateArgCount(len(cmd.Flags().Args()), cmd.PositionalArgs); err != nil {
		return fmt.Errorf("incorrect usage: %w", err)
	}

	ok, err := cmd.Conn().HasAPIEndpoint()
	if err != nil {
		return fmt.Errorf("no API endpoint set: %w", err)
	}
	if !ok {
		return errors.New("no API endpoint set")
	}

	endpoint, err := cmd.Conn().ApiEndpoint()
	if err != nil {
		return fmt.Errorf("could not retrieve API endpoint: %w", err)
	}
	endpoint = strings.Replace(endpoint, "api", "log-cache", 1)

	skipSSL, err := conn.IsSSLDisabled()
	if err != nil {
		return fmt.Errorf("could not retrieve SSL settings: %w", err)
	}

	var client LogCacheClient = DefaultClient
	if client == nil {
		c := NewHTTPClient(conn, skipSSL)
		opt := logcache.WithHTTPClient(c)
		client = logcache.NewClient(endpoint, opt)
	}

	return cmd.Run(cmd, client, cmd.Flags().Args())
}

func validateArgCount(received int, expected int) error {
	switch {
	case received < expected:
		return fmt.Errorf("requires at least %d arg(s), only received %d", expected, received)
	case expected < received:
		return fmt.Errorf("accepts at most %d arg(s), received %d", expected, received)
	default:
		return nil
	}
}

func Commands() []plugin.Command {
	var pluginCmds []plugin.Command
	for _, c := range commands {
		pluginCmds = append(pluginCmds, plugin.Command{
			Name:         c.Name,
			HelpText:     c.HelpText,
			UsageDetails: c.UsageDetails,
		})
	}
	return pluginCmds
}
