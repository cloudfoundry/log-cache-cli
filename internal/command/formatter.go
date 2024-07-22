package command

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"code.cloudfoundry.org/go-loggregator/v10/rpc/loggregator_v2"
	"google.golang.org/protobuf/encoding/protojson"
)

const timeFormat = "2006-01-02T15:04:05.00-0700"

const (
	prettyFormat formatterKind = iota
	jsonFormat
	templateFormat
)

const (
	appHeaderFormat     = "Retrieving logs for app %s in org %s / space %s as %s..."
	serviceHeaderFormat = "Retrieving logs for service %s in org %s / space %s as %s..."
	sourceHeaderFormat  = "Retrieving logs for source %s as %s..."
)

type formatterKind int

type formatter interface {
	appHeader(app, org, space, user string) (string, bool)
	serviceHeader(service, org, space, user string) (string, bool)
	sourceHeader(sourceID, _, _, user string) (string, bool)
	formatEnvelope(e *loggregator_v2.Envelope) (string, bool)
	flush() (string, bool)
}

func newFormatter(sourceID string, following bool, kind formatterKind, log Logger, t *template.Template, newLineReplacer rune) formatter {
	bf := baseFormatter{
		log: log,
	}

	switch kind {
	case prettyFormat:
		return prettyFormatter{
			baseFormatter: bf,
			sourceID:      sourceID,
			newLine:       newLineReplacer,
		}
	case jsonFormat:
		return &jsonFormatter{
			following:     following,
			baseFormatter: bf,
		}
	case templateFormat:
		return templateFormatter{
			baseFormatter:  bf,
			outputTemplate: t,
		}
	default:
		log.Fatalf("Unknown formatter kind")
		return baseFormatter{}
	}
}

type baseFormatter struct {
	log Logger
}

func (f baseFormatter) flush() (string, bool) {
	return "", false
}

func (f baseFormatter) appHeader(_, _, _, _ string) (string, bool) {
	return "", false
}

func (f baseFormatter) serviceHeader(_, _, _, _ string) (string, bool) {
	return "", false
}

func (f baseFormatter) sourceHeader(_, _, _, _ string) (string, bool) {
	return "", false
}

func (f baseFormatter) formatEnvelope(e *loggregator_v2.Envelope) (string, bool) {
	return "", false
}

type prettyFormatter struct {
	baseFormatter
	sourceID string
	newLine  rune
}

func (f prettyFormatter) appHeader(app, org, space, user string) (string, bool) {
	return fmt.Sprintf(
		appHeaderFormat,
		app,
		org,
		space,
		user,
	), true
}

func (f prettyFormatter) serviceHeader(service, org, space, user string) (string, bool) {
	return fmt.Sprintf(
		serviceHeaderFormat,
		service,
		org,
		space,
		user,
	), true
}

func (f prettyFormatter) sourceHeader(sourceID, _, _, user string) (string, bool) {
	return fmt.Sprintf(
		sourceHeaderFormat,
		sourceID,
		user,
	), true
}

func (f prettyFormatter) formatEnvelope(e *loggregator_v2.Envelope) (string, bool) {
	return envelopeWrapper{sourceID: f.sourceID, Envelope: e, newLine: f.newLine}.String(), true
}

type jsonFormatter struct {
	baseFormatter

	following bool
	es        []string
}

func (f *jsonFormatter) formatEnvelope(e *loggregator_v2.Envelope) (string, bool) {
	output, err := jsonEnvelope(e)
	if err != nil {
		log.Printf("failed to marshal envelope: %s", err)
		return "", false
	}

	if !f.following {
		f.es = append(f.es, string(output))
		return "", false
	}

	return string(output), true
}

func (f *jsonFormatter) flush() (string, bool) {
	if f.following {
		return "", false
	}

	output := fmt.Sprintf(`{"batch":[%s]}`, strings.Join(f.es, ","))

	return string(output), true
}

type LogEnvelopeForMarshalling struct {
	Timestamp      string            `json:"timestamp"`
	SourceId       string            `json:"source_id"`
	InstanceID     string            `json:"instance_id"`
	DeprecatedTags map[string]string `json:"deprecated_tags,omitempty"`
	Tags           map[string]string `json:"tags"`
	Log            Log               `json:"log"`
}

type Log struct {
	Payload string `json:"payload"`
}

