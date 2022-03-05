package command

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"text/template"
	"time"
	"unicode/utf8"

	"code.cloudfoundry.org/cli/plugin"
	logcache "code.cloudfoundry.org/go-log-cache"
	logcache_v1 "code.cloudfoundry.org/go-log-cache/rpc/logcache_v1"
	"code.cloudfoundry.org/go-loggregator/v8/rpc/loggregator_v2"
	"github.com/blang/semver"
	flags "github.com/jessevdk/go-flags"
)

const (
	timeFormat = "2006-01-02T15:04:05.00-0700"
)

type TailOption func(*tailOptions)

func WithTailNoHeaders() TailOption {
	return func(o *tailOptions) {
		o.noHeaders = true
	}
}

// Tail will fetch the logs for a given application guid and write them to
// stdout.
func Tail(
	ctx context.Context,
	cli plugin.CliConnection,
	args []string,
	c HTTPClient,
	w io.Writer,
	opts ...TailOption,
) {
	o, err := newTailOptions(cli, args)
	if err != nil {
		log.Panicf("%s", err)
	}

	for _, opt := range opts {
		opt(&o)
	}

	sourceID := o.guid
	formatter := newFormatter(o.providedName, o.follow, formatterKindFromOptions(o), o.outputTemplate, o.newLineReplacer)
	lw := lineWriter{w: w}

	defer func() {
		if value, ok := formatter.flush(); ok {
			lw.Write(value)
		}
	}()

	logCacheAddr := os.Getenv("LOG_CACHE_ADDR")
	if logCacheAddr == "" {
		hasAPI, err := cli.HasAPIEndpoint()
		if err != nil {
			log.Panicf("%s", err)
		}

		if !hasAPI {
			log.Panicf("No API endpoint targeted.")
		}

		tokenURL, err := cli.ApiEndpoint()
		if err != nil {
			log.Panicf("%s", err)
		}

		user, err := cli.Username()
		if err != nil {
			log.Panicf("%s", err)
		}

		org, err := cli.GetCurrentOrg()
		if err != nil {
			log.Panicf("%s", err)
		}

		space, err := cli.GetCurrentSpace()
		if err != nil {
			log.Panicf("%s", err)
		}

		logCacheAddr = strings.Replace(tokenURL, "api", "log-cache", 1)

		headerPrinter := formatter.appHeader
		if o.isService {
			headerPrinter = formatter.serviceHeader
		}
		if sourceID == "" {
			// not an app or service, use generic header
			headerPrinter = formatter.sourceHeader
		}

		if !o.noHeaders {
			header, ok := headerPrinter(o.providedName, org.Name, space.Name, user)
			if ok {
				lw.Write(header)
				lw.Write("")
			}
		}
	}

	filterAndFormat := func(e *loggregator_v2.Envelope) (string, bool) {
		if !typeFilter(e, o) {
			return "", false
		}

		return formatter.formatEnvelope(e)
	}

	tokenClient := &tokenHTTPClient{
		c:         c,
		tokenFunc: func() string { return "" },
	}

	if strings.ToLower(os.Getenv("LOG_CACHE_SKIP_AUTH")) != "true" {
		tokenClient.tokenFunc = func() string {
			token, err := cli.AccessToken()
			if err != nil {
				log.Panicf("Unable to get Access Token: %s", err)
			}
			return token
		}
	}

	client := logcache.NewClient(logCacheAddr, logcache.WithHTTPClient(tokenClient))

	checkFeatureVersioning(client, ctx, o.nameFilter)

	if sourceID == "" {
		// fall back to provided name
		sourceID = o.providedName
	}

	walkStartTime := time.Now().Add(-5 * time.Second).UnixNano()
	if o.lines > 0 {
		envelopes, err := client.Read(
			context.Background(),
			sourceID,
			o.startTime,
			logcache.WithEndTime(o.endTime),
			logcache.WithEnvelopeTypes(o.envelopeType),
			logcache.WithLimit(o.lines),
			logcache.WithDescending(),
			logcache.WithNameFilter(o.nameFilter),
		)

		if err != nil && !o.follow {
			log.Panicf("%s", err)
		}

		// we get envelopes in descending order but want to print them ascending
		for i := len(envelopes) - 1; i >= 0; i-- {
			walkStartTime = envelopes[i].Timestamp + 1
			if formatted, ok := filterAndFormat(envelopes[i]); ok {
				lw.Write(formatted)
			}
		}
	}

	if o.follow {
		logcache.Walk(
			ctx,
			sourceID,
			logcache.Visitor(func(envelopes []*loggregator_v2.Envelope) bool {
				for _, e := range envelopes {
					if formatted, ok := filterAndFormat(e); ok {
						lw.Write(formatted)
					}
				}
				return true
			}),
			client.Read,
			logcache.WithWalkStartTime(time.Unix(0, walkStartTime)),
			logcache.WithWalkEnvelopeTypes(o.envelopeType),
			logcache.WithWalkBackoff(logcache.NewAlwaysRetryBackoff(250*time.Millisecond)),
			logcache.WithWalkNameFilter(o.nameFilter),
		)
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

type tailOptions struct {
	startTime     time.Time
	endTime       time.Time
	envelopeType  logcache_v1.EnvelopeType
	envelopeClass envelopeClass
	lines         int
	follow        bool

	guid                 string
	isService            bool
	providedName         string
	outputTemplate       *template.Template
	jsonOutput           bool
	tokenRefreshInterval time.Duration

	nameFilter string

	noHeaders       bool
	newLineReplacer rune
}

type tailOptionFlags struct {
	StartTime     int64  `long:"start-time"`
	EndTime       int64  `long:"end-time"`
	EnvelopeType  string `long:"envelope-type" short:"t"`
	Lines         uint   `long:"lines" short:"n" default:"10"`
	Follow        bool   `long:"follow" short:"f"`
	OutputFormat  string `long:"output-format" short:"o"`
	JSONOutput    bool   `long:"json"`
	EnvelopeClass string `long:"envelope-class" short:"c"`
	NewLine       string `long:"new-line" optional:"true" optional-value:"\\u2028"`
	NameFilter    string `long:"name-filter"`
}

func newTailOptions(cli plugin.CliConnection, args []string) (tailOptions, error) {
	opts := tailOptionFlags{
		EndTime: time.Now().UnixNano(),
	}

	args, err := flags.ParseArgs(&opts, args)
	if err != nil {
		return tailOptions{}, err
	}

	if len(args) != 1 {
		return tailOptions{}, fmt.Errorf("Expected 1 argument, got %d.", len(args))
	}

	if opts.JSONOutput && opts.OutputFormat != "" {
		return tailOptions{}, errors.New("Cannot use output-format and json flags together")
	}

	if opts.EnvelopeType != "" && opts.EnvelopeClass != "" {
		return tailOptions{}, errors.New("--envelope-type cannot be used with --envelope-class")
	}

	if opts.EnvelopeClass != "" {
		opts.EnvelopeType = "ANY"
	}

	var outputTemplate *template.Template
	if opts.OutputFormat != "" {
		outputTemplate, err = parseOutputFormat(opts.OutputFormat)
		if err != nil {
			log.Panicf("%s", err)
		}
	}

	id, isService := getGUID(args[0], cli)
	o := tailOptions{
		startTime:            time.Unix(0, opts.StartTime),
		endTime:              time.Unix(0, opts.EndTime),
		envelopeType:         translateEnvelopeType(opts.EnvelopeType),
		lines:                int(opts.Lines),
		guid:                 id,
		isService:            isService,
		providedName:         args[0],
		follow:               opts.Follow,
		outputTemplate:       outputTemplate,
		jsonOutput:           opts.JSONOutput,
		tokenRefreshInterval: 5 * time.Minute,
		nameFilter:           opts.NameFilter,
		envelopeClass:        toEnvelopeClass(opts.EnvelopeClass),
	}

	if opts.NewLine != "" {
		o.newLineReplacer, err = parseNewLineArgument(opts.NewLine)
		if err != nil {
			log.Panicf("%s", err)
		}
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

func formatterKindFromOptions(o tailOptions) formatterKind {
	if o.jsonOutput {
		return jsonFormat
	}

	if o.outputTemplate != nil {
		return templateFormat
	}

	return prettyFormat
}

func typeFilter(e *loggregator_v2.Envelope, o tailOptions) bool {
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

func (o tailOptions) validate() error {
	if o.startTime.After(o.endTime) && o.endTime != time.Unix(0, 0) {
		return errors.New("Invalid date/time range. Ensure your start time is prior or equal the end time.")
	}

	if o.lines > 1000 || o.lines < 0 {
		return errors.New("Lines cannot be greater than 1000.")
	}

	_, err := regexp.Compile(o.nameFilter)
	if err != nil {
		return errors.New(fmt.Sprintf("Invalid name filter '%s'. Ensure your name-filter is a valid regex.", o.nameFilter))
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

func translateEnvelopeType(t string) logcache_v1.EnvelopeType {
	t = strings.ToUpper(t)

	switch t {
	case "ANY", "":
		return logcache_v1.EnvelopeType_ANY
	case "LOG":
		return logcache_v1.EnvelopeType_LOG
	case "COUNTER":
		return logcache_v1.EnvelopeType_COUNTER
	case "GAUGE":
		return logcache_v1.EnvelopeType_GAUGE
	case "TIMER":
		return logcache_v1.EnvelopeType_TIMER
	case "EVENT":
		return logcache_v1.EnvelopeType_EVENT
	default:
		log.Panicf("--envelope-type must be LOG, COUNTER, GAUGE, TIMER, EVENT or ANY")

		// Won't get here, but log.Panicf isn't obvious to the compiler that
		// execution will halt.
		return logcache_v1.EnvelopeType_ANY
	}
}

func getGUID(name string, cli plugin.CliConnection) (string, bool) {
	var id string
	if id = getAppGUID(name, cli); id == "" {
		return getServiceGUID(name, cli), true
	}
	return id, false
}

func getAppGUID(appName string, cli plugin.CliConnection) string {
	r, err := cli.CliCommandWithoutTerminalOutput(
		"app",
		appName,
		"--guid",
	)

	if err != nil {
		if err.Error() != "App "+appName+" not found" {
			log.Printf("%s", err)
		}
		return ""
	}

	return strings.Join(r, "")
}

func getServiceGUID(serviceName string, cli plugin.CliConnection) string {
	r, err := cli.CliCommandWithoutTerminalOutput(
		"service",
		serviceName,
		"--guid",
	)

	if err != nil {
		if err.Error() != "Service instance "+serviceName+" not found" {
			log.Printf("%s", err)
		}
		return ""
	}

	return strings.Join(r, "")
}

func parseNewLineArgument(s string) (rune, error) {
	if strings.TrimSpace(s) == "" {
		return '\u2028', nil
	}

	if utf8.RuneCountInString(s) == 1 {
		r, _ := utf8.DecodeRuneInString(s)
		return r, nil
	}

	s = strings.ToLower(s)
	if strings.HasPrefix(s, "\\u") {
		var r rune
		_, err := fmt.Sscanf(s, "\\u%x", &r)
		if err != nil {
			return 0, err
		}

		return r, nil
	}

	return 0, errors.New("--new-line argument must be single unicode character or in the format \\uXXXXX")
}

func checkFeatureVersioning(client *logcache.Client, ctx context.Context, nameFilter string) {
	version, _ := client.LogCacheVersion(ctx)

	if nameFilter != "" {
		nameFilterVersion, _ := semver.Parse("2.1.0")
		if version.LT(nameFilterVersion) {
			log.Panicf("Use of --name-filter requires minimum log-cache version 2.1.0")
		}
	}
}
