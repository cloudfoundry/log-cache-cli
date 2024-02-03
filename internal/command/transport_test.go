package command_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/cli/plugin/pluginfakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/log-cache-cli/v4/internal/command"
)

var _ = Describe("Transport", func() {
	var (
		ts  *httptest.Server
		fcc *pluginfakes.FakeCliConnection

		req *http.Request
		err error
	)

	BeforeEach(func() {
		ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			req = r.Clone(context.Background())
		}))
		DeferCleanup(ts.Close)

		fcc = new(pluginfakes.FakeCliConnection)
		fcc.AccessTokenReturns("test-token", nil)

	})

	JustBeforeEach(func() {
		c := command.NewHTTPClient(fcc, false)
		_, err = c.Get(ts.URL)
	})

	It("sets an 'Authorization' header on the request", func() {
		Expect(req.Header.Get("Authorization")).To(Equal("test-token"))
	})

	It("returns a nil error", func() {
		Expect(err).NotTo(HaveOccurred())
	})

	Context("tokenFunc returns an error", func() {
		BeforeEach(func() {
			fcc.AccessTokenReturns("", fmt.Errorf("test-error"))
		})

		It("returns an error", func() {
			Expect(err).To(HaveOccurred())
		})
	})
})
