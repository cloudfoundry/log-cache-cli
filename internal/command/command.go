package command

import (
	"context"
	"io"
	"net/http"

	"code.cloudfoundry.org/cli/plugin"
)

// Command is the interface to implement plugin commands
type Command func(ctx context.Context, cli plugin.CliConnection, args []string, c HTTPClient, log Logger, w io.Writer)

// Logger is used for outputting log-cache results and errors
type Logger interface {
	Fatalf(format string, args ...interface{})
	Printf(format string, args ...interface{})
}

// HTTPClient is the client used for HTTP requests
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type tokenHTTPClient struct {
	c         HTTPClient
	tokenFunc func() string
}

func (c *tokenHTTPClient) Do(req *http.Request) (*http.Response, error) {
	accessToken := c.tokenFunc()
	if len(accessToken) > 0 {
		req.Header.Set("Authorization", accessToken)
	}

	return c.c.Do(req)

}
