package command_test

import (
	"context"
	"fmt"
	"strings"
	"time"

	"code.cloudfoundry.org/log-cache-cli/internal/command"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Query", func() {
	var (
		logger     *stubLogger
		writer     *stubWriter
		httpClient *stubHTTPClient
		cliConn    *spyCliConnection
		startTime  time.Time
		timeFormat string
	)

	BeforeEach(func() {
		startTime = time.Now().Truncate(time.Second).Add(-time.Minute)
		timeFormat = "2006-01-02T15:04:05.00-0700"
		logger = &stubLogger{}
		writer = &stubWriter{}

		httpClient = newStubHTTPClient()
		httpClient.responseBody = []string{responseBody(startTime)}

		cliConn = newSpyCliConnection()
		cliConn.cliCommandResult = [][]string{{"{}"}}
	})

	It("executes and returns result from query", func() {
		now := time.Now()
		f := func(sourceID string, start, end time.Time) []string {
			switch sourceID {
			case "source-x":
				return []string{
					fmt.Sprintf(`{"timestamp":"%d","sourceId":"source-x","counter":{"name":"x","total":"100"},"tags":{"deployment":"other"}}`, now.Add(-10*time.Second).UnixNano()),
					fmt.Sprintf(`{"timestamp":"%d","sourceId":"source-x","counter":{"name":"x","total":"1"},"tags":{"deployment":"cf","__name__":"other","source_id":"other"}}`, now.Add(-10*time.Second).UnixNano()),
					fmt.Sprintf(`{"timestamp":"%d","sourceId":"source-x","counter":{"name":"x","total":"100"},"tags":{"deployment":"cf"}}`, now.Add(-9*time.Second).UnixNano()),
					fmt.Sprintf(`{"timestamp":"%d","sourceId":"source-x","counter":{"name":"other","total":"2"}}`, now.Add(-8*time.Second).UnixNano()),
					fmt.Sprintf(`{"timestamp":"%d","sourceId":"source-x","counter":{"name":"x","total":"3"},"tags":{"deployment":"cf"}}`, now.UnixNano()),
				}
			case "source-y":
				return []string{
					fmt.Sprintf(`{"timestamp":"%d","sourceId":"source-y","counter":{"name":"y","total":"10"},"tags":{"deployment":"cf"}}`, now.Add(-10*time.Second).UnixNano()),
					fmt.Sprintf(`{"timestamp":"%d","sourceId":"source-y","gauge":{"metrics":{"other":{"value":7}}}}`, now.Add(-9*time.Second).UnixNano()),
					fmt.Sprintf(`{"timestamp":"%d","sourceId":"source-y","gauge":{"metrics":{"y":{"value":12}}}}`, now.Add(-8*time.Second).UnixNano()),
				}
			default:
				panic("unexpected source-id")
			}
		}
		args := []string{`max_over_time(x{source_id="source-x",deployment="cf"}[1h])+max_over_time(y{source_id="source-y",deployment="cf"}[1h])`}
		command.Query(context.Background(), cliConn, f, args, httpClient, logger, writer)

		Expect(strings.Join(writer.lines(), " ")).To(ContainSubstring("=> 110 @["))
	})

	It("executes the tail command with specified start and end time for each metric", func() {
		var (
			sourceIDs []string
			starts    []int64
			ends      []int64
		)

		tailer := func(sourceID string, start, end time.Time) []string {
			sourceIDs = append(sourceIDs, sourceID)
			starts = append(starts, start.UnixNano())
			ends = append(ends, end.UnixNano())
			return nil
		}
		args := []string{`max_over_time(x{source_id="source-x",deployment="cf"}[1h])+max_over_time(y{source_id="source-y",deployment="cf"}[1h])`}
		command.Query(context.Background(), cliConn, tailer, args, httpClient, logger, writer)

		Expect(sourceIDs).To(ConsistOf("source-x", "source-y"))
		now := time.Now().UnixNano()
		Expect(starts).To(ConsistOf(
			BeNumerically("~", now-int64(time.Hour), 10*time.Minute),
			BeNumerically("~", now-int64(time.Hour), 10*time.Minute),
		))
		Expect(ends).To(ConsistOf(
			BeNumerically("~", now, 10*time.Minute),
			BeNumerically("~", now, 10*time.Minute),
		))
	})

	It("fatally logs if the query is not provided", func() {
		f := func(sourceID string, start, end time.Time) []string { return nil }
		Expect(func() {
			command.Query(context.Background(), cliConn, f, nil, httpClient, logger, writer)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("Expected 1 argument, got 0."))
	})

	It("fatally logs if the query is not valid", func() {
		f := func(sourceID string, start, end time.Time) []string { return nil }
		args := []string{"..invalid.."}
		Expect(func() {
			command.Query(context.Background(), cliConn, f, args, httpClient, logger, writer)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("Invalid query: parse error at char 1: unexpected character: '.'"))
	})

	It("fatally logs if the metric does not have a source_id label", func() {
		f := func(sourceID string, start, end time.Time) []string { return nil }
		args := []string{`max_over_time(x[1h])+max_over_time(y{source_id="source-y",deployment="cf"}[1h])`}
		Expect(func() {
			command.Query(context.Background(), cliConn, f, args, httpClient, logger, writer)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("Metric 'x' does not have a 'source_id' label."))
	})

	It("fatally logs if the json fails to unmarshal", func() {
		f := func(sourceID string, start, end time.Time) []string {
			switch sourceID {
			case "source-x":
				return []string{
					`{"timestamp":"invalid"}`,
				}
			case "source-y":
				return []string{
					`{"timestamp":"300080080103","sourceId":"source-y","counter":{"name":"y","total":"10"}}`,
					`{"timestamp":"301000000000","sourceId":"source-y","gauge":{"metrics":{"other":{"value":7}}}}`,
					`{"timestamp":"400000000000","sourceId":"source-y","gauge":{"metrics":{"y":{"value":12}}}}`,
				}
			default:
				panic("unexpected source-id")
			}
		}

		args := []string{`max_over_time(x{source_id="source-x",deployment="cf"}[1h])+max_over_time(y{source_id="source-y",deployment="cf"}[1h])`}
		Expect(func() {
			command.Query(context.Background(), cliConn, f, args, httpClient, logger, writer)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("Failed to unmarshal JSON from LogCache: invalid character 'i' looking for beginning of value"))
	})
})
