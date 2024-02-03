package command

import (
	"net/http"

	"code.cloudfoundry.org/cli/plugin"
)

type Transport struct {
	tokenFunc func() (string, error)
	rt        http.RoundTripper
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	accessToken, err := t.tokenFunc()
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", accessToken)
	return t.rt.RoundTrip(req)
}

func NewHTTPClient(conn plugin.CliConnection, skipSSL bool) *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.TLSClientConfig.InsecureSkipVerify = skipSSL
	return &http.Client{
		Transport: &Transport{
			tokenFunc: func() (string, error) {
				return conn.AccessToken()
			},
			rt: t,
		},
	}
}
