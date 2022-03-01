package command

import (
	"net/http"
	"os"
	"strings"

	"code.cloudfoundry.org/cli/plugin"
	logcache "code.cloudfoundry.org/go-log-cache"
)

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

func newLogCacheClient(c HTTPClient, log Logger, cli plugin.CliConnection) *logcache.Client {
	addr := os.Getenv("LOG_CACHE_ADDR")
	if addr == "" {
		addrAPI, err := cli.ApiEndpoint()
		if err != nil {
			log.Fatalf("Could not determine Log Cache endpoint: %s", err)
		}
		addr = strings.Replace(addrAPI, "api", "log-cache", 1)
	}

	if strings.ToLower(os.Getenv("LOG_CACHE_SKIP_AUTH")) != "true" {
		c = &tokenHTTPClient{
			c: c,
			tokenFunc: func() string {
				token, err := cli.AccessToken()
				if err != nil {
					log.Fatalf("Unable to get Access Token: %s", err)
				}
				return token
			},
		}
	}

	return logcache.NewClient(
		addr,
		logcache.WithHTTPClient(c),
	)
}