func jsonEnvelope(e *loggregator_v2.Envelope) ([]byte, error) {
	switch e.Message.(type) {
	case *loggregator_v2.Envelope_Log:
		depTags := map[string]string{}
		for tag, value := range e.GetDeprecatedTags() {
			depTags[tag] = value.String()
		}
		m := LogEnvelopeForMarshalling{
			Timestamp:      strconv.FormatInt(e.GetTimestamp(), 10),
			SourceId:       e.GetSourceId(),
			InstanceID:     e.GetInstanceId(),
			Tags:           e.GetTags(),
			DeprecatedTags: depTags,
			Log:            Log{Payload: string(e.GetLog().GetPayload())},
		}

		return json.Marshal(m)
	default:
		return protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(e)
	}
}

type templateFormatter struct {
	baseFormatter

	outputTemplate *template.Template
}

func (f templateFormatter) appHeader(app, org, space, user string) (string, bool) {
	return fmt.Sprintf(
		appHeaderFormat,
		app,
		org,
		space,
		user,
	), true
}

func (f templateFormatter) serviceHeader(service, org, space, user string) (string, bool) {
	return fmt.Sprintf(
		serviceHeaderFormat,
		service,
		org,
		space,
		user,
	), true
}

func (f templateFormatter) sourceHeader(sourceID, _, _, user string) (string, bool) {
	return fmt.Sprintf(
		sourceHeaderFormat,
		sourceID,
		user,
	), true
}

func (f templateFormatter) formatEnvelope(e *loggregator_v2.Envelope) (string, bool) {
	b := bytes.Buffer{}
	if err := f.outputTemplate.Execute(&b, e); err != nil {
		f.log.Fatalf("Output template parsed, but failed to execute: %s", err)
	}

	if b.Len() == 0 {
		return "", false
	}

	return b.String(), true
}

type envelopeWrapper struct {
	*loggregator_v2.Envelope
	sourceID string
	newLine  rune
}

func (e envelopeWrapper) String() string {
	ts := time.Unix(0, e.Timestamp)

	switch e.Message.(type) {
	case *loggregator_v2.Envelope_Log:
		payload := string(e.GetLog().GetPayload())
		sanitizer := func(r rune) rune {
			if r == e.newLine {
				return '\n'
			}
			return r
		}
		if e.newLine != 0 {
			payload = strings.Map(sanitizer, payload)
		}

		return fmt.Sprintf("%s%s %s",
			e.header(ts),
			e.GetLog().GetType(),
			payload,
		)
	case *loggregator_v2.Envelope_Counter:
		return fmt.Sprintf("%sCOUNTER %s:%d",
			e.header(ts),
			e.GetCounter().GetName(),
			e.GetCounter().GetTotal(),
		)
	case *loggregator_v2.Envelope_Gauge:
		var values []string
		for k, v := range e.GetGauge().GetMetrics() {
			values = append(values, fmt.Sprintf("%s:%f %s", k, v.Value, v.Unit))
		}

		sort.Strings(values)

		return fmt.Sprintf("%sGAUGE %s",
			e.header(ts),
			strings.Join(values, " "),
		)
	case *loggregator_v2.Envelope_Timer:
		timer := e.GetTimer()
		return fmt.Sprintf("%sTIMER %s %f ms",
			e.header(ts),
			timer.GetName(),
			float64(timer.GetStop()-timer.GetStart())/1000000.0,
		)
	case *loggregator_v2.Envelope_Event:
		return fmt.Sprintf("%sEVENT %s:%s",
			e.header(ts),
			e.GetEvent().GetTitle(),
			e.GetEvent().GetBody(),
		)
	default:
		return e.Envelope.String()
	}
}

func (e envelopeWrapper) header(ts time.Time) string {
	if e.InstanceId == "" {
		return fmt.Sprintf("   %s [%s] ",
			ts.Format(timeFormat),
			e.source(),
		)
	} else {
		return fmt.Sprintf("   %s [%s/%s] ",
			ts.Format(timeFormat),
			e.source(),
			e.GetInstanceId(),
		)
	}
}

func (e envelopeWrapper) source() string {
	switch e.Message.(type) {
	case *loggregator_v2.Envelope_Log:
		return e.sourceType()
	default:
		return e.sourceID
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
