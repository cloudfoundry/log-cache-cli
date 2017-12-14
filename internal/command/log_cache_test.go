package command_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/cli/plugin"
	"code.cloudfoundry.org/cli/plugin/models"
	"code.cloudfoundry.org/log-cache-cli/internal/command"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LogCache", func() {
	var (
		logger     *stubLogger
		httpClient *stubHTTPClient
		cliConn    *stubCliConnection
		startTime  time.Time
	)

	BeforeEach(func() {
		startTime = time.Now().Truncate(time.Second).Add(time.Minute)
		logger = &stubLogger{}
		httpClient = newStubHTTPClient(responseBody(startTime))
		cliConn = newStubCliConnection()
		cliConn.cliCommandResult = []string{"app-guid"}
		cliConn.usernameResp = "a-user"
		cliConn.orgName = "organization"
		cliConn.spaceName = "space"
	})

	It("reports successful results", func() {
		command.LogCache(cliConn, []string{"app-name"}, httpClient, logger)

		Expect(httpClient.requestURLs).To(HaveLen(1))
		requestURL, err := url.Parse(httpClient.requestURLs[0])
		end, err := strconv.ParseInt(requestURL.Query().Get("end_time"), 10, 64)
		Expect(err).ToNot(HaveOccurred())
		Expect(end).To(BeNumerically("~", time.Now().UnixNano(), 10000000))
		timeFormat := "2006-01-02T15:04:05.00-0700"
		logFormat := "   %s [APP/PROC/WEB/0] OUT log body"
		Expect(logger.printfMessages).To(Equal([]string{
			fmt.Sprintf(
				"Retrieving logs for app %s in org %s / space %s as %s...",
				"app-name",
				cliConn.orgName,
				cliConn.spaceName,
				cliConn.usernameResp,
			),
			"",
			fmt.Sprintf(logFormat, startTime.Format(timeFormat)),
			fmt.Sprintf(logFormat, startTime.Add(1*time.Second).Format(timeFormat)),
			fmt.Sprintf(logFormat, startTime.Add(2*time.Second).Format(timeFormat)),
		}))
	})

	It("reports successful results with deprecated tags", func() {
		httpClient.responseBody = []string{
			deprecatedTagsResponseBody(startTime),
		}
		command.LogCache(cliConn, []string{"app-name"}, httpClient, logger)

		Expect(httpClient.requestURLs).To(HaveLen(1))
		requestURL, err := url.Parse(httpClient.requestURLs[0])
		end, err := strconv.ParseInt(requestURL.Query().Get("end_time"), 10, 64)
		Expect(err).ToNot(HaveOccurred())
		Expect(end).To(BeNumerically("~", time.Now().UnixNano(), 10000000))
		timeFormat := "2006-01-02T15:04:05.00-0700"
		logFormat := "   %s [APP/PROC/WEB/0] OUT log body"
		Expect(logger.printfMessages).To(Equal([]string{
			fmt.Sprintf(
				"Retrieving logs for app %s in org %s / space %s as %s...",
				"app-name",
				cliConn.orgName,
				cliConn.spaceName,
				cliConn.usernameResp,
			),
			"",
			fmt.Sprintf(logFormat, startTime.Format(timeFormat)),
			fmt.Sprintf(logFormat, startTime.Add(1*time.Second).Format(timeFormat)),
			fmt.Sprintf(logFormat, startTime.Add(2*time.Second).Format(timeFormat)),
		}))
	})

	It("accepts start-time, end-time, envelope-type and limit flags", func() {
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
		Expect(requestURL.Path).To(Equal("/v1/read/app-guid"))
		Expect(requestURL.Query().Get("start_time")).To(Equal("100"))
		Expect(requestURL.Query().Get("end_time")).To(Equal("123"))
		Expect(requestURL.Query().Get("envelope_type")).To(Equal("log"))
		Expect(requestURL.Query().Get("limit")).To(Equal("99"))
	})

	It("accepts a recent flag", func() {
		args := []string{
			"--recent",
			"app-name",
		}
		command.LogCache(cliConn, args, httpClient, logger)

		Expect(httpClient.requestURLs).To(HaveLen(1))
		requestURL, err := url.Parse(httpClient.requestURLs[0])
		Expect(err).ToNot(HaveOccurred())
		Expect(requestURL.Path).To(Equal("/v1/read/app-guid"))
		Expect(requestURL.Query().Get("start_time")).To(BeEmpty())
		Expect(requestURL.Query().Get("envelope_type")).To(Equal("log"))
		Expect(requestURL.Query().Get("limit")).To(Equal("100"))

		end, err := strconv.ParseInt(requestURL.Query().Get("end_time"), 10, 64)
		Expect(err).ToNot(HaveOccurred())
		Expect(end).To(BeNumerically("~", time.Now().UnixNano(), 10000000))
	})

	It("requests all pages until end time is reached", func() {
		startTimeA := time.Unix(0, 0)
		startTimeB := time.Now().Truncate(time.Second)
		httpClient.responseBody = []string{
			responseBody(startTimeA),
			responseBody(startTimeB),
		}

		args := []string{
			"--recent",
			"app-name",
		}
		command.LogCache(cliConn, args, httpClient, logger)

		Expect(httpClient.requestURLs).To(HaveLen(2))
		requestURL, err := url.Parse(httpClient.requestURLs[0])
		Expect(err).ToNot(HaveOccurred())
		Expect(requestURL.Path).To(Equal("/v1/read/app-guid"))
		Expect(requestURL.Query().Get("start_time")).To(BeEmpty())
		Expect(requestURL.Query().Get("envelope_type")).To(Equal("log"))
		Expect(requestURL.Query().Get("limit")).To(Equal("100"))

		requestURL, err = url.Parse(httpClient.requestURLs[1])
		Expect(err).ToNot(HaveOccurred())
		Expect(requestURL.Path).To(Equal("/v1/read/app-guid"))
		Expect(requestURL.Query().Get("envelope_type")).To(Equal("log"))
		Expect(requestURL.Query().Get("limit")).To(Equal("100"))

		expectedStartTime := startTimeA.Add(2*time.Second + time.Nanosecond)
		Expect(requestURL.Query().Get("start_time")).To(
			Equal(strconv.FormatInt(expectedStartTime.UnixNano(), 10)),
		)

		timeFormat := "2006-01-02T15:04:05.00-0700"
		logFormat := "   %s [APP/PROC/WEB/0] OUT log body"
		Expect(logger.printfMessages).To(Equal([]string{
			fmt.Sprintf(
				"Retrieving logs for app %s in org %s / space %s as %s...",
				"app-name",
				cliConn.orgName,
				cliConn.spaceName,
				cliConn.usernameResp,
			),
			"",
			"   1969-12-31T17:00:00.00-0700 [APP/PROC/WEB/0] OUT log body",
			"   1969-12-31T17:00:01.00-0700 [APP/PROC/WEB/0] OUT log body",
			"   1969-12-31T17:00:02.00-0700 [APP/PROC/WEB/0] OUT log body",
			fmt.Sprintf(logFormat, startTimeB.Format(timeFormat)),
			fmt.Sprintf(logFormat, startTimeB.Add(1*time.Second).Format(timeFormat)),
			fmt.Sprintf(logFormat, startTimeB.Add(2*time.Second).Format(timeFormat)),
		}))
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

	It("fatally logs if username cannot be fetched", func() {
		cliConn.usernameErr = errors.New("unknown user")
		args := []string{"app-name"}

		Expect(func() {
			command.LogCache(cliConn, args, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("unknown user"))
	})

	It("fatally logs if org name cannot be fetched", func() {
		cliConn.orgErr = errors.New("Organization could not be fetched")
		args := []string{"app-name"}

		Expect(func() {
			command.LogCache(cliConn, args, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("Organization could not be fetched"))
	})

	It("fatally logs if space cannot be fetched", func() {
		cliConn.spaceErr = errors.New("unknown space")
		args := []string{"app-name"}

		Expect(func() {
			command.LogCache(cliConn, args, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("unknown space"))
	})

	It("fatally logs if the start > end", func() {
		args := []string{"--start-time", "1000", "--end-time", "100", "app-name"}
		Expect(func() {
			command.LogCache(cliConn, args, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("Invalid date/time range. Ensure your start time is prior or equal the end time."))
	})

	It("fatally logs if the limit is greater than 1000", func() {
		args := []string{"--limit", "1001", "app-name"}
		Expect(func() {
			command.LogCache(cliConn, args, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("Invalid limit value. It must be 1000 or less."))
	})

	It("allows for empty end time with populated start time", func() {
		args := []string{"--start-time", "1000", "app-name"}
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
			command.LogCache(cliConn, []string{"app-name"}, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("some-error"))
	})

	It("errors if there is no API endpoint", func() {
		cliConn.hasAPIEndpoint = false

		Expect(func() {
			command.LogCache(cliConn, []string{"app-name"}, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("No API endpoint targeted."))
	})

	It("errors if there is an error while checking for API endpoint", func() {
		cliConn.hasAPIEndpoint = true
		cliConn.hasAPIEndpointErr = errors.New("some-error")

		Expect(func() {
			command.LogCache(cliConn, []string{"app-name"}, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("some-error"))
	})

	It("errors if the response code is not 200", func() {
		httpClient.responseCode = 400

		Expect(func() {
			command.LogCache(cliConn, []string{"app-name"}, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("Expected 200 response code, but got 400."))
	})

	It("errors if the request returns an error", func() {
		httpClient.responseErr = errors.New("some-error")

		Expect(func() {
			command.LogCache(cliConn, []string{"app-name"}, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("some-error"))
	})

	It("errors if the response body has an invalid timestmap", func() {
		httpClient.responseBody = []string{
			invalidTimestampResponse,
		}

		args := []string{"--recent", "app-name"}
		Expect(func() {
			command.LogCache(cliConn, args, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal(
			`Error parsing timestamp: strconv.ParseInt: parsing "not-a-timestamp": invalid syntax`,
		))
	})

	It("errors if the payload cannot be base64 decoded", func() {
		httpClient.responseBody = []string{
			invalidPayloadResponse,
		}

		args := []string{"--recent", "app-name"}
		Expect(func() {
			command.LogCache(cliConn, args, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal(
			"Error decoding log payload: illegal base64 data at input byte 0",
		))
	})

	It("errors if the payload cannot be base64 decoded", func() {
		httpClient.responseBody = []string{"{"}

		args := []string{"--recent", "app-name"}
		Expect(func() {
			command.LogCache(cliConn, args, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal(
			"Error unmarshalling log: unexpected EOF",
		))
	})
})

type stubLogger struct {
	fatalfMessage  string
	printfMessages []string
}

func (l *stubLogger) Fatalf(format string, args ...interface{}) {
	l.fatalfMessage = fmt.Sprintf(format, args...)
	panic(l.fatalfMessage)
}

func (l *stubLogger) Printf(format string, args ...interface{}) {
	l.printfMessages = append(l.printfMessages, fmt.Sprintf(format, args...))
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

	usernameResp string
	usernameErr  error
	orgName      string
	orgErr       error
	spaceName    string
	spaceErr     error
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

func (s *stubCliConnection) Username() (string, error) {
	return s.usernameResp, s.usernameErr
}

func (s *stubCliConnection) GetCurrentOrg() (plugin_models.Organization, error) {
	return plugin_models.Organization{
		plugin_models.OrganizationFields{
			Name: s.orgName,
		},
	}, s.orgErr
}

func (s *stubCliConnection) GetCurrentSpace() (plugin_models.Space, error) {
	return plugin_models.Space{
		plugin_models.SpaceFields{
			Name: s.spaceName,
		},
	}, s.spaceErr
}

func responseBody(startTime time.Time) string {
	return fmt.Sprintf(responseTemplate,
		startTime.UnixNano(),
		startTime.Add(1*time.Second).UnixNano(),
		startTime.Add(2*time.Second).UnixNano(),
	)
}

func deprecatedTagsResponseBody(startTime time.Time) string {
	return fmt.Sprintf(deprecatedTagsResponseTemplate,
		startTime.UnixNano(),
		startTime.Add(1*time.Second).UnixNano(),
		startTime.Add(2*time.Second).UnixNano(),
	)
}

var responseTemplate = `{
	"envelopes": {
		"batch": [
			{
				"timestamp":"%d",
				"instance_id":"0",
				"tags":{
					"source_type":"APP/PROC/WEB"
				},
				"log":{
					"payload":"bG9nIGJvZHk="
				}
			},
			{
				"timestamp":"%d",
				"instance_id":"0",
				"tags":{
					"source_type":"APP/PROC/WEB"
				},
				"log":{
					"payload":"bG9nIGJvZHk="
				}
			},
			{
				"timestamp":"%d",
				"instance_id":"0",
				"tags":{
					"source_type":"APP/PROC/WEB"
				},
				"log":{
					"payload":"bG9nIGJvZHk="
				}
			}
		]
	}
}`

var deprecatedTagsResponseTemplate = `{
	"envelopes": {
		"batch": [
			{
				"timestamp":"%d",
				"instance_id":"0",
				"deprecated_tags": {
					"source_type":{"text":"APP/PROC/WEB"}
				},
				"log":{"payload":"bG9nIGJvZHk="}
			},
			{
				"timestamp":"%d",
				"instance_id":"0",
				"deprecated_tags": {
					"source_type":{"text":"APP/PROC/WEB"}
				},
				"log":{"payload":"bG9nIGJvZHk="}
			},
			{
				"timestamp":"%d",
				"instance_id":"0",
				"deprecated_tags": {
					"source_type":{"text":"APP/PROC/WEB"}
				},
				"log":{"payload":"bG9nIGJvZHk="}
			}
		]
	}
}`

var invalidTimestampResponse = `{
	"envelopes": {
		"batch": [
			{
				"timestamp":"not-a-timestamp",
				"instance_id":"0",
				"deprecated_tags": {
					"source_type":{"text":"APP/PROC/WEB"}
				},
				"log":{"payload":"bG9nIGJvZHk="}
			}
		]
	}
}`

var invalidPayloadResponse = `{
	"envelopes": {
		"batch": [
			{
				"timestamp":"0",
				"instance_id":"0",
				"deprecated_tags": {
					"source_type":{"text":"APP/PROC/WEB"}
				},
				"log":{"payload":"~*&#"}
			}
		]
	}
}`
