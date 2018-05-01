package command_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/log-cache-cli/pkg/command"
)

var _ = Describe("Execute", func() {
	It("wires up the meta cmd", func() {
		cleanup := patchArgs([]string{"--help"})
		defer cleanup()
		var buf bytes.Buffer

		err := command.Execute(command.WithOutput(&buf))

		Expect(err).ToNot(HaveOccurred())
		Expect(buf.String()).To(ContainSubstring("List cluster logs and metrics"))
	})

	It("wires up tail cmd", func() {
		cleanup := patchArgs([]string{"tail", "--help"})
		defer cleanup()
		var buf bytes.Buffer

		err := command.Execute(command.WithOutput(&buf))

		Expect(err).ToNot(HaveOccurred())
		Expect(buf.String()).To(ContainSubstring("Output logs and metrics for a given source-id"))
	})

	It("runs meta by default", func() {
		cleanup := patchArgs(nil)
		defer cleanup()
		paths := make(chan string, 100)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			paths <- r.URL.Path
		}))
		defer server.Close()
		cleanup = patchEnv("LOG_CACHE_ADDR", server.URL)
		defer cleanup()

		err := command.Execute()

		Expect(err).ToNot(HaveOccurred())
		Eventually(paths).Should(Receive(Equal("/v1/meta")))
	})

	It("returns an error if it can't build config", func() {
		cleanup := patchEnv("HOME", "/does/not/exist")
		defer cleanup()

		err := command.Execute()

		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
	})
})

func patchArgs(args []string) func() {
	orig := os.Args
	new := make([]string, 0, len(args)+1)
	new = append(new, orig[0])
	new = append(new, args...)
	os.Args = new

	return func() {
		os.Args = orig
	}
}
