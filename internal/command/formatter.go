package command

import (
	"bytes"
	"fmt"
	"log"
	"sort"
	"strings"
	"text/template"
	"time"

	"code.cloudfoundry.org/go-loggregator/v8/rpc/loggregator_v2"
	"github.com/golang/protobuf/jsonpb"
)

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

func newFormatter(sourceID string, following bool, kind formatterKind, t *template.Template, newLineReplacer rune) formatter {
	bf := baseFormatter{}

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

type baseFormatter struct{}

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
	es        []*loggregator_v2.Envelope
	marshaler jsonpb.Marshaler
}

func (f *jsonFormatter) formatEnvelope(e *loggregator_v2.Envelope) (string, bool) {
	if f.following {
		output, err := f.marshaler.MarshalToString(e)
		if err != nil {
			log.Printf("failed to marshal envelope: %s", err)
			return "", false
		}

		return string(output), true
	}

	f.es = append(f.es, e)

	return "", false
}

func (f *jsonFormatter) flush() (string, bool) {
	if f.following {
		return "", false
	}

	output, err := f.marshaler.MarshalToString(&loggregator_v2.EnvelopeBatch{
		Batch: f.es,
	})
	if err != nil {
		log.Printf("failed to marshal envelopes: %s", err)
		return "", false
	}

	return string(output), true
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
		log.Panicf("Output template parsed, but failed to execute: %s", err)
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
