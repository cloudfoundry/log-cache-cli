package command

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"text/template"
	"time"

	"code.cloudfoundry.org/cli/plugin"
	logcache "code.cloudfoundry.org/go-log-cache"
	logcacherpc "code.cloudfoundry.org/go-log-cache/rpc/logcache"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
)

const (
	timeFormat = "2006-01-02T15:04:05.00-0700"
)

// Command is the interface to implement plugin commands
type Command func(ctx context.Context, cli plugin.CliConnection, args []string, c HTTPClient, log Logger, w io.Writer)

// Logger is used for outputting log-cache results and errors
type Logger interface {
	Fatalf(format string, args ...interface{})
}

// HTTPClient is the client used for HTTP requests
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Tail will fetch the logs for a given application guid and write them to
// stdout.
func Tail(ctx context.Context, cli plugin.CliConnection, args []string, c HTTPClient, log Logger, w io.Writer) {
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

	lw := lineWriter{w: w}

	tc := &tokenHTTPClient{
		c:        c,
		getToken: cli.AccessToken,
	}

	client := logcache.NewClient(strings.Replace(tokenURL, "api", "log-cache", 1),
		logcache.WithHTTPClient(tc),
	)

	formatter := newFormatter(formatterKindFromOptions(o), log, o.outputTemplate)

	guid := o.guid
	headerPrinter := formatter.appHeader
	if guid == "" {
		// fall back to provided name
		guid = o.providedName
		headerPrinter = formatter.sourceHeader
	}

	header, ok := headerPrinter(o.providedName, org.Name, space.Name, user)
	if ok {
		lw.Write(header)
		lw.Write("")
	}

	if o.gaugeName != "" {
		o.envelopeType = logcacherpc.EnvelopeTypes_GAUGE
	}

	if o.counterName != "" {
		o.envelopeType = logcacherpc.EnvelopeTypes_COUNTER
	}

	filterAndFormat := func(e *loggregator_v2.Envelope) (string, bool) {
		if !nameFilter(e, o) || !typeFilter(e, o) {
			return "", false
		}

		return formatter.formatEnvelope(e)
	}

	if o.follow {
		logcache.Walk(
			ctx,
			guid,
			logcache.Visitor(func(envelopes []*loggregator_v2.Envelope) bool {
				for _, e := range envelopes {
					if formatted, ok := filterAndFormat(e); ok {
						lw.Write(formatted)
					}
				}
				return true
			}),
			client.Read,
			logcache.WithWalkStartTime(time.Now().Add(-5*time.Second)),
			logcache.WithWalkEnvelopeType(o.envelopeType),
			logcache.WithWalkBackoff(logcache.NewAlwaysRetryBackoff(250*time.Millisecond)),
		)

		return
	}

	// Lines mode
	envelopes, err := client.Read(
		context.Background(),
		guid,
		o.startTime,
		logcache.WithEndTime(o.endTime),
		logcache.WithEnvelopeType(o.envelopeType),
		logcache.WithLimit(o.lines),
		logcache.WithDescending(),
	)

	if err != nil {
		log.Fatalf("%s", err)
	}

	// we get envelopes in descending order but want to print them ascending
	for i := len(envelopes) - 1; i >= 0; i-- {
		if formatted, ok := filterAndFormat(envelopes[i]); ok {
			lw.Write(formatted)
		}
	}
}

type lineWriter struct {
	w io.Writer
}

func (w *lineWriter) Write(line string) error {
	line = strings.TrimSuffix(line, "\n") + "\n"
	_, err := w.w.Write([]byte(line))
	return err
}

const (
	envelopeClassAny envelopeClass = iota
	envelopeClassMetric
	envelopeClassLog
)

type envelopeClass int

type options struct {
	startTime     time.Time
	endTime       time.Time
	envelopeType  logcacherpc.EnvelopeTypes
	envelopeClass envelopeClass
	lines         int
	follow        bool

	guid           string
	providedName   string
	outputTemplate *template.Template
	jsonOutput     bool

	gaugeName   string
	counterName string
}

