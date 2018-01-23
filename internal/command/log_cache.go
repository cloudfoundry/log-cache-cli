package command

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"code.cloudfoundry.org/cli/plugin"
	logcache "code.cloudfoundry.org/go-log-cache"
	logcacherpc "code.cloudfoundry.org/go-log-cache/rpc/logcache"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
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
	Do(req *http.Request) (*http.Response, error)
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

	tc := &tokenHTTPClient{
		c:        c,
		getToken: cli.AccessToken,
	}

	client := logcache.NewClient(strings.Replace(tokenURL, "api", "log-cache", 1),
		logcache.WithHTTPClient(tc),
	)

	log.Printf(
		"Retrieving logs for app %s in org %s / space %s as %s...",
		o.appName,
		org.Name,
		space.Name,
		user,
	)
	log.Printf("")

	logcache.Walk(
		context.Background(),
		o.guid,
		func(b []*loggregator_v2.Envelope) bool {
			for _, e := range b {
				log.Printf("%s", envelopeWrapper{e})
			}

			lastEnv := b[len(b)-1]
			if lastEnv.Timestamp >= o.endTime.UnixNano() {
				return false
			}

			return true
		},
		client.Read,
		logcache.WithWalkStartTime(o.startTime),
		logcache.WithWalkEndTime(o.endTime),
		logcache.WithWalkEnvelopeType(o.envelopeType),
		logcache.WithWalkBackoff(newBackoff(log)),
	)
}

type options struct {
	startTime    time.Time
	endTime      time.Time
	envelopeType logcacherpc.EnvelopeTypes

	guid    string
	appName string
}

func newOptions(cli plugin.CliConnection, args []string, log Logger) (options, error) {
	f := flag.NewFlagSet("log-cache", flag.ContinueOnError)
	start := f.Int64("start-time", 0, "")
	end := f.Int64("end-time", time.Now().UnixNano(), "")
	envelopeType := f.String("envelope-type", "", "")
	recent := f.Bool("recent", false, "")

	err := f.Parse(args)
	if err != nil {
		return options{}, err
	}

	if len(f.Args()) != 1 {
		return options{}, fmt.Errorf("Expected 1 argument, got %d.", len(f.Args()))
	}

	o := options{
		startTime:    time.Unix(0, *start),
		endTime:      time.Unix(0, *end),
		envelopeType: translateEnvelopeType(*envelopeType),
		guid:         getAppGuid(f.Args()[0], cli, log),
		appName:      f.Args()[0],
	}

	if *recent {
		o.startTime = time.Unix(0, 0)
		o.endTime = time.Now()
		o.envelopeType = logcacherpc.EnvelopeTypes_LOG
	}

	return o, o.validate()
}

func (o options) validate() error {
	if o.startTime.After(o.endTime) && o.endTime != time.Unix(0, 0) {
		return errors.New("Invalid date/time range. Ensure your start time is prior or equal the end time.")
	}

	return nil
}

func translateEnvelopeType(t string) logcacherpc.EnvelopeTypes {
	t = strings.ToUpper(t)

	switch t {
	case "ANY":
		return logcacherpc.EnvelopeTypes_ANY
	case "LOG", "":
		return logcacherpc.EnvelopeTypes_LOG
	case "COUNTER":
		return logcacherpc.EnvelopeTypes_COUNTER
	case "GAUGE":
		return logcacherpc.EnvelopeTypes_GAUGE
	case "TIMER":
		return logcacherpc.EnvelopeTypes_TIMER
	case "EVENT":
		return logcacherpc.EnvelopeTypes_EVENT
	default:
		return logcacherpc.EnvelopeTypes_LOG
	}
}

type envelopeWrapper struct {
	*loggregator_v2.Envelope
}

func (e envelopeWrapper) String() string {
	ts := time.Unix(0, e.Timestamp)

	switch e.Message.(type) {
	case *loggregator_v2.Envelope_Log:
		return fmt.Sprintf("   %s [%s/%s] %s %s",
			ts.Format(timeFormat),
			e.sourceType(),
			e.InstanceId,
			e.GetLog().GetType(),
			e.GetLog().GetPayload(),
		)
	case *loggregator_v2.Envelope_Counter:
		return fmt.Sprintf("   %s COUNTER %s:%d",
			ts.Format(timeFormat),
			e.GetCounter().GetName(),
			e.GetCounter().GetTotal(),
		)
	case *loggregator_v2.Envelope_Gauge:
		var values []string
		for k, v := range e.GetGauge().GetMetrics() {
			values = append(values, fmt.Sprintf("%s:%f %s", k, v.Value, v.Unit))
		}

		sort.Sort(sort.StringSlice(values))

		return fmt.Sprintf("   %s GAUGE %s",
			ts.Format(timeFormat),
			strings.Join(values, " "),
		)
	case *loggregator_v2.Envelope_Timer:
		return fmt.Sprintf("   %s TIMER start=%d stop=%d",
			ts.Format(timeFormat),
			e.GetTimer().GetStart(),
			e.GetTimer().GetStop(),
		)
	case *loggregator_v2.Envelope_Event:
		return fmt.Sprintf("   %s EVENT %s:%s",
			ts.Format(timeFormat),
			e.GetEvent().GetTitle(),
			e.GetEvent().GetBody(),
		)
	default:
		return e.Envelope.String()
	}
}

func (e envelopeWrapper) sourceType() string {
	st, ok := e.Tags["source_type"]
	if !ok {
		t, ok := e.DeprecatedTags["source_type"]
		if !ok {
			return "unknown"
		}

		return t.GetText()
	}

	return st
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

type backoff struct {
	logcache.AlwaysDoneBackoff

	logger Logger
}

func newBackoff(log Logger) backoff {
	return backoff{logger: log}
}

func (b backoff) OnErr(err error) bool {
	b.logger.Fatalf("%s", err)
	return b.AlwaysDoneBackoff.OnErr(err)
}

type tokenHTTPClient struct {
	c        HTTPClient
	getToken func() (string, error)
}

func (c *tokenHTTPClient) Do(req *http.Request) (*http.Response, error) {
	token, err := c.getToken()
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", token)

	return c.c.Do(req)
}
