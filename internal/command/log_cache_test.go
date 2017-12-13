package command_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"code.cloudfoundry.org/cli/plugin"
	"code.cloudfoundry.org/log-cache-cli/internal/command"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LogCache", func() {
	var (
		logger     *stubLogger
		httpClient *stubHTTPClient
		cliConn    *stubCliConnection
	)

	BeforeEach(func() {
		logger = &stubLogger{}
		httpClient = newStubHTTPClient(validPayload())
		cliConn = newStubCliConnection()
	})

	It("reports successful results", func() {
		cliConn.cliCommandResult = []string{"app-guid"}

		command.LogCache(cliConn, []string{"app-name"}, httpClient, logger)

		Expect(httpClient.requestURLs).To(HaveLen(1))
		Expect(httpClient.requestURLs[0]).To(Equal("https://log-cache.some-system.com/app-guid"))
		Expect(logger.printfMessage).To(Equal(validPayload()))
	})

	It("accepts start-time, end-time, envelope-type and limit flags", func() {
		cliConn.cliCommandResult = []string{"app-guid"}

		args := []string{
			"--start-time", "100",
			"--end-time", "123",
			"--envelope-type", "log",
			"--limit", "99",
			"app-name",
		}
		command.LogCache(cliConn, args, httpClient, logger)

		Expect(httpClient.requestURLs).To(HaveLen(1))
		requestURL, err := url.Parse(httpClient.requestURLs[0])
		Expect(err).ToNot(HaveOccurred())
		Expect(requestURL.Scheme).To(Equal("https"))
		Expect(requestURL.Host).To(Equal("log-cache.some-system.com"))
		Expect(requestURL.Path).To(Equal("/app-guid"))
		Expect(requestURL.Query().Get("starttime")).To(Equal("100"))
		Expect(requestURL.Query().Get("endtime")).To(Equal("123"))
		Expect(requestURL.Query().Get("envelopetype")).To(Equal("log"))
		Expect(requestURL.Query().Get("limit")).To(Equal("99"))
	})

	It("requests the app guid", func() {
		args := []string{"some-app"}
		command.LogCache(cliConn, args, httpClient, logger)

		Expect(cliConn.cliCommandArgs).To(HaveLen(3))
		Expect(cliConn.cliCommandArgs[0]).To(Equal("app"))
		Expect(cliConn.cliCommandArgs[1]).To(Equal("some-app"))
		Expect(cliConn.cliCommandArgs[2]).To(Equal("--guid"))
	})

	It("fatally logs if app name is unknown", func() {
		args := []string{"unknown-app"}
		cliConn.cliCommandErr = errors.New("some-error")
		Expect(func() {
			command.LogCache(cliConn, args, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("some-error"))
	})

	It("fatally logs if the start > end", func() {
		args := []string{"--start-time", "1000", "--end-time", "100", "app-guid"}
		Expect(func() {
			command.LogCache(cliConn, args, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("Invalid date/time range. Ensure your start time is prior or equal the end time."))
	})

	It("fatally logs if the limit is greater than 1000", func() {
		args := []string{"--limit", "1001", "app-guid"}
		Expect(func() {
			command.LogCache(cliConn, args, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("Invalid limit value. It must be 1000 or less."))
	})

	It("allows for empty end time with populated start time", func() {
		args := []string{"--start-time", "1000", "app-guid"}
		Expect(func() {
			command.LogCache(cliConn, args, httpClient, logger)
		}).ToNot(Panic())
	})

	It("fatally logs if too many arguments are given", func() {
		Expect(func() {
			command.LogCache(cliConn, []string{"one", "two"}, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("Expected 1 argument, got 2."))
	})

	It("fatally logs if not enough arguments are given", func() {
		Expect(func() {
			command.LogCache(cliConn, []string{}, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("Expected 1 argument, got 0."))
	})

	It("errors if there is an error while getting API endpoint", func() {
		cliConn.apiEndpointErr = errors.New("some-error")

		Expect(func() {
			command.LogCache(cliConn, []string{"app-guid"}, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("some-error"))
	})

	It("errors if there is no API endpoint", func() {
		cliConn.hasAPIEndpoint = false

		Expect(func() {
			command.LogCache(cliConn, []string{"app-guid"}, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("No API endpoint targeted."))
	})

	It("errors if there is an error while checking for API endpoint", func() {
		cliConn.hasAPIEndpoint = true
		cliConn.hasAPIEndpointErr = errors.New("some-error")

		Expect(func() {
			command.LogCache(cliConn, []string{"app-guid"}, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("some-error"))
	})

	It("errors if the response code is not 200", func() {
		httpClient.responseCode = 400

		Expect(func() {
			command.LogCache(cliConn, []string{"app-guid"}, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("Expected 200 response code, but got 400."))
	})

	It("errors if the request returns an error", func() {
		httpClient.responseErr = errors.New("some-error")

		Expect(func() {
			command.LogCache(cliConn, []string{"app-guid"}, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("some-error"))
	})
})

type stubLogger struct {
	fatalfMessage string
	printfMessage string
}

func (l *stubLogger) Fatalf(format string, args ...interface{}) {
	l.fatalfMessage = fmt.Sprintf(format, args...)
	panic(l.fatalfMessage)
}

func (l *stubLogger) Printf(format string, args ...interface{}) {
	l.printfMessage = fmt.Sprintf(format, args...)
}

type stubHTTPClient struct {
	responseCount int
	responseBody  []string
	responseCode  int
	responseErr   error

	requestURLs []string
}

func newStubHTTPClient(payload string) *stubHTTPClient {
	return &stubHTTPClient{
		responseCode: http.StatusOK,
		responseBody: []string{payload},
	}
}

func (s *stubHTTPClient) Get(url string) (*http.Response, error) {
	s.requestURLs = append(s.requestURLs, url)

	resp := &http.Response{
		StatusCode: s.responseCode,
		Body: ioutil.NopCloser(
			strings.NewReader(s.responseBody[s.responseCount]),
		),
	}

	s.responseCount++

	return resp, s.responseErr
}

type stubCliConnection struct {
	plugin.CliConnection

	apiEndpointErr error

	hasAPIEndpoint    bool
	hasAPIEndpointErr error

	cliCommandArgs   []string
	cliCommandResult []string
	cliCommandErr    error
}

func newStubCliConnection() *stubCliConnection {
	return &stubCliConnection{
		hasAPIEndpoint: true,
	}
}

func (s *stubCliConnection) ApiEndpoint() (string, error) {
	return "https://api.some-system.com", s.apiEndpointErr
}

func (s *stubCliConnection) HasAPIEndpoint() (bool, error) {
	return s.hasAPIEndpoint, s.hasAPIEndpointErr
}

func (s *stubCliConnection) CliCommandWithoutTerminalOutput(args ...string) ([]string, error) {
	s.cliCommandArgs = args
	return s.cliCommandResult, s.cliCommandErr
}

func responseBody(startTime time.Time) string {
	return fmt.Sprintf(responseBodyTemplate,
		startTime,
		startTime.Add(1*time.Second),
		startTime.Add(2*time.Second),
	)
}

func validPayload() string {
	return `
	{
		"envelopes": [{
			"timestamp":"1000000000",
			"sourceId":"6e707796-71bf-47de-8d50-3063df307de0",
			"instanceId":"0",
			"deprecatedTags": {
				"deployment":{"text":"cf"},
				"index":{"text":"a5d9c003-3d89-469a-86df-a8b36f0ad79f"},
				"ip":{"text":"10.0.16.17"},
				"job":{"text":"diego-cell"},
				"origin":{"text":"rep"},
				"source_type":{"text":"APP/PROC/WEB"}
			},
			"tags":{"source_id":"6e707796-71bf-47de-8d50-3063df307de0"},
			"log":{"payload":"eyJtc2ciOiJMb2cgIyA0MzQwOTI1In0="}
		}]
	}`
}

var responseBodyTemplate = `{
	"envelopes": [
		{
			"timestamp":"%d",
			"sourceId":"6e707796-71bf-47de-8d50-3063df307de0",
			"instanceId":"0",
			"deprecatedTags": {
				"deployment":{"text":"cf"},
				"index":{"text":"a5d9c003-3d89-469a-86df-a8b36f0ad79f"},
				"ip":{"text":"10.0.16.17"},
				"job":{"text":"diego-cell"},
				"origin":{"text":"rep"},
				"source_type":{"text":"APP/PROC/WEB"}
			},
			"tags":{"source_id":"6e707796-71bf-47de-8d50-3063df307de0"},
			"log":{"payload":"eyJtc2ciOiJMb2cgIyA0MzQwOTI1In0="}
		},
		{
			"timestamp":"%d",
			"sourceId":"6e707796-71bf-47de-8d50-3063df307de0",
			"instanceId":"0",
			"deprecatedTags": {
				"deployment":{"text":"cf"},
				"index":{"text":"a5d9c003-3d89-469a-86df-a8b36f0ad79f"},
				"ip":{"text":"10.0.16.17"},
				"job":{"text":"diego-cell"},
				"origin":{"text":"rep"},
				"source_type":{"text":"APP/PROC/WEB"}
			},
			"tags":{"source_id":"6e707796-71bf-47de-8d50-3063df307de0"},
			"log":{"payload":"eyJtc2ciOiJMb2cgIyA0MzQwOTI1In0="}
		},
		{
			"timestamp":"%d",
			"sourceId":"6e707796-71bf-47de-8d50-3063df307de0",
			"instanceId":"0",
			"deprecatedTags": {
				"deployment":{"text":"cf"},
				"index":{"text":"a5d9c003-3d89-469a-86df-a8b36f0ad79f"},
				"ip":{"text":"10.0.16.17"},
				"job":{"text":"diego-cell"},
				"origin":{"text":"rep"},
				"source_type":{"text":"APP/PROC/WEB"}
			},
			"tags":{"source_id":"6e707796-71bf-47de-8d50-3063df307de0"},
			"log":{"payload":"eyJtc2ciOiJMb2cgIyA0MzQwOTI1In0="}
		}
	]
}`