func newOptions(cli plugin.CliConnection, args []string, log Logger) (options, error) {
	f := flag.NewFlagSet("log-cache", flag.ContinueOnError)
	start := f.Int64("start-time", 0, "")
	end := f.Int64("end-time", time.Now().UnixNano(), "")
	envelopeType := f.String("envelope-type", "", "")
	lines := f.Uint("lines", 10, "")
	follow := f.Bool("follow", false, "")
	outputFormat := f.String("output-format", "", "")
	jsonOutput := f.Bool("json", false, "")
	gaugeName := f.String("gauge-name", "", "")
	counterName := f.String("counter-name", "", "")
	envelopeClass := f.String("type", "", "")

	err := f.Parse(args)
	if err != nil {
		return options{}, err
	}

	if len(f.Args()) != 1 {
		return options{}, fmt.Errorf("Expected 1 argument, got %d.", len(f.Args()))
	}

	if *jsonOutput && *outputFormat != "" {
		return options{}, errors.New("Cannot use output-format and json flags together")
	}

	if *envelopeType != "" && *counterName != "" {
		return options{}, errors.New("--counter-name cannot be used with --envelope-type")
	}

	if *envelopeType != "" && *gaugeName != "" {
		return options{}, errors.New("--gauge-name cannot be used with --envelope-type")
	}

	if *gaugeName != "" && *counterName != "" {
		return options{}, errors.New("--counter-name cannot be used with --gauge-name")
	}

	if *envelopeType != "" && *envelopeClass != "" {
		return options{}, errors.New("--envelope-type cannot be used with --type")
	}

	if *envelopeClass != "" {
		*envelopeType = "ANY"
	}

	var outputTemplate *template.Template
	if *outputFormat != "" {
		outputTemplate, err = parseOutputFormat(*outputFormat)
		if err != nil {
			log.Fatalf("%s", err)
		}
	}

	o := options{
		startTime:      time.Unix(0, *start),
		endTime:        time.Unix(0, *end),
		envelopeType:   translateEnvelopeType(*envelopeType),
		lines:          int(*lines),
		guid:           getAppGUID(f.Args()[0], cli, log),
		providedName:   f.Args()[0],
		follow:         *follow,
		outputTemplate: outputTemplate,
		jsonOutput:     *jsonOutput,
		gaugeName:      *gaugeName,
		counterName:    *counterName,
		envelopeClass:  toEnvelopeClass(*envelopeClass),
	}

	return o, o.validate()
}

func toEnvelopeClass(class string) envelopeClass {
	switch strings.ToUpper(class) {
	case "METRICS":
		return envelopeClassMetric
	case "LOGS":
		return envelopeClassLog
	case "ANY":
		return envelopeClassAny
	default:
		return envelopeClassAny
	}
}

func formatterKindFromOptions(o options) formatterKind {
	if o.jsonOutput {
		return jsonFormat
	}

	if o.outputTemplate != nil {
		return templateFormat
	}

	return prettyFormat
}

func nameFilter(e *loggregator_v2.Envelope, o options) bool {
	if o.gaugeName != "" {
		for name := range e.GetGauge().GetMetrics() {
			if name == o.gaugeName {
				return true
			}
		}

		return false
	}

	if o.counterName != "" {
		return e.GetCounter().GetName() == o.counterName
	}

	return true
}

func typeFilter(e *loggregator_v2.Envelope, o options) bool {
	if o.envelopeClass == envelopeClassAny {
		return true
	}

	switch e.Message.(type) {
	case *loggregator_v2.Envelope_Counter, *loggregator_v2.Envelope_Gauge, *loggregator_v2.Envelope_Timer:
		return o.envelopeClass == envelopeClassMetric
	case *loggregator_v2.Envelope_Log, *loggregator_v2.Envelope_Event:
		return o.envelopeClass == envelopeClassLog
	}

	return false
}

func (o options) validate() error {
	if o.startTime.After(o.endTime) && o.endTime != time.Unix(0, 0) {
		return errors.New("Invalid date/time range. Ensure your start time is prior or equal the end time.")
	}

	if o.lines > 1000 || o.lines < 1 {
		return errors.New("Lines must be 1 to 1000.")
	}

	return nil
}

func parseOutputFormat(f string) (*template.Template, error) {
	templ := template.New("OutputFormat")
	_, err := templ.Parse(f)
	if err != nil {
		return nil, err
	}
	return templ, nil
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

func getAppGUID(appName string, cli plugin.CliConnection, log Logger) string {
	r, err := cli.CliCommandWithoutTerminalOutput(
		"app",
		appName,
		"--guid",
	)
	if err != nil {
		return ""
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
