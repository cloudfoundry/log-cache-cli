package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	logcache "code.cloudfoundry.org/go-log-cache"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/spf13/cobra"
)

type Tail struct {
	*cobra.Command

	conf      Config
	noHeaders bool
	timeout   time.Duration

	follow     bool
	jsonOutput bool
}

type TailOption func(*Tail)

func WithTailNoHeaders() TailOption {
	return func(t *Tail) {
		t.noHeaders = true
	}
}

func WithTailTimeout(d time.Duration) TailOption {
	return func(t *Tail) {
		t.timeout = d
	}
}

func NewTail(conf Config, opts ...TailOption) *cobra.Command {
	t := &Tail{
		conf:    conf,
		timeout: 5000 * time.Hour,
	}
	t.Command = t.command()

	for _, o := range opts {
		o(t)
	}

	return t.Command
}

func (t *Tail) command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tail <namespace/type/resource|resource>",
		Short: "Output logs and metrics for a given resource",
		RunE:  t.runE,
		Args:  cobra.ExactArgs(1),
	}
	cmd.Flags().BoolVarP(
		&t.follow,
		"follow",
		"f",
		false,
		"Output appended to stdout as logs are egressed.",
	)
	cmd.Flags().BoolVar(
		&t.jsonOutput,
		"json",
		false,
		"Output envelopes in JSON format.",
	)
	return cmd
}

func (t *Tail) runE(_ *cobra.Command, args []string) error {
	sourceID := args[0]

	if !t.noHeaders && !t.jsonOutput {
		header := fmt.Sprintf("Retrieving logs for %s...\n\n", sourceID)
		fmt.Fprintf(t.OutOrStdout(), header)
	}

	client := logcache.NewClient(t.conf.Addr)
	ctx, cancel := context.WithTimeout(context.Background(), t.timeout)
	defer cancel()
	if t.follow {
		return t.walk(ctx, client, sourceID, t.printer())
	}
	return t.read(ctx, client, sourceID, t.printer())
}

func (t *Tail) printer() printer {
	w := t.OutOrStdout()
	switch {
	case t.jsonOutput && t.follow:
		p := &streamingJSONPrinter{}
		p.w = w
		return p
	case t.jsonOutput:
		p := &jsonPrinter{}
		p.w = w
		return p
	default:
		p := &defaultPrinter{}
		p.w = w
		return p
	}
}

type printer interface {
	writeEnvs([]*loggregator_v2.Envelope) error
}

func (t *Tail) walk(
	ctx context.Context,
	client *logcache.Client,
	sourceID string,
	printer printer,
) error {
	var err error
	logcache.Walk(
		ctx,
		sourceID,
		logcache.Visitor(func(envs []*loggregator_v2.Envelope) bool {
			err = printer.writeEnvs(envs)
			if err != nil {
				return false
			}
			return true
		}),
		client.Read,
		logcache.WithWalkStartTime(time.Unix(0, 0)),
		logcache.WithWalkBackoff(logcache.NewAlwaysRetryBackoff(250*time.Millisecond)),
	)
	return err
}

func (t *Tail) read(
	ctx context.Context,
	client *logcache.Client,
	sourceID string,
	printer printer,
) error {
	envs, err := client.Read(ctx, sourceID, time.Unix(0, 0))
	if err != nil {
		return err
	}

	return printer.writeEnvs(envs)
}

type basePrinter struct {
	w io.Writer
}

func (p *basePrinter) sort(envs []*loggregator_v2.Envelope) error {
	sort.Slice(envs, func(i, j int) bool {
		return envs[i].GetTimestamp() < envs[j].GetTimestamp()
	})
	return nil
}

type jsonPrinter struct {
	basePrinter
}

func (p *jsonPrinter) writeEnvs(envs []*loggregator_v2.Envelope) error {
	p.sort(envs)
	m := &jsonpb.Marshaler{}
	fmt.Fprint(p.w, "[\n    ")
	var err error
	for i, e := range envs {
		err = m.Marshal(p.w, e)
		if err != nil {
			return err
		}
		if i != len(envs)-1 {
			_, err = fmt.Fprint(p.w, ",\n    ")
			if err != nil {
				return err
			}
		} else {
			_, err = fmt.Fprint(p.w, "\n")
			if err != nil {
				return err
			}
		}
	}
	_, err = fmt.Fprint(p.w, "]\n")
	return err
}

// streamingJSONPrinter prints envelopes in a newline delimited format
// See: https://en.wikipedia.org/wiki/JSON_streaming#Line-delimited_JSON
type streamingJSONPrinter struct {
	basePrinter
}

