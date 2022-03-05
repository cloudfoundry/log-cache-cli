package command

import (
	"context"
	"io"

	"code.cloudfoundry.org/cli/plugin"
)

// Command is the interface to implement plugin commands
type Command func(ctx context.Context, cli plugin.CliConnection, args []string, c HTTPClient, log Logger, w io.Writer)
