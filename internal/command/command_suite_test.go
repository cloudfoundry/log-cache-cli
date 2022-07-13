package command_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"

	"code.cloudfoundry.org/cli/plugin"
	plugin_models "code.cloudfoundry.org/cli/plugin/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestCommand(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Command Suite")
}

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

type stubWriter struct {
	bytes []byte
}

// stubWriter implements io.Writer
func (w *stubWriter) Write(p []byte) (int, error) {
	w.bytes = append(w.bytes, p...)
	return len(p), nil
}

func (w *stubWriter) lines() []string {
	return strings.Split(strings.TrimRight(string(w.bytes), "\n\t "), "\n")
}

type stubHTTPClient struct {
	mu            sync.Mutex
	responseCount int
	responseBody  []string
	responseCode  int
	responseErr   error

	requestURLs    []string
	requestHeaders []http.Header

	serverVersion string
}

func newStubHTTPClient() *stubHTTPClient {
	return &stubHTTPClient{
		responseCode:  http.StatusOK,
		responseBody:  []string{},
		serverVersion: "1.4.7",
	}
}

func (s *stubHTTPClient) Do(r *http.Request) (*http.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if r.URL.Path == "/api/v1/info" {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: ioutil.NopCloser(strings.NewReader(
				fmt.Sprintf(`{"version": "%s"}`, s.serverVersion),
			)),
		}, nil
	}

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
	sync.Mutex

	apiEndpointErr error

	hasAPIEndpoint    bool
	hasAPIEndpointErr error

	cliCommandArgs   [][]string
	cliCommandResult [][]string
	cliCommandErr    []error

	usernameResp string
	usernameErr  error
	orgName      string
	orgErr       error
	spaceName    string
	spaceErr     error

	accessTokenCount int
	accessToken      string
	accessTokenErr   error
}

func newStubCliConnection() *stubCliConnection {
	return &stubCliConnection{
		hasAPIEndpoint: true,
		accessToken:    "fake-token",
	}
}

func (s *stubCliConnection) ApiEndpoint() (string, error) {
	return "https://api.some-system.com", s.apiEndpointErr
}

func (s *stubCliConnection) HasAPIEndpoint() (bool, error) {
	return s.hasAPIEndpoint, s.hasAPIEndpointErr
}

func (s *stubCliConnection) CliCommandWithoutTerminalOutput(args ...string) ([]string, error) {
	s.cliCommandArgs = append(s.cliCommandArgs, args)
	commandIndex := len(s.cliCommandArgs) - 1

	if len(s.cliCommandResult) <= commandIndex {
		return nil, errors.New("INVALID TEST SETUP")
	}
	var err error
	if len(s.cliCommandErr) > commandIndex {
		err = s.cliCommandErr[commandIndex]
	}
	return s.cliCommandResult[commandIndex], err
}

func (s *stubCliConnection) Username() (string, error) {
	return s.usernameResp, s.usernameErr
}

func (s *stubCliConnection) GetCurrentOrg() (plugin_models.Organization, error) {
	return plugin_models.Organization{
		OrganizationFields: plugin_models.OrganizationFields{
			Name: s.orgName,
		},
	}, s.orgErr
}

func (s *stubCliConnection) GetCurrentSpace() (plugin_models.Space, error) {
	return plugin_models.Space{
		SpaceFields: plugin_models.SpaceFields{
			Name: s.spaceName,
		},
	}, s.spaceErr
}

func (s *stubCliConnection) AccessToken() (string, error) {
	s.Lock()
	defer s.Unlock()

	s.accessTokenCount++
	return s.accessToken, s.accessTokenErr
}
