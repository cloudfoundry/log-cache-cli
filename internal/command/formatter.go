package command

import (
	"bytes"
	"fmt"
	"log"
	"sort"
	"strings"
	"text/template"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"github.com/golang/protobuf/jsonpb"
)

const (
	PrettyFormat FormatterKind = iota
	JSONFormat
	TemplateFormat
)

const (
	appHeaderFormat    = "Retrieving logs for app %s in org %s / space %s as %s..."
	sourceHeaderFormat = "Retrieving logs for %s as %s..."
)

type FormatterKind int

type Formatter interface {
	AppHeader(app, org, space, user string) (string, bool)
	SourceHeader(sourceID, _, _, user string) (string, bool)
	FormatEnvelope(e *loggregator_v2.Envelope) (string, bool)
}

func NewFormatter(kind FormatterKind, log Logger, t *template.Template) Formatter {
	baseFormatter := BaseFormatter{
		log: log,
	}

	switch kind {
	case PrettyFormat:
		return PrettyFormatter{
			BaseFormatter: baseFormatter,
		}
	case JSONFormat:
		return JSONFormatter{
			BaseFormatter: baseFormatter,
		}
	case TemplateFormat:
		return TemplateFormatter{
			BaseFormatter:  baseFormatter,
			outputTemplate: t,
		}
	default:
		log.Fatalf("Unknown formatter kind")
		return BaseFormatter{}
	}
}

type BaseFormatter struct {
	log Logger
}

func (f BaseFormatter) AppHeader(_, _, _, _ string) (string, bool) {
	return "", false
}

func (f BaseFormatter) SourceHeader(_, _, _, _ string) (string, bool) {
	return "", false
}

func (f BaseFormatter) FormatEnvelope(e *loggregator_v2.Envelope) (string, bool) {
	return "", false
}

type PrettyFormatter struct {
	BaseFormatter
}

func (f PrettyFormatter) AppHeader(app, org, space, user string) (string, bool) {
	return fmt.Sprintf(
		appHeaderFormat,
		app,
		org,
		space,
		user,
	), true
}

func (f PrettyFormatter) SourceHeader(sourceID, _, _, user string) (string, bool) {
	return fmt.Sprintf(
		sourceHeaderFormat,
		sourceID,
		user,
	), true
}

func (f PrettyFormatter) FormatEnvelope(e *loggregator_v2.Envelope) (string, bool) {
	return fmt.Sprintf("%s", envelopeWrapper{e}), true
}

type JSONFormatter struct {
	BaseFormatter

	marshaler jsonpb.Marshaler
}

func (f JSONFormatter) FormatEnvelope(e *loggregator_v2.Envelope) (string, bool) {
	output, err := f.marshaler.MarshalToString(e)
	if err != nil {
		log.Printf("failed to marshal envelope: %s", err)
		return "", false
	}

	return string(output), true
}

type TemplateFormatter struct {
	BaseFormatter

	outputTemplate *template.Template
}

func (f TemplateFormatter) AppHeader(app, org, space, user string) (string, bool) {
	return fmt.Sprintf(
		appHeaderFormat,
		app,
		org,
		space,
		user,
	), true
}

func (f TemplateFormatter) SourceHeader(sourceID, _, _, user string) (string, bool) {
	return fmt.Sprintf(
		sourceHeaderFormat,
		sourceID,
		user,
	), true
}

func (f TemplateFormatter) FormatEnvelope(e *loggregator_v2.Envelope) (string, bool) {
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
