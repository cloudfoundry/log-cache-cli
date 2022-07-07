package http

import (
	"net/http"
	"testing"
)

func TestTokenClient(t *testing.T) {
	mc := &mockClient{}
	tc := NewTokenClient(mc, func() string {
		return "test"
	})

	r, err := http.NewRequest("GET", "fakeurl", nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := tc.Do(r)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("got %d, want %d", resp.StatusCode, 200)
	}

	auth := mc.lastReq.Header.Get("Authorization")
	if auth != "test" {
		t.Errorf("got %s, want %s", auth, "test")
	}
}

type mockClient struct {
	lastReq http.Request
}

func (c *mockClient) Do(req *http.Request) (*http.Response, error) {
	c.lastReq = *req
	return &http.Response{StatusCode: 200}, nil
}
