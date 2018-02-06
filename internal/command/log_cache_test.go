package command_test

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
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
		timeFormat string
	)

	BeforeEach(func() {
		startTime = time.Now().Truncate(time.Second).Add(time.Minute)
		timeFormat = "2006-01-02T15:04:05.00-0700"
		logger = &stubLogger{}
		httpClient = newStubHTTPClient(responseBody(startTime))
		cliConn = newStubCliConnection()
		cliConn.cliCommandResult = []string{"app-guid"}
		cliConn.usernameResp = "a-user"
		cliConn.orgName = "organization"
		cliConn.spaceName = "space"
	})

	It("reports successful results", func() {
		command.LogCache(context.Background(), cliConn, []string{"app-name"}, httpClient, logger)
		Expect(httpClient.requestURLs).To(HaveLen(1))

		requestURL, err := url.Parse(httpClient.requestURLs[0])
		end, err := strconv.ParseInt(requestURL.Query().Get("end_time"), 10, 64)
		Expect(err).ToNot(HaveOccurred())
		Expect(end).To(BeNumerically("~", time.Now().UnixNano(), 10000000))

		logFormat := "   %s [APP/PROC/WEB/0] %s log body"
		Expect(logger.printfMessages).To(Equal([]string{
			fmt.Sprintf(
				"Retrieving logs for app %s in org %s / space %s as %s...",
				"app-name",
				cliConn.orgName,
				cliConn.spaceName,
				cliConn.usernameResp,
			),
			"",
			fmt.Sprintf(logFormat, startTime.Format(timeFormat), "ERR"),
			fmt.Sprintf(logFormat, startTime.Add(1*time.Second).Format(timeFormat), "OUT"),
			fmt.Sprintf(logFormat, startTime.Add(2*time.Second).Format(timeFormat), "OUT"),
		}))
	})

	It("reports successful results with deprecated tags", func() {
		httpClient.responseBody = []string{
			deprecatedTagsResponseBody(startTime),
		}
		command.LogCache(context.Background(), cliConn, []string{"app-name"}, httpClient, logger)

		Expect(httpClient.requestURLs).To(HaveLen(1))
		requestURL, err := url.Parse(httpClient.requestURLs[0])
		end, err := strconv.ParseInt(requestURL.Query().Get("end_time"), 10, 64)
		Expect(err).ToNot(HaveOccurred())
		Expect(end).To(BeNumerically("~", time.Now().UnixNano(), 10000000))
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

	It("reports successful results with counter envelopes", func() {
		httpClient.responseBody = []string{
			counterResponseBody(startTime),
		}
		command.LogCache(context.Background(), cliConn, []string{"app-name"}, httpClient, logger)

		Expect(httpClient.requestURLs).To(HaveLen(1))
		requestURL, err := url.Parse(httpClient.requestURLs[0])
		end, err := strconv.ParseInt(requestURL.Query().Get("end_time"), 10, 64)
		Expect(err).ToNot(HaveOccurred())
		Expect(end).To(BeNumerically("~", time.Now().UnixNano(), 10000000))
		logFormat := "   %s COUNTER %s:%d"
		Expect(logger.printfMessages).To(Equal([]string{
			fmt.Sprintf(
				"Retrieving logs for app %s in org %s / space %s as %s...",
				"app-name",
				cliConn.orgName,
				cliConn.spaceName,
				cliConn.usernameResp,
			),
			"",
			fmt.Sprintf(logFormat, startTime.Format(timeFormat), "some-name", 99),
		}))
	})

	It("reports successful results with guage envelopes", func() {
		httpClient.responseBody = []string{
			gaugeResponseBody(startTime),
		}
		command.LogCache(context.Background(), cliConn, []string{"app-name"}, httpClient, logger)

		Expect(httpClient.requestURLs).To(HaveLen(1))
		requestURL, err := url.Parse(httpClient.requestURLs[0])
		end, err := strconv.ParseInt(requestURL.Query().Get("end_time"), 10, 64)
		Expect(err).ToNot(HaveOccurred())
		Expect(end).To(BeNumerically("~", time.Now().UnixNano(), 10000000))
		logFormat := "   %s GAUGE %s:%f %s %s:%f %s"
		Expect(logger.printfMessages).To(Equal([]string{
			fmt.Sprintf(
				"Retrieving logs for app %s in org %s / space %s as %s...",
				"app-name",
				cliConn.orgName,
				cliConn.spaceName,
				cliConn.usernameResp,
			),
			"",
			fmt.Sprintf(logFormat, startTime.Format(timeFormat), "some-name", 99.0, "my-unit", "some-other-name", 101.0, "my-unit"),
		}))
	})

	It("reports successful results with timer envelopes", func() {
		httpClient.responseBody = []string{
			timerResponseBody(startTime),
		}
		ctx, _ := context.WithTimeout(context.Background(), 250*time.Millisecond)
		command.LogCache(ctx, cliConn, []string{"app-name"}, httpClient, logger)

		Expect(httpClient.requestURLs).To(HaveLen(1))
		requestURL, err := url.Parse(httpClient.requestURLs[0])
		end, err := strconv.ParseInt(requestURL.Query().Get("end_time"), 10, 64)
		Expect(err).ToNot(HaveOccurred())
		Expect(end).To(BeNumerically("~", time.Now().UnixNano(), 10000000))
		Expect(logger.printfMessages).To(ConsistOf(
			fmt.Sprintf(
				"Retrieving logs for app %s in org %s / space %s as %s...",
				"app-name",
				cliConn.orgName,
				cliConn.spaceName,
				cliConn.usernameResp,
			),
			"",
			And(ContainSubstring(startTime.Format(timeFormat)), ContainSubstring("start"), ContainSubstring("stop")),
		))
	})

	It("writes out json", func() {
		httpClient.responseBody = []string{
			mixedResponseBody(startTime),
		}

		ctx, _ := context.WithTimeout(context.Background(), 250*time.Millisecond)
		args := []string{"--envelope-type", "any", "--json", "app-name"}
		command.LogCache(ctx, cliConn, args, httpClient, logger)

		Expect(logger.printfMessages).To(ConsistOf(
			fmt.Sprintf(`{"timestamp":"%d","event":{"title":"some-title","body":"some-body"}}`, startTime.UnixNano()),
			fmt.Sprintf(`{"timestamp":"%d","timer":{"name":"http","start":"1517940773000000000","stop":"1517940773000000000"}}`, startTime.UnixNano()),
			fmt.Sprintf(`{"timestamp":"%d","gauge":{"metrics":{"some-name":{"unit":"my-unit","value":99}}}}`, startTime.UnixNano()),
			fmt.Sprintf(`{"timestamp":"%d","instanceId":"0","counter":{"name":"some-name","total":"99"}}`, startTime.UnixNano()),
			fmt.Sprintf(`{"timestamp":"%d","instanceId":"0","tags":{"source_type":"APP/PROC/WEB"},"log":{"payload":"bG9nIGJvZHk="}}`, startTime.UnixNano()),
		))
	})

	It("reports successful results when following", func() {
		httpClient.responseBody = []string{
			responseBodyAsc(startTime),
			responseBodyAsc(startTime.Add(3 * time.Second)),
		}
		logFormat := "   %s [APP/PROC/WEB/0] %s log body"

		ctx, _ := context.WithTimeout(context.Background(), 250*time.Millisecond)
		command.LogCache(ctx, cliConn, []string{"--follow", "app-name"}, httpClient, logger)

		Expect(httpClient.requestURLs).ToNot(BeEmpty())
		requestURL, err := url.Parse(httpClient.requestURLs[0])

		now := time.Now()

		start, err := strconv.ParseInt(requestURL.Query().Get("start_time"), 10, 64)
		Expect(err).ToNot(HaveOccurred())
		Expect(start).To(BeNumerically("~", now.UnixNano(), time.Second))

		_, ok := requestURL.Query()["end_time"]
		Expect(ok).To(BeFalse())

		envelopeType := requestURL.Query().Get("envelope_type")
		Expect(envelopeType).To(Equal("LOG"))

		Expect(logger.printfMessages).To(ConsistOf(
			fmt.Sprintf(
				"Retrieving logs for app %s in org %s / space %s as %s...",
				"app-name",
				cliConn.orgName,
				cliConn.spaceName,
				cliConn.usernameResp,
			),
			"",
			fmt.Sprintf(logFormat, startTime.Format(timeFormat), "OUT"),
			fmt.Sprintf(logFormat, startTime.Add(1*time.Second).Format(timeFormat), "OUT"),
			fmt.Sprintf(logFormat, startTime.Add(2*time.Second).Format(timeFormat), "ERR"),
			fmt.Sprintf(logFormat, startTime.Add(3*time.Second).Format(timeFormat), "OUT"),
			fmt.Sprintf(logFormat, startTime.Add(4*time.Second).Format(timeFormat), "OUT"),
			fmt.Sprintf(logFormat, startTime.Add(5*time.Second).Format(timeFormat), "ERR"),
		))
	})

	It("follow retries for empty responses", func() {
		httpClient.responseBody = nil

		go command.LogCache(context.Background(), cliConn, []string{"--follow", "app-name"}, httpClient, logger)

		Eventually(httpClient.requestCount).Should(BeNumerically(">", 2))
	})

	It("follow retries for an error", func() {
		httpClient.responseBody = nil
		httpClient.responseErr = errors.New("some-error")

		go command.LogCache(context.Background(), cliConn, []string{"--follow", "app-name"}, httpClient, logger)

		Eventually(httpClient.requestCount).Should(BeNumerically(">", 2))
	})

	It("reports successful results with event envelopes", func() {
		httpClient.responseBody = []string{
			eventResponseBody(startTime),
		}
		command.LogCache(context.Background(), cliConn, []string{"app-name"}, httpClient, logger)

		Expect(httpClient.requestURLs).To(HaveLen(1))
		requestURL, err := url.Parse(httpClient.requestURLs[0])
		end, err := strconv.ParseInt(requestURL.Query().Get("end_time"), 10, 64)
		Expect(err).ToNot(HaveOccurred())
		Expect(end).To(BeNumerically("~", time.Now().UnixNano(), 10000000))
		logFormat := "   %s EVENT %s:%s"
		Expect(logger.printfMessages).To(Equal([]string{
			fmt.Sprintf(
				"Retrieving logs for app %s in org %s / space %s as %s...",
				"app-name",
				cliConn.orgName,
				cliConn.spaceName,
				cliConn.usernameResp,
			),
			"",
			fmt.Sprintf(logFormat, startTime.Format(timeFormat), "some-title", "some-body"),
		}))
	})

	It("accepts start-time, end-time, envelope-type, and lines flags", func() {
		args := []string{
			"--start-time", "100",
			"--end-time", "123",
			"--envelope-type", "gauge", // deliberately lowercase
			"--lines", "99",
			"app-name",
		}
		command.LogCache(context.Background(), cliConn, args, httpClient, logger)

		Expect(httpClient.requestURLs).To(HaveLen(1))
		requestURL, err := url.Parse(httpClient.requestURLs[0])
		Expect(err).ToNot(HaveOccurred())
		Expect(requestURL.Scheme).To(Equal("https"))
		Expect(requestURL.Host).To(Equal("log-cache.some-system.com"))
		Expect(requestURL.Path).To(Equal("/v1/read/app-guid"))
		Expect(requestURL.Query().Get("start_time")).To(Equal("100"))
		Expect(requestURL.Query().Get("end_time")).To(Equal("123"))
		Expect(requestURL.Query().Get("envelope_type")).To(Equal("GAUGE"))
		Expect(requestURL.Query().Get("descending")).To(Equal("true"))
		Expect(requestURL.Query().Get("limit")).To(Equal("99"))
	})

	It("defaults lines flag to 10", func() {
		args := []string{
			"app-name",
		}
		command.LogCache(context.Background(), cliConn, args, httpClient, logger)

		Expect(httpClient.requestURLs).To(HaveLen(1))
		requestURL, err := url.Parse(httpClient.requestURLs[0])
		Expect(err).ToNot(HaveOccurred())
		Expect(requestURL.Query().Get("limit")).To(Equal("10"))
	})

	It("requests the app guid", func() {
		args := []string{"some-app"}
		command.LogCache(context.Background(), cliConn, args, httpClient, logger)

		Expect(cliConn.cliCommandArgs).To(HaveLen(3))
		Expect(cliConn.cliCommandArgs[0]).To(Equal("app"))
		Expect(cliConn.cliCommandArgs[1]).To(Equal("some-app"))
		Expect(cliConn.cliCommandArgs[2]).To(Equal("--guid"))
	})

	It("places the JWT in the 'Authorization' header", func() {
		args := []string{"some-app"}
		cliConn.accessToken = "bearer some-token"
		command.LogCache(context.Background(), cliConn, args, httpClient, logger)

		Expect(httpClient.requestHeaders).To(HaveLen(1))
		Expect(httpClient.requestHeaders[0]).To(HaveLen(1))
		Expect(httpClient.requestHeaders[0].Get("Authorization")).To(Equal("bearer some-token"))
	})

	It("formats the output via text/template", func() {
		httpClient.responseBody = []string{responseBody(time.Unix(0, 1))}
		args := []string{
			"--output-format", `{{.Timestamp}} {{printf "%s" .GetLog.GetPayload}}`,
			"app-guid",
		}

		command.LogCache(context.Background(), cliConn, args, httpClient, logger)

		Expect(logger.printfMessages).To(ContainElement("1 log body"))
	})

	It("errors if output-format and json flags are given", func() {
		httpClient.responseBody = []string{responseBody(time.Unix(0, 1))}
		args := []string{
			"--output-format", `{{.Timestamp}} {{printf "%s" .GetLog.GetPayload}}`,
			"--json",
			"app-guid",
		}

		Expect(func() {
			command.LogCache(context.Background(), cliConn, args, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("Cannot use output-format and json flags together"))
	})

	It("errors if an output-format is malformed", func() {
		args := []string{"--output-format", "{{INVALID}}", "app-guid"}
		Expect(func() {
			command.LogCache(context.Background(), cliConn, args, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal(`template: OutputFormat:1: function "INVALID" not defined`))
	})

	It("errors if an output-format won't execute", func() {
		httpClient.responseBody = []string{`{"envelopes":{"batch":[{"source_id": "a", "timestamp": 1},{"source_id":"b", "timestamp":2}]}}`}
		args := []string{
			"--output-format", "{{.invalid 9}}",
			"app-guid",
		}

		Expect(func() {
			command.LogCache(context.Background(), cliConn, args, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal(`Output template parsed, but failed to execute: template: OutputFormat:1:2: executing "OutputFormat" at <.invalid>: can't evaluate field invalid in type *loggregator_v2.Envelope`))
	})

	It("fatally logs if lines is greater than 1000 or less than 1", func() {
		args := []string{
			"--lines", "1001",
			"some-app",
		}
		Expect(func() {
			command.LogCache(context.Background(), cliConn, args, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("Lines must be 1 to 1000."))

		args = []string{
			"--lines", "0",
			"some-app",
		}
		Expect(func() {
			command.LogCache(context.Background(), cliConn, args, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("Lines must be 1 to 1000."))
	})

	It("fatally logs if app name is unknown", func() {
		args := []string{"unknown-app"}
		cliConn.cliCommandErr = errors.New("some-error")
		Expect(func() {
			command.LogCache(context.Background(), cliConn, args, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("some-error"))
	})

	It("fatally logs if username cannot be fetched", func() {
		cliConn.usernameErr = errors.New("unknown user")
		args := []string{"app-name"}

		Expect(func() {
			command.LogCache(context.Background(), cliConn, args, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("unknown user"))
	})

	It("fatally logs if org name cannot be fetched", func() {
		cliConn.orgErr = errors.New("Organization could not be fetched")
		args := []string{"app-name"}

		Expect(func() {
			command.LogCache(context.Background(), cliConn, args, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("Organization could not be fetched"))
	})

	It("fatally logs if space cannot be fetched", func() {
		cliConn.spaceErr = errors.New("unknown space")
		args := []string{"app-name"}

		Expect(func() {
			command.LogCache(context.Background(), cliConn, args, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("unknown space"))
	})

	It("fatally logs if the start > end", func() {
		args := []string{"--start-time", "1000", "--end-time", "100", "app-name"}
		Expect(func() {
			command.LogCache(context.Background(), cliConn, args, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("Invalid date/time range. Ensure your start time is prior or equal the end time."))
	})

	It("allows for empty end time with populated start time", func() {
		args := []string{"--start-time", "1000", "app-name"}
		Expect(func() {
			command.LogCache(context.Background(), cliConn, args, httpClient, logger)
		}).ToNot(Panic())
	})

	It("fatally logs if too many arguments are given", func() {
		Expect(func() {
			command.LogCache(context.Background(), cliConn, []string{"one", "two"}, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("Expected 1 argument, got 2."))
	})

	It("fatally logs if not enough arguments are given", func() {
		Expect(func() {
			command.LogCache(context.Background(), cliConn, []string{}, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("Expected 1 argument, got 0."))
	})

	It("errors if there is an error while getting API endpoint", func() {
		cliConn.apiEndpointErr = errors.New("some-error")

		Expect(func() {
			command.LogCache(context.Background(), cliConn, []string{"app-name"}, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("some-error"))
	})

	It("errors if there is no API endpoint", func() {
		cliConn.hasAPIEndpoint = false

		Expect(func() {
			command.LogCache(context.Background(), cliConn, []string{"app-name"}, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("No API endpoint targeted."))
	})

	It("errors if there is an error while checking for API endpoint", func() {
		cliConn.hasAPIEndpoint = true
		cliConn.hasAPIEndpointErr = errors.New("some-error")

		Expect(func() {
			command.LogCache(context.Background(), cliConn, []string{"app-name"}, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("some-error"))
	})

	It("errors if the request returns an error", func() {
		httpClient.responseErr = errors.New("some-error")

		Expect(func() {
			command.LogCache(context.Background(), cliConn, []string{"app-name"}, httpClient, logger)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("some-error"))
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
	mu            sync.Mutex
	responseCount int
	responseBody  []string
	responseCode  int
	responseErr   error

	requestURLs    []string
	requestHeaders []http.Header
}

func newStubHTTPClient(payload string) *stubHTTPClient {
	return &stubHTTPClient{
		responseCode: http.StatusOK,
		responseBody: []string{payload},
	}
}

func (s *stubHTTPClient) Do(r *http.Request) (*http.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.requestURLs = append(s.requestURLs, r.URL.String())
	s.requestHeaders = append(s.requestHeaders, r.Header)

	var body string
	if s.responseCount < len(s.responseBody) {
		body = s.responseBody[s.responseCount]
	}

	resp := &http.Response{
		StatusCode: s.responseCode,
		Body: ioutil.NopCloser(
			strings.NewReader(body),
		),
	}

	s.responseCount++

	return resp, s.responseErr
}

func (s *stubHTTPClient) requestCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.requestURLs)
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

	accessToken    string
	accessTokenErr error
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

func (s *stubCliConnection) AccessToken() (string, error) {
	return s.accessToken, s.accessTokenErr
}

func responseBody(startTime time.Time) string {
	// NOTE: These are in descending order.
	return fmt.Sprintf(responseTemplate,
		startTime.Add(2*time.Second).UnixNano(),
		startTime.Add(1*time.Second).UnixNano(),
		startTime.UnixNano(),
	)
}

func responseBodyAsc(startTime time.Time) string {
	// NOTE: These are in descending order.
	return fmt.Sprintf(responseTemplate,
		startTime.UnixNano(),
		startTime.Add(1*time.Second).UnixNano(),
		startTime.Add(2*time.Second).UnixNano(),
	)
}

func deprecatedTagsResponseBody(startTime time.Time) string {
	// NOTE: These are in descending order.
	return fmt.Sprintf(deprecatedTagsResponseTemplate,
		startTime.Add(2*time.Second).UnixNano(),
		startTime.Add(1*time.Second).UnixNano(),
		startTime.UnixNano(),
	)
}

func counterResponseBody(startTime time.Time) string {
	return fmt.Sprintf(counterResponseTemplate,
		startTime.UnixNano(),
	)
}

func gaugeResponseBody(startTime time.Time) string {
	return fmt.Sprintf(gaugeResponseTemplate,
		startTime.UnixNano(),
	)
}

func timerResponseBody(startTime time.Time) string {
	return fmt.Sprintf(timerResponseTemplate,
		startTime.UnixNano(),
		startTime.Add(1*time.Second).UnixNano(),
		startTime.Add(2*time.Second).UnixNano(),
	)
}

func eventResponseBody(startTime time.Time) string {
	return fmt.Sprintf(eventResponseTemplate,
		startTime.UnixNano(),
	)
}

func mixedResponseBody(startTime time.Time) string {
	return fmt.Sprintf(mixedResponseTemplate,
		startTime.UnixNano(),
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
					"payload":"bG9nIGJvZHk=",
					"type": "ERR"
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

var counterResponseTemplate = `{
	"envelopes": {
		"batch": [
			{
				"timestamp":"%d",
				"instance_id":"0",
				"counter":{"name":"some-name","total":99}
			}
		]
	}
}`

var gaugeResponseTemplate = `{
	"envelopes": {
		"batch": [
			{
				"timestamp": "%d",
				"gauge": {
				  "metrics": {
					"some-name": {
					  "value": 99,
					  "unit":"my-unit"
					},
					"some-other-name": {
					  "value": 101,
					  "unit":"my-unit"
					}
				  }
				}
			  }
		]
	}
}`

var timerResponseTemplate = `{
	"envelopes": {
		"batch": [
			{
				"timestamp": "%d",
				"timer": {
					"name": "http",
					"start": "%d",
					"stop": "%d"
				}
			}
		]
	}
}`

var eventResponseTemplate = `{
	"envelopes": {
		"batch": [
			{
				"timestamp": "%d",
				"event": {
					"title": "some-title",
					"body": "some-body"
				}
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

var mixedResponseTemplate = `{
	"envelopes": {
		"batch": [
			{
				"timestamp":"%[1]d",
				"instance_id":"0",
				"tags":{
					"source_type":"APP/PROC/WEB"
				},
				"log":{
					"payload":"bG9nIGJvZHk="
				}
			},
			{
				"timestamp":"%[1]d",
				"instance_id":"0",
				"counter":{"name":"some-name","total":99}
			},
			{
				"timestamp":"%[1]d",
				"gauge": {
					"metrics": {
						"some-name": {
							"value": 99,
							"unit":"my-unit"
						}
					}
				}
			},
			{
				"timestamp":"%[1]d",
				"timer": {
					"name": "http",
					"start": "1517940773000000000",
					"stop": "1517940773000000000"
				}
			},
			{
				"timestamp":"%[1]d",
				"event": {
					"title": "some-title",
					"body": "some-body"
				}
			}
		]
	}
}`
