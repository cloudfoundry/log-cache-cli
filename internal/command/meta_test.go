package command_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"code.cloudfoundry.org/log-cache-cli/internal/command"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Meta", func() {
	var (
		logger      *stubLogger
		httpClient  *stubHTTPClient
		cliConn     *stubCliConnection
		tableWriter *bytes.Buffer
	)

	BeforeEach(func() {
		logger = &stubLogger{}
		httpClient = newStubHTTPClient()
		cliConn = newStubCliConnection()
		cliConn.cliCommandResult = []string{"app-guid"}
		cliConn.usernameResp = "a-user"
		cliConn.orgName = "organization"
		cliConn.spaceName = "space"
		tableWriter = bytes.NewBuffer(nil)
	})
	It("returns app names with app source guids", func() {
		httpClient.responseBody = []string{
			metaResponseInfo("source-1"),
			capiResponse(map[string]string{"source-1": "app-1"}),
		}

		command.Meta(context.Background(), cliConn, httpClient, logger, tableWriter)
		Expect(httpClient.requestCount()).To(Equal(2))
		Expect(httpClient.requestURLs[1]).To(Equal("https://api.some-system.com/v3/apps?guids=source-1"))

		Expect(strings.Split(tableWriter.String(), "\n")).To(Equal([]string{
			fmt.Sprintf(
				"Retrieving log cache metadata as %s...",
				cliConn.usernameResp,
			),
			"",
			"Source ID  App Name",
			"source-1   app-1",
			"",
		}))

	})

	It("prints source IDs without app names when CAPI doesn't return info", func() {
		httpClient.responseBody = []string{
			metaResponseInfo("source-1", "source-2"),
			capiResponse(map[string]string{"source-1": "app-1"}),
		}

		command.Meta(context.Background(), cliConn, httpClient, logger, tableWriter)
		Expect(httpClient.requestCount()).To(Equal(2))
		Expect(httpClient.requestURLs[1]).To(Equal("https://api.some-system.com/v3/apps?guids=source-1,source-2"))

		Expect(strings.Split(tableWriter.String(), "\n")).To(Equal([]string{
			fmt.Sprintf(
				"Retrieving log cache metadata as %s...",
				cliConn.usernameResp,
			),
			"",
			"Source ID  App Name",
			"source-1   app-1",
			"source-2",
			"",
		}))

	})

})

func metaResponseInfo(sourceIDs ...string) string {
	var metaInfos []string
	for _, sourceID := range sourceIDs {
		metaInfos = append(metaInfos, fmt.Sprintf(`"%s": {}`, sourceID))
	}
	return fmt.Sprintf(`{ "meta": { %s }}`, strings.Join(metaInfos, ","))
}

func capiResponse(apps map[string]string) string {
	var resources []string
	for appID, appName := range apps {
		resources = append(resources, fmt.Sprintf(`{"guid": "%s", "name": "%s"}`, appID, appName))
	}
	return fmt.Sprintf(`{ "resources": [%s] }`, strings.Join(resources, ","))
}
