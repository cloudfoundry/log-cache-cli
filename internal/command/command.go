package command

import (
	"context"
	"io"
	"net/http"

	"code.cloudfoundry.org/cli/plugin"
)

// Command is the interface to implement plugin commands
type Command func(ctx context.Context, cli plugin.CliConnection, args []string, c HTTPClient, log Logger, w io.Writer)

// HTTPClient is the client used for HTTP requests
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}
