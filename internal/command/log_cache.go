package command

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"code.cloudfoundry.org/cli/plugin"
)

// Logger is used for outputting log-cache results and errors
type Logger interface {
	Fatalf(format string, args ...interface{})
	Printf(format string, args ...interface{})
}

// HTTPClient is the client used for HTTP requests
type HTTPClient interface {
	Get(url string) (resp *http.Response, err error)
}

// LogCache will fetch the logs for a given application guid and write them to
// stdout.
func LogCache(cli plugin.CliConnection, args []string, c HTTPClient, log Logger) {
	f := flag.NewFlagSet("log-cache", flag.ExitOnError)
	start := f.Int64("start-time", 0, "")
	end := f.Int64("end-time", 0, "")

	err := f.Parse(args)
	if err != nil {
		log.Fatalf("%s", err)
	}

	if len(f.Args()) != 1 {
		log.Fatalf("Expected 1 argument, got %d.", len(f.Args()))
	}

	if *start > *end {
		log.Fatalf("Invalid date/time range. Ensure your start time is prior or equal the end time.")
	}

	hasAPI, err := cli.HasAPIEndpoint()
	if err != nil {
		log.Fatalf("%s", err)
	}

	if !hasAPI {
		log.Fatalf("No API endpoint targeted.")
	}

	tokenURL, err := cli.ApiEndpoint()
	if err != nil {
		log.Fatalf("%s", err)
	}

	query := url.Values{}
	if *start != 0 {
		query.Set("starttime", fmt.Sprintf("%d", *start))
	}

	if *end != 0 {
		query.Set("endtime", fmt.Sprintf("%d", *end))
	}

	URL, err := url.Parse(strings.Replace(tokenURL, "api", "log-cache", 1))
	URL.Path = f.Args()[0]
	URL.RawQuery = query.Encode()

	resp, err := c.Get(URL.String())
	if err != nil {
		log.Fatalf("%s", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Expected 200 response code, but got %d.", resp.StatusCode)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("%s", err)
	}

	log.Printf("%s", data)
}