func (p *streamingJSONPrinter) writeEnvs(envs []*loggregator_v2.Envelope) error {
	p.sort(envs)
	m := &jsonpb.Marshaler{}
	for _, e := range envs {
		err := m.Marshal(p.w, e)
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(p.w, "\n")
		if err != nil {
			return err
		}
	}
	return nil
}

type defaultPrinter struct {
	basePrinter
}

func (p *defaultPrinter) writeEnvs(envs []*loggregator_v2.Envelope) error {
	p.sort(envs)
	for _, e := range envs {
		err := p.writeEnv(e)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *defaultPrinter) writeEnv(e *loggregator_v2.Envelope) error {
	const timeFormat = "2006-01-02T15:04:05.00-0700"
	ts := time.Unix(0, e.Timestamp)
	var err error
	switch e.Message.(type) {
	case *loggregator_v2.Envelope_Log:
		_, err = fmt.Fprintf(
			p.w,
			format(e),
			ts.Format(timeFormat),
			e.GetSourceId(),
			e.GetInstanceId(),
			e.GetLog().GetType(),
			bytes.TrimRight(e.GetLog().GetPayload(), "\n"),
		)
	case *loggregator_v2.Envelope_Counter:
		_, err = fmt.Fprintf(
			p.w,
			format(e),
			ts.Format(timeFormat),
			e.GetSourceId(),
			e.GetInstanceId(),
			e.GetCounter().GetName(),
			e.GetCounter().GetTotal(),
		)
	case *loggregator_v2.Envelope_Gauge:
		var values []string
		for k, v := range e.GetGauge().GetMetrics() {
			values = append(values, fmt.Sprintf("%s:%f %s", k, v.Value, v.Unit))
		}

		sort.Sort(sort.StringSlice(values))

		_, err = fmt.Fprintf(
			p.w,
			format(e),
			ts.Format(timeFormat),
			e.GetSourceId(),
			e.GetInstanceId(),
			strings.Join(values, " "),
		)
	case *loggregator_v2.Envelope_Timer:
		_, err = fmt.Fprintf(
			p.w,
			format(e),
			ts.Format(timeFormat),
			e.GetSourceId(),
			e.GetInstanceId(),
			time.Unix(0, e.GetTimer().GetStop()).Sub(time.Unix(0, e.GetTimer().GetStart())),
		)
	case *loggregator_v2.Envelope_Event:
		_, err = fmt.Fprintf(
			p.w,
			format(e),
			ts.Format(timeFormat),
			e.GetSourceId(),
			e.GetInstanceId(),
			e.GetEvent().GetTitle(),
			e.GetEvent().GetBody(),
		)
	default:
		ec, ok := proto.Clone(e).(*loggregator_v2.Envelope)
		if ok {
			// Zero out fields that are already displayed in the message
			// format.
			ec.Timestamp = 0
			ec.SourceId = ""
			ec.InstanceId = ""
		} else {
			// We are not going to cover this. The type assertion should
			// always be ok. If it is not it isn't the end of the world. Just
			// fall back to displaying the whole envelope.
			ec = e
		}
		_, err = fmt.Fprintf(
			p.w,
			format(e),
			ts.Format(timeFormat),
			e.GetSourceId(),
			e.GetInstanceId(),
			strings.TrimSpace(ec.String()),
		)
	}

	return err
}

func format(e *loggregator_v2.Envelope) string {
	switch e.Message.(type) {
	case *loggregator_v2.Envelope_Log:
		if e.GetInstanceId() == "" {
			return "%s [%s] LOG/%[4]s %s\n"
		} else {
			return "%s [%s/%s] LOG/%s %s\n"
		}
	case *loggregator_v2.Envelope_Counter:
		if e.GetInstanceId() == "" {
			return "%s [%s] COUNTER %[4]s:%d\n"
		} else {
			return "%s [%s/%s] COUNTER %s:%d\n"
		}
	case *loggregator_v2.Envelope_Gauge:
		if e.GetInstanceId() == "" {
			return "%s [%s] GAUGE %[4]s\n"
		} else {
			return "%s [%s/%s] GAUGE %s\n"
		}
	case *loggregator_v2.Envelope_Timer:
		if e.GetInstanceId() == "" {
			return "%s [%s] TIMER %[4]s\n"
		} else {
			return "%s [%s/%s] TIMER %s\n"
		}
	case *loggregator_v2.Envelope_Event:
		if e.GetInstanceId() == "" {
			return "%s [%s] EVENT %[4]s:%s\n"
		} else {
			return "%s [%s/%s] EVENT %s:%s\n"
		}
	default:
		if e.GetInstanceId() == "" {
			return "%s [%s] UNKNOWN %[4]s\n"
		} else {
			return "%s [%s/%s] UNKNOWN %s\n"
		}
	}
}
