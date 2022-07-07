package http

import "net/http"

// Client is the client used for HTTP requests
type Client interface {
	Do(req *http.Request) (*http.Response, error)
}

// TokenClient is an interface representing an authenticated HTTP client
type TokenClient struct {
	c         Client
	tokenFunc func() string
}

func NewTokenClient(c Client, tf func() string) *TokenClient {
	return &TokenClient{
		c:         c,
		tokenFunc: tf,
	}
}

func (c *TokenClient) Do(req *http.Request) (*http.Response, error) {
	accessToken := c.tokenFunc()
	if len(accessToken) > 0 {
		req.Header.Set("Authorization", accessToken)
	}

	return c.c.Do(req)
}
