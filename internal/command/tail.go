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

	formatter := NewFormatter(formatterKind(o), log, o.outputTemplate)

	guid := o.guid
	headerPrinter := formatter.AppHeader
	if guid == "" {
		// fall back to provided name
		guid = o.providedName
		headerPrinter = formatter.SourceHeader
	}

	header, ok := headerPrinter(o.providedName, org.Name, space.Name, user)
	if ok {
		lw.Write(header)
		lw.Write("")
	}

	if o.follow {
		logcache.Walk(
			ctx,
			guid,
			logcache.Visitor(func(envelopes []*loggregator_v2.Envelope) bool {
				for _, e := range envelopes {
					if output, ok := formatter.FormatEnvelope(e); ok {
						lw.Write(output)
					}
				}
				return true
			}),
			client.Read,
			logcache.WithWalkStartTime(time.Now()),
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
		if output, ok := formatter.FormatEnvelope(envelopes[i]); ok {
			lw.Write(output)
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

type options struct {
	startTime    time.Time
	endTime      time.Time
	envelopeType logcacherpc.EnvelopeTypes
	lines        int
	follow       bool

	guid           string
	providedName   string
	outputTemplate *template.Template
	jsonOutput     bool
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
	}

	return o, o.validate()
}

func formatterKind(o options) FormatterKind {
	if o.jsonOutput {
		return JSONFormat
	}

	if o.outputTemplate != nil {
		return TemplateFormat
	}

	return PrettyFormat
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
