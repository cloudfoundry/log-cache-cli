package command_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/log-cache-cli/pkg/command"
)

var _ = Describe("Meta", func() {
	It("prints source ids returned from the api in ascending order", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, metaResponseInfo("source-id-5", "source-id-3", "source-id-4", "source-id-2"))
		}))
		defer server.Close()
		var buf bytes.Buffer
		metaCmd := command.NewMeta(command.Config{
			Addr: server.URL,
		})
		metaCmd.SetOutput(&buf)
		metaCmd.SetArgs([]string{})

		err := metaCmd.Execute()

		Expect(err).ToNot(HaveOccurred())
		Expect(strings.Split(buf.String(), "\n")).To(Equal([]string{
			"Source ID    Count   Expired  Cache Duration",
			"source-id-2  100000  85008    11m45s",
			"source-id-3  100000  85008    11m45s",
			"source-id-4  100000  85008    11m45s",
			"source-id-5  100000  85008    11m45s",
			"",
		}))
	})

	It("removes header when not writing to a tty", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, metaResponseInfo("source-id-5", "source-id-3", "source-id-4", "source-id-2"))
		}))
		defer server.Close()
		var buf bytes.Buffer
		metaCmd := command.NewMeta(command.Config{
			Addr: server.URL,
		}, command.WithMetaNoHeaders())
		metaCmd.SetOutput(&buf)
		metaCmd.SetArgs([]string{})

		err := metaCmd.Execute()

		Expect(err).ToNot(HaveOccurred())
		Expect(strings.Split(buf.String(), "\n")).To(Equal([]string{
			"source-id-2  100000  85008  11m45s",
			"source-id-3  100000  85008  11m45s",
			"source-id-4  100000  85008  11m45s",
			"source-id-5  100000  85008  11m45s",
			"",
		}))
	})

	It("doesn't return an error if the server responds with no data", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		defer server.Close()
		metaCmd := command.NewMeta(command.Config{
			Addr: server.URL,
		})
		var buf bytes.Buffer
		metaCmd.SetOutput(&buf)
		metaCmd.SetArgs([]string{})

		err := metaCmd.Execute()

		Expect(err).ToNot(HaveOccurred())
		Expect(buf.String()).To(BeEmpty())
	})

	It("returns an error if unable to retrieve meta info", func() {
		metaCmd := command.NewMeta(command.Config{
			Addr: "http://does-not-exist",
		})
		metaCmd.SetOutput(ioutil.Discard)
		metaCmd.SetArgs([]string{})

		err := metaCmd.Execute()

		Expect(err).To(MatchError(ContainSubstring("no such host")))
	})

	It("timesout when server is taking too long", func() {
		done := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-time.After(time.Second):
			case <-done:
			}
		}))
		defer server.Close()
		metaCmd := command.NewMeta(command.Config{
			Addr: server.URL,
		}, command.WithMetaTimeout(time.Nanosecond))
		metaCmd.SetOutput(ioutil.Discard)
		metaCmd.SetArgs([]string{})

		var err error
		go func() {
			defer close(done)
			err = metaCmd.Execute()
		}()

		Eventually(done, "500ms").Should(BeClosed())
		Expect(err).To(MatchError(ContainSubstring("context deadline exceeded")))
	})
})

func metaResponseInfo(sourceIDs ...string) string {
	var metaInfos []string
	for _, sourceID := range sourceIDs {
		metaInfos = append(metaInfos, fmt.Sprintf(`"%s": {
		  "count": "100000",
		  "expired": "85008",
		  "oldestTimestamp": "1519256157847077020",
		  "newestTimestamp": "1519256863126668345"
		}`, sourceID))
	}
	return fmt.Sprintf(`{ "meta": { %s }}`, strings.Join(metaInfos, ","))
}
