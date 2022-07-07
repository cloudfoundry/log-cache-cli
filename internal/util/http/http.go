// Package http provides HTTP client implementations.
package http

import "net/http"

// A Client is implemented by the standard library's http.Client and
// TokenClient.
type Client interface {
	Do(req *http.Request) (*http.Response, error)
}

// A TokenClient wraps an HTTP client to automatically set Authorization headers
// in requests using the provided function to generate tokens.
type TokenClient struct {
	c         Client
	tokenFunc func() string
}

// NewTokenClient returns a TokenClient given a client and token-generating
// funtion.
func NewTokenClient(c Client, tf func() string) *TokenClient {
	return &TokenClient{
		c:         c,
		tokenFunc: tf,
	}
}

// Do makes an HTTP request using the underlying client. If the token function
// returns a non-empty string then it will be set as the Authorization header of
// the request.
func (c *TokenClient) Do(req *http.Request) (*http.Response, error) {
	accessToken := c.tokenFunc()
	if len(accessToken) > 0 {
		req.Header.Set("Authorization", accessToken)
	}

	return c.c.Do(req)
}
