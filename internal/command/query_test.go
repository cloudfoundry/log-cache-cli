package command_test

import (
	"context"
	"fmt"
	"net/url"

	"code.cloudfoundry.org/log-cache-cli/v4/internal/command"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("LogCache", func() {
	Describe("error handling for queries", func() {
		It("reports an error for a failed request", func() {
			tc := setup("", 503)

			tc.query(`placeholder-for-a-query`)

			Expect(tc.writer.lines()).To(Equal([]string{
				"Could not process query: unexpected status code 503",
			}))
		})

		It("reports an error for a failed request", func() {
			tc := setup("", 500)

			tc.query(`placeholder-for-a-query`)

			Expect(tc.writer.lines()).To(Equal([]string{
				"Could not process query: unexpected end of JSON input (status code 500)",
			}))
		})

		It("reports the returned error message for an invalid PromQL query", func() {
			json := `{"status":"error","errorType":"bad_data","error": "query does not request any source_ids"}`
			tc := setup(json, 400)

			tc.query(`not-a-valid-query`)

			Expect(tc.writer.lines()).To(Equal([]string{
				"The PromQL API returned an error (bad_data): query does not request any source_ids",
			}))
		})

		It("hints at authorization failures when receiving a 404", func() {
			tc := setup("", 404)

			tc.query(`not-a-valid-query`)

			Expect(tc.writer.lines()).To(Equal([]string{
				"Could not process query: unexpected status code 404 (check authorization?)",
			}))
		})

		It("exits with an error when no query is provided", func() {
			tc := setup("", 200)

			Expect(func() {
				tc.query()
			}).To(Panic())

			Expect(tc.logger.fatalfMessage).To(HavePrefix(`Must specify a PromQL query`))
			Expect(tc.httpClient.requestURLs).To(HaveLen(0))
		})
	})

	Describe("parsing command line flags", func() {
		It("gives you an error if you supply only a subset of --start, --end, or --step", func() {
			tc := setup("", 200)

			Expect(func() {
				tc.query(`egress{source_id="doppler"}`, "--start", "123", "--end", "456")
			}).To(Panic())

			Expect(tc.logger.fatalfMessage).To(HavePrefix(
				"When issuing a range query, you must specify all of --start, --end, and --step",
			))
		})

		It("gives you an error if you mix --time with --start, --end, or --step", func() {
			tc := setup("", 200)

			Expect(func() {
				tc.query(`egress{source_id="doppler"}`, "--time", "123", "--start", "321")
			}).To(Panic())

			Expect(tc.logger.fatalfMessage).To(HavePrefix(
				"When issuing an instant query, you cannot specify --start, --end, or --step",
			))
		})

		Context("when issuing an instant query", func() {
			It("passes the query to the /api/v1/query when no flags are provided", func() {
				json := `{"status":"success","data":{"resultType":"scalar","result":[1.234,"2.5"]}}`
				tc := setup(json, 200)

				tc.query(`egress{source_id="doppler"}`)
				Expect(tc.httpClient.requestURLs).To(HaveLen(1))

				requestURL, err := url.Parse(tc.httpClient.requestURLs[0])
				Expect(err).ToNot(HaveOccurred())

				Expect(requestURL.Path).To(Equal("/api/v1/query"))
				params := requestURL.Query()
				query := params.Get("query")
				Expect(query).To(Equal(`egress{source_id="doppler"}`))

				_, found := params["time"]
				Expect(found).To(BeFalse())

				Expect(tc.writer.lines()).To(Equal([]string{json}))
				Expect(tc.cliConnection.accessTokenCount).To(Equal(1))
			})

			It("passes the query and time correctly to the /api/v1/query when the --time flag is provided", func() {
				tc := setup("", 200)

				tc.query(
					`egress{source_id="doppler"}`,
					"--time", "123",
				)
				Expect(tc.httpClient.requestURLs).To(HaveLen(1))

				requestURL, err := url.Parse(tc.httpClient.requestURLs[0])
				Expect(err).ToNot(HaveOccurred())

				Expect(requestURL.Path).To(Equal("/api/v1/query"))
				query := requestURL.Query().Get("query")
				Expect(query).To(Equal(`egress{source_id="doppler"}`))
				time := requestURL.Query().Get("time")
				Expect(time).To(Equal("123.000"))
			})

			DescribeTable("with valid times",
				func(timeArg string) {
					tc := setup("", 200)

					Expect(func() {
						tc.query(`egress{source_id="doppler"}`, "--time", timeArg)
					}).NotTo(Panic())
				},
				Entry("with a valid integer", "123456789"),
				Entry("with a valid RFC3339 timestamp", "2018-02-23T19:00:00Z"),
			)

			DescribeTable("with invalid times",
				func(timeArg string) {
					tc := setup("", 200)

					Expect(func() {
						tc.query(`egress{source_id="doppler"}`, "--time", timeArg)
					}).To(Panic())

					Expect(tc.logger.fatalfMessage).To(HavePrefix(
						fmt.Sprintf("Couldn't parse --time: invalid time format: %s", timeArg),
					))
				},
				Entry("with an arbitary string", "asdfkj"),
				Entry("with an unsupported duration", "5d"),
				Entry("with a malformed RFC3339 timestamp", "2018-02-23T19:00:00"),
			)
		})

		Context("when issuing a range query", func() {
			It("correctly uses the /api/v1/query_range endpoint when the --start, --end, and --step flags are provided", func() {
				json := `{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"__name__":"egress"},"values":[[1.234,"2.5"]]}]}}`
				tc := setup(json, 200)

				tc.query(
					`egress{source_id="doppler"}`,
					"--start", "123",
					"--end", "456",
					"--step", "15s",
				)
				Expect(tc.httpClient.requestURLs).To(HaveLen(1))

				requestURL, err := url.Parse(tc.httpClient.requestURLs[0])
				Expect(err).ToNot(HaveOccurred())

				Expect(requestURL.Path).To(Equal("/api/v1/query_range"))
				start := requestURL.Query().Get("start")
				Expect(start).To(Equal("123.000"))
				end := requestURL.Query().Get("end")
				Expect(end).To(Equal("456.000"))
				step := requestURL.Query().Get("step")
				Expect(step).To(Equal("15s"))

				Expect(tc.writer.lines()).To(Equal([]string{json}))
				Expect(tc.cliConnection.accessTokenCount).To(Equal(1))
			})

			DescribeTable("with valid times",
				func(startArg, endArg, stepArg string) {
					tc := setup("", 200)

					Expect(func() {
						tc.query(
							`egress{source_id="doppler"}`,
							"--start", startArg,
							"--end", endArg,
							"--step", stepArg,
						)
					}).NotTo(Panic())
				},
				Entry("with a valid integer timestamps", "123456789", "987654321", "15s"),
				Entry("with a valid RFC3339 timestamps", "2018-02-23T19:00:00Z", "2018-08-23T19:00:00Z", "1m"),
				Entry("with mixed timestamps", "123456789", "2018-08-23T19:00:00Z", "1m"),
			)

			DescribeTable("with invalid times",
				func(invalidField, invalidArg string) {
					tc := setup("", 200)

					args := map[string]string{
						"start": "123",
						"end":   "456",
						"step":  "15s",
					}

					args[invalidField] = invalidArg

					Expect(func() {
						tc.query(
							`egress{source_id="doppler"}`,
							"--start", args["start"],
							"--end", args["end"],
							"--step", args["step"],
						)
					}).To(Panic())

					Expect(tc.logger.fatalfMessage).To(HavePrefix(
						fmt.Sprintf("Couldn't parse --%s: invalid time format: %s", invalidField, invalidArg),
					))
				},
				Entry("with an arbitary string for start", "start", "asdfkj"),
				Entry("with an arbitary string for end", "end", "asdfkj"),
				Entry("with an unsupported duration for start", "start", "5d"),
				Entry("with an unsupported duration for end", "end", "5d"),
				Entry("with a malformed RFC3339 timestamp for start", "start", "2018-02-23T19:00:00"),
				Entry("with a malformed RFC3339 timestamp for end", "end", "2018-02-23T19:00:00"),
			)
		})
	})
})

type testContext struct {
	cliConnection *stubCliConnection
	httpClient    *stubHTTPClient
	logger        *stubLogger
	writer        *stubWriter
}

func setup(responseBody string, responseCode int) *testContext {
	httpClient := newStubHTTPClient()
	httpClient.responseBody = []string{responseBody}
	httpClient.responseCode = responseCode

	return &testContext{
		cliConnection: newStubCliConnection(),
		httpClient:    httpClient,
		logger:        &stubLogger{},
		writer:        &stubWriter{},
	}
}

func (tc *testContext) query(args ...string) {
	command.Query(
		context.Background(),
		tc.cliConnection,
		args,
		tc.httpClient,
		tc.logger,
		tc.writer,
	)
}
