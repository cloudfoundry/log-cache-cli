package command

import "net/http"

// HTTPClient is the client used for HTTP requests
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// tokenHTTPClient is an interface representing an authenticated HTTP client
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
