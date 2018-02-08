package command_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
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
		}

		cliConn.cliCommandResult = []string{capiResponse(map[string]string{"source-1": "app-1"})}
		cliConn.cliCommandErr = nil

		command.Meta(context.Background(), cliConn, nil, httpClient, logger, tableWriter)

		Expect(cliConn.cliCommandArgs).To(HaveLen(2))
		Expect(cliConn.cliCommandArgs[0]).To(Equal("curl"))
		Expect(cliConn.cliCommandArgs[1]).To(Equal("/v3/apps?guids=source-1"))

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

		Expect(httpClient.requestCount()).To(Equal(1))
	})

	It("prints source IDs without app names when CAPI doesn't return info", func() {
		httpClient.responseBody = []string{
			metaResponseInfo("source-1", "source-2"),
		}

		cliConn.cliCommandResult = []string{capiResponse(map[string]string{"source-1": "app-1"})}
		cliConn.cliCommandErr = nil

		command.Meta(context.Background(), cliConn, nil, httpClient, logger, tableWriter)

		Expect(cliConn.cliCommandArgs).To(HaveLen(2))
		Expect(cliConn.cliCommandArgs[0]).To(Equal("curl"))

		uri, err := url.Parse(cliConn.cliCommandArgs[1])
		Expect(err).ToNot(HaveOccurred())
		Expect(uri.Path).To(Equal("/v3/apps"))

		guidsParam, ok := uri.Query()["guids"]
		Expect(ok).To(BeTrue())
		Expect(len(guidsParam)).To(Equal(1))
		Expect(strings.Split(guidsParam[0], ",")).To(ConsistOf("source-1", "source-2"))

		Expect(httpClient.requestCount()).To(Equal(1))
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

	It("does not request more than 50 guids", func() {
		var guids []string
		for i := 0; i < 51; i++ {
			guids = append(guids, fmt.Sprintf("source-%d", i))
		}

		httpClient.responseBody = []string{
			metaResponseInfo(guids...),
		}

		cliConn.cliCommandResult = []string{capiResponse(map[string]string{})}
		cliConn.cliCommandErr = nil

		command.Meta(context.Background(), cliConn, nil, httpClient, logger, tableWriter)

		Expect(cliConn.cliCommandArgs).To(HaveLen(2))
		Expect(cliConn.cliCommandArgs[0]).To(Equal("curl"))

		uri, err := url.Parse(cliConn.cliCommandArgs[1])
		Expect(err).ToNot(HaveOccurred())
		Expect(uri.Path).To(Equal("/v3/apps"))

		guidsParam, ok := uri.Query()["guids"]
		Expect(ok).To(BeTrue())
		Expect(len(guidsParam)).To(Equal(1))
		Expect(strings.Split(guidsParam[0], ",")).To(HaveLen(50))

		// 50 entries, 2 blank lines, "Retrieving..." preamble and table
		// header comes to 54 lines.
		Expect(strings.Split(tableWriter.String(), "\n")).To(HaveLen(54))
	})

	It("fatally logs when getting ApiEndpoint fails", func() {
		cliConn.apiEndpointErr = errors.New("some-error")

		Expect(func() {
			command.Meta(context.Background(), cliConn, nil, httpClient, logger, tableWriter)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(HavePrefix(`Could not determine Log Cache endpoint: some-error`))
	})

	It("fatally logs when CAPI request fails", func() {
		httpClient.responseBody = []string{
			metaResponseInfo("source-1", "source-2"),
		}

		cliConn.cliCommandResult = nil
		cliConn.cliCommandErr = errors.New("some-error")

		Expect(func() {
			command.Meta(context.Background(), cliConn, nil, httpClient, logger, tableWriter)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(HavePrefix(`Failed to make CAPI request: some-error`))
	})

	It("fatally logs when username cannot be found", func() {
		httpClient.responseBody = []string{
			metaResponseInfo("source-1"),
		}

		cliConn.cliCommandResult = []string{capiResponse(map[string]string{"source-1": "app-1"})}
		cliConn.cliCommandErr = nil

		cliConn.usernameErr = errors.New("some-error")

		Expect(func() {
			command.Meta(context.Background(), cliConn, nil, httpClient, logger, tableWriter)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal(`Could not get username: some-error`))
	})

	It("fatally logs when CAPI response is not proper JSON", func() {
		httpClient.responseBody = []string{
			metaResponseInfo("source-1", "source-2"),
		}

		cliConn.cliCommandResult = []string{"invalid"}
		cliConn.cliCommandErr = nil

		Expect(func() {
			command.Meta(context.Background(), cliConn, nil, httpClient, logger, tableWriter)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(HavePrefix(`Could not decode CAPI response: `))
	})

	It("fatally logs when Meta fails", func() {
		httpClient.responseErr = errors.New("some-error")

		Expect(func() {
			command.Meta(context.Background(), cliConn, nil, httpClient, logger, tableWriter)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal(`Failed to read Meta information: some-error`))
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
