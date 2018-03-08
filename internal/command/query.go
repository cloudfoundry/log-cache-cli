package command

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/golang/protobuf/jsonpb"
	flags "github.com/jessevdk/go-flags"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/storage"

	"code.cloudfoundry.org/cli/plugin"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
)

type Tailer func(sourceID string, start, end time.Time) []string

// Query will fetch the logs for a given application guid and write them to
// stdout.
func Query(
	ctx context.Context,
	cli plugin.CliConnection,
	tailer Tailer,
	args []string,
	c HTTPClient,
	log Logger,
	w io.Writer,
) {
	o, err := newQueryOptions(cli, args, log)
	if err != nil {
		log.Fatalf("%s", err)
	}

	promql.LookbackDelta = 0

	interval := time.Second
	e := promql.NewEngine(&logCacheQueryable{
		log:      log,
		interval: interval,
		tailer:   tailer,
	}, nil)
	q, err := e.NewRangeQuery(o.query, o.startTime, o.endTime, interval)
	if err != nil {
		log.Fatalf("Invalid query: %s", err)
	}

	result := q.Exec(context.Background())
	w.Write([]byte(result.String() + "\n"))
}

type queryOptions struct {
	startTime time.Time
	endTime   time.Time
	query     string
}

type queryOptionFlags struct {
	StartTime int64 `long:"start-time"`
	EndTime   int64 `long:"end-time"`
}

func newQueryOptions(cli plugin.CliConnection, args []string, log Logger) (queryOptions, error) {
	opts := queryOptionFlags{
		StartTime: time.Now().Add(-5 * time.Minute).UnixNano(),
		EndTime:   time.Now().UnixNano(),
	}
	args, err := flags.ParseArgs(&opts, args)
	if err != nil {
		return queryOptions{}, err
	}

	if len(args) != 1 {
		return queryOptions{}, fmt.Errorf("Expected 1 argument, got %d.", len(args))
	}

	o := queryOptions{
		startTime: time.Unix(0, opts.StartTime).Truncate(time.Second),
		endTime:   time.Unix(0, opts.EndTime).Truncate(time.Second),
		query:     args[0],
	}

	return o, nil
}

type logCacheQueryable struct {
	log      Logger
	interval time.Duration
	tailer   Tailer
}

func (l *logCacheQueryable) Querier(ctx context.Context, mint int64, maxt int64) (storage.Querier, error) {
	return &LogCacheQuerier{
		log:      l.log,
		ctx:      ctx,
		start:    time.Unix(0, mint*int64(time.Millisecond)),
		end:      time.Unix(0, maxt*int64(time.Millisecond)),
		interval: l.interval,
		tailer:   l.tailer,
	}, nil
}

type LogCacheQuerier struct {
	log      Logger
	ctx      context.Context
	start    time.Time
	end      time.Time
	interval time.Duration
	tailer   Tailer
}

func (l *LogCacheQuerier) Select(ll ...*labels.Matcher) (storage.SeriesSet, error) {
	var (
		sourceID string
		metric   string
		ls       []labels.Label
	)
	for _, l := range ll {
		ls = append(ls, labels.Label{
			Name:  l.Name,
			Value: l.Value,
		})
		if l.Name == "__name__" {
			metric = l.Value
			continue
		}
		if l.Name == "source_id" {
			sourceID = l.Value
			continue
		}
	}

	if sourceID == "" {
		l.log.Fatalf("Metric '%s' does not have a 'source_id' label.", metric)
	}

	output := l.tailer(sourceID, l.start, l.end)

	var es []sample
	for _, line := range output {
		var e loggregator_v2.Envelope
		if err := jsonpb.Unmarshal(strings.NewReader(line), &e); err != nil {
			l.log.Fatalf("Failed to unmarshal JSON from LogCache: %s", err)
		}

		if e.GetCounter().GetName() != metric && e.GetGauge().GetMetrics()[metric] == nil {
			continue
		}

		if !l.hasLabels(e.GetTags(), ls) {
			continue
		}

		e.Timestamp = time.Unix(0, e.GetTimestamp()).Truncate(l.interval).UnixNano()

		var f float64
		switch e.Message.(type) {
		case *loggregator_v2.Envelope_Counter:
			f = float64(e.GetCounter().GetTotal())
		case *loggregator_v2.Envelope_Gauge:
			f = e.GetGauge().GetMetrics()[metric].GetValue()
		}

		es = append(es, sample{
			t: e.GetTimestamp() / int64(time.Millisecond),
			v: f,
		})
	}

	return fromEnvelopes(es, ls), nil
}

func (l *LogCacheQuerier) hasLabels(tags map[string]string, ls []labels.Label) bool {
	for _, l := range ls {
		if l.Name == "__name__" || l.Name == "source_id" {
			continue
		}

		if v, ok := tags[l.Name]; !ok || v != l.Value {
			return false
		}
	}

	return true
}

func (l *LogCacheQuerier) LabelValues(name string) ([]string, error) {
	panic("not implemented")
}

func (l *LogCacheQuerier) Close() error {
	return nil
}

func fromEnvelopes(es []sample, ls []labels.Label) storage.SeriesSet {
	return &concreteSeriesSet{
		series: []storage.Series{
			&concreteSeries{
				labels:  ls,
				samples: es,
			},
		},
	}
}

// concreteSeriesSet implements storage.SeriesSet.
type concreteSeriesSet struct {
	cur    int
	series []storage.Series
}

func (c *concreteSeriesSet) Next() bool {
	c.cur++
	return c.cur-1 < len(c.series)
}

func (c *concreteSeriesSet) At() storage.Series {
	return c.series[c.cur-1]
}

func (c *concreteSeriesSet) Err() error {
	return nil
}

// concreteSeries implementes storage.Series.
type concreteSeries struct {
	labels  labels.Labels
	samples []sample
}

type sample struct {
	t int64
	v float64
}

func (c *concreteSeries) Labels() labels.Labels {
	return labels.New(c.labels...)
}

func (c *concreteSeries) Iterator() storage.SeriesIterator {
	return newConcreteSeriersIterator(c)
}

// concreteSeriesIterator implements storage.SeriesIterator.
type concreteSeriesIterator struct {
	cur    int
	series *concreteSeries
}

func newConcreteSeriersIterator(series *concreteSeries) storage.SeriesIterator {
	return &concreteSeriesIterator{
		cur:    -1,
		series: series,
	}
}

// Seek implements storage.SeriesIterator.
func (c *concreteSeriesIterator) Seek(t int64) bool {
	c.cur = sort.Search(len(c.series.samples), func(n int) bool {
		return c.series.samples[n].t >= t
	})
	return c.cur < len(c.series.samples)
}

// At implements storage.SeriesIterator.
func (c *concreteSeriesIterator) At() (t int64, v float64) {
	s := c.series.samples[c.cur]

	return s.t, s.v
}

// Next implements storage.SeriesIterator.
func (c *concreteSeriesIterator) Next() bool {
	c.cur++
	return c.cur < len(c.series.samples)
}

// Err implements storage.SeriesIterator.
func (c *concreteSeriesIterator) Err() error {
	return nil
}
