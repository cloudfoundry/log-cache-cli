package command_test

import (
	"context"
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
		f := func(sourceID string, start, end time.Time) []string {
			switch sourceID {
			case "source-x":
				return []string{
					`{"timestamp":"300100000002","sourceId":"source-x","counter":{"name":"x","total":"100"},"tags":{"deployment":"other"}}`,
					`{"timestamp":"300100000003","sourceId":"source-x","counter":{"name":"x","total":"1"},"tags":{"deployment":"cf","__name__":"other","source_id":"other"}}`,
					`{"timestamp":"300100000004","sourceId":"source-x","counter":{"name":"x","total":"100"},"tags":{"deployment":"other"}}`,
					`{"timestamp":"301000000000","sourceId":"source-x","counter":{"name":"other","total":"2"}}`,
					`{"timestamp":"400000101179","sourceId":"source-x","counter":{"name":"x","total":"3"},"tags":{"deployment":"cf"}}`,
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
		args := []string{`max(x{source_id="source-x",deployment="cf"}) + max(y{source_id="source-y"})`, "--start-time", "300000000000", "--end-time", "400000000000"}
		command.Query(context.Background(), cliConn, f, args, httpClient, logger, writer)

		Expect(strings.Join(writer.lines(), " ")).To(Equal("{} => 11 @[300000] 15 @[400000]"))
	})

	It("executes the tail command with specified start and end time for each metric", func() {
		var (
			sourceIDs []string
			starts    []time.Time
			ends      []time.Time
		)

		tailer := func(sourceID string, start, end time.Time) []string {
			sourceIDs = append(sourceIDs, sourceID)
			starts = append(starts, start)
			ends = append(ends, end)
			return nil
		}
		args := []string{`x{source_id="source-x"} + y{source_id="source-y"}`, "--start-time", "300000000000", "--end-time", "600000000000"}
		command.Query(context.Background(), cliConn, tailer, args, httpClient, logger, writer)

		Expect(sourceIDs).To(ConsistOf("source-x", "source-y"))
		Expect(starts).To(ConsistOf(
			time.Unix(0, 300000000000),
			time.Unix(0, 300000000000),
		))
		Expect(ends).To(ConsistOf(
			time.Unix(0, 600000000000),
			time.Unix(0, 600000000000),
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
		args := []string{`x + y{source_id="source-y"}`, "--start-time", "300000000000", "--end-time", "600000000000"}
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

		args := []string{`max(x{source_id="source-x"}) + max(y{source_id="source-y"})`, "--start-time", "300000000000", "--end-time", "400000000000"}
		Expect(func() {
			command.Query(context.Background(), cliConn, f, args, httpClient, logger, writer)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("Failed to unmarshal JSON from LogCache: invalid character 'i' looking for beginning of value"))
	})
})
