package command

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/cli/plugin"
)

const (
	timeFormat = "2006-01-02T15:04:05.00-0700"
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

	o, err := newOptions(cli, args, log)
	if err != nil {
		log.Fatalf("%s", err)
	}

	user, err := cli.Username()
	if err != nil {
		log.Fatalf("%s", err)
	}

	org, err := cli.GetCurrentOrg()
	if err != nil {
		log.Fatalf("%s", err)
	}

	space, err := cli.GetCurrentSpace()
	if err != nil {
		log.Fatalf("%s", err)
	}

	log.Printf(
		"Retrieving logs for app %s in org %s / space %s as %s...",
		o.appName,
		org.Name,
		space.Name,
		user,
	)
	log.Printf("")

	for {
		URL, err := url.Parse(strings.Replace(tokenURL, "api", "log-cache", 1))
		URL.Path = fmt.Sprintf("/v1/read/%s", o.guid)
		URL.RawQuery = o.query().Encode()

		resp, err := c.Get(URL.String())
		if err != nil {
			log.Fatalf("%s", err)
		}

		if resp.StatusCode != http.StatusOK {
			log.Fatalf("Expected 200 response code, but got %d.", resp.StatusCode)
		}

		var r response
		err = json.NewDecoder(resp.Body).Decode(&r)
		if err != nil {
			log.Fatalf("Error unmarshalling log: %s", err)
		}

		if len(r.Envelopes.Batch) == 0 {
			return
		}

		for _, e := range r.Envelopes.Batch {
			l, err := e.format()
			if err != nil {
				log.Fatalf("%s", err)
			}

			log.Printf(l)
		}

		lastEnv := r.Envelopes.Batch[len(r.Envelopes.Batch)-1]
		lastTS, err := lastEnv.timestamp()
		if err != nil {
			log.Fatalf("%s", err)
		}

		if lastTS.UnixNano() >= o.endTime {
			return
		}

		o.startTime = lastTS.UnixNano() + 1
	}
}

type options struct {
	startTime    int64
	endTime      int64
	envelopeType string
	limit        uint64

	guid    string
	appName string
}

func newOptions(cli plugin.CliConnection, args []string, log Logger) (options, error) {
	f := flag.NewFlagSet("log-cache", flag.ContinueOnError)
	start := f.Int64("start-time", 0, "")
	end := f.Int64("end-time", time.Now().UnixNano(), "")
	envelopeType := f.String("envelope-type", "", "")
	limit := f.Uint64("limit", 0, "")
	recent := f.Bool("recent", false, "")

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
		guid:         getAppGuid(f.Args()[0], cli, log),
		appName:      f.Args()[0],
	}

	if *recent {
		o.startTime = 0
		o.endTime = time.Now().UnixNano()
		o.envelopeType = "log"
		o.limit = 100
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
		query.Set("start_time", fmt.Sprintf("%d", o.startTime))
	}

	if o.endTime != 0 {
		query.Set("end_time", fmt.Sprintf("%d", o.endTime))
	}

	if o.envelopeType != "" {
		query.Set("envelope_type", o.envelopeType)
	}

	if o.limit != 0 {
		query.Set("limit", fmt.Sprintf("%d", o.limit))
	}

	return query
}

type response struct {
	Envelopes struct {
		Batch []envelope `json:"batch"`
	} `json:"envelopes"`
}

type envelope struct {
	Timestamp  string `json:"timestamp"`
	InstanceID string `json:"instance_id"`

	Tags struct {
		SourceType string `json:"source_type"`
	} `json:"tags"`

	DeprecatedTags struct {
		SourceType struct {
			Text string `json:"text"`
		} `json:"source_type"`
	} `json:"deprecated_tags"`

	Log struct {
		Payload string `json:"payload"`
	} `json:"log"`
}

func (e envelope) format() (string, error) {
	ts, err := e.timestamp()
	if err != nil {
		return "", err
	}

	body, err := e.body()
	if err != nil {
		return "", err
	}

	// TODO DO NOT HARDCODE OUT
	return fmt.Sprintf("   %s [%s] OUT %s",
		ts.Format(timeFormat),
		e.source(),
		body,
	), nil
}

func (e envelope) timestamp() (time.Time, error) {
	ts, err := strconv.ParseInt(e.Timestamp, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("Error parsing timestamp: %s", err)
	}

	return time.Unix(0, ts), nil
}

func (e envelope) source() string {
	if e.Tags.SourceType != "" {
		return fmt.Sprintf("%s/%s",
			e.Tags.SourceType,
			e.InstanceID,
		)
	}
	return fmt.Sprintf("%s/%s",
		e.DeprecatedTags.SourceType.Text,
		e.InstanceID,
	)
}

func (e envelope) body() ([]byte, error) {
	str, err := base64.StdEncoding.DecodeString(e.Log.Payload)
	if err != nil {
		return nil, fmt.Errorf("Error decoding log payload: %s", err)
	}

	return str, nil
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
