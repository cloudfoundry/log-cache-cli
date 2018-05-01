package command

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	logcache "code.cloudfoundry.org/go-log-cache"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"github.com/gogo/protobuf/proto"
	"github.com/spf13/cobra"
)

type Tail struct {
	*cobra.Command

	conf      Config
	noHeaders bool
	timeout   time.Duration

	follow bool
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
		Use:   "tail <source-id>",
		Short: "Output logs and metrics for a given source-id",
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
	return cmd
}

func (t *Tail) runE(_ *cobra.Command, args []string) error {
	sourceID := args[0]

	if !t.noHeaders {
		header := fmt.Sprintf("Retrieving logs for %s...\n\n", sourceID)
		fmt.Fprintf(t.OutOrStdout(), header)
	}

	client := logcache.NewClient(t.conf.Addr)
	ctx, cancel := context.WithTimeout(context.Background(), t.timeout)
	defer cancel()
	if t.follow {
		return t.walk(ctx, client, sourceID)
	}
	return t.read(ctx, client, sourceID)
}

func (t *Tail) walk(
	ctx context.Context,
	client *logcache.Client,
	sourceID string,
) error {
	var err error
	logcache.Walk(
		ctx,
		sourceID,
		logcache.Visitor(func(envs []*loggregator_v2.Envelope) bool {
			err = t.writeEnvs(envs)
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
) error {
	envs, err := client.Read(ctx, sourceID, time.Unix(0, 0))
	if err != nil {
		return err
	}

	return t.writeEnvs(envs)
}

func (t *Tail) writeEnvs(envs []*loggregator_v2.Envelope) error {
	sort.Slice(envs, func(i, j int) bool {
		return envs[i].GetTimestamp() < envs[j].GetTimestamp()
	})

	for _, e := range envs {
		err := t.writeEnv(e)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *Tail) writeEnv(e *loggregator_v2.Envelope) error {
	const timeFormat = "2006-01-02T15:04:05.00-0700"
	ts := time.Unix(0, e.Timestamp)
	var err error
	switch e.Message.(type) {
	case *loggregator_v2.Envelope_Log:
		_, err = fmt.Fprintf(
			t.OutOrStdout(),
			"%s [%s/%s] LOG/%s %s\n",
			ts.Format(timeFormat),
			e.GetSourceId(),
			e.GetInstanceId(),
			e.GetLog().GetType(),
			bytes.TrimRight(e.GetLog().GetPayload(), "\n"),
		)
	case *loggregator_v2.Envelope_Counter:
		_, err = fmt.Fprintf(
			t.OutOrStdout(),
			"%s [%s/%s] COUNTER %s:%d\n",
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
			t.OutOrStdout(),
			"%s [%s/%s] GAUGE %s\n",
			ts.Format(timeFormat),
			e.GetSourceId(),
			e.GetInstanceId(),
			strings.Join(values, " "),
		)
	case *loggregator_v2.Envelope_Timer:
		_, err = fmt.Fprintf(
			t.OutOrStdout(),
			"%s [%s/%s] TIMER %s\n",
			ts.Format(timeFormat),
			e.GetSourceId(),
			e.GetInstanceId(),
			time.Unix(0, e.GetTimer().GetStop()).Sub(time.Unix(0, e.GetTimer().GetStart())),
		)
	case *loggregator_v2.Envelope_Event:
		_, err = fmt.Fprintf(
			t.OutOrStdout(),
			"%s [%s/%s] EVENT %s:%s\n",
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
			t.OutOrStdout(),
			"%s [%s/%s] UNKNOWN %s\n",
			ts.Format(timeFormat),
			e.GetSourceId(),
			e.GetInstanceId(),
			strings.TrimSpace(ec.String()),
		)
	}

	return err
}
