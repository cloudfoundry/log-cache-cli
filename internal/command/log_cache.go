package command

import (
	"errors"
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

	o, err := newOptions(args)
	if err != nil {
		log.Fatalf("%s", err)
	}

	appGuid := getAppGuid(f.Args()[0], cli, log)
	URL, err := url.Parse(strings.Replace(tokenURL, "api", "log-cache", 1))
	URL.Path = o.guid
	URL.RawQuery = o.query().Encode()

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

type options struct {
	startTime    int64
	endTime      int64
	envelopeType string
	limit        uint64

	guid string
}

func newOptions(args []string) (options, error) {
	f := flag.NewFlagSet("log-cache", flag.ContinueOnError)
	start := f.Int64("start-time", 0, "")
	end := f.Int64("end-time", 0, "")
	envelopeType := f.String("envelope-type", "", "")
	limit := f.Uint64("limit", 0, "")

	err := f.Parse(args)
	if err != nil {
		return options{}, err
	}

	if len(f.Args()) != 1 {
		return options{}, fmt.Errorf("Expected 1 argument, got %d.", len(f.Args()))
	}

	o := options{
		startTime:    *start,
		endTime:      *end,
		envelopeType: *envelopeType,
		limit:        *limit,
		guid:         f.Args()[0],
	}

	return o, o.validate()
}

func (o options) validate() error {
	if o.startTime > o.endTime && o.endTime != 0 {
		return errors.New("Invalid date/time range. Ensure your start time is prior or equal the end time.")
	}

	if o.limit > 1000 {
		return errors.New("Invalid limit value. It must be 1000 or less.")
	}

	return nil
}

func (o options) query() url.Values {
	query := url.Values{}
	if o.startTime != 0 {
		query.Set("starttime", fmt.Sprintf("%d", o.startTime))
	}

	if o.endTime != 0 {
		query.Set("endtime", fmt.Sprintf("%d", o.endTime))
	}

	if o.envelopeType != "" {
		query.Set("envelopetype", o.envelopeType)
	}

	if o.limit != 0 {
		query.Set("limit", fmt.Sprintf("%d", o.limit))
	}

	return query
}

func getAppGuid(appName string, cli plugin.CliConnection, log Logger) string {
	r, err := cli.CliCommandWithoutTerminalOutput(
		"app",
		appName,
		"--guid",
	)
	if err != nil {
		log.Fatalf("%s", err)
	}

	return strings.Join(r, "")
}
