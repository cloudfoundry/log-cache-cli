package command_test

import (
	"context"
	"errors"
	"log"
	"net/url"

	"code.cloudfoundry.org/log-cache-cli/v4/internal/command"
	"code.cloudfoundry.org/log-cache-cli/v4/internal/util/query"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Query", func() {
	var instantSuccessResp string
	var rangeSuccessResp string

	BeforeEach(func() {
		instantSuccessResp = `{"status":"success","data":{"resultType":"scalar","result":[1.234,"2.5"]}}`
		rangeSuccessResp = `{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"__name__":"egress"},"values":[[1.234,"2.5"]]}]}}`
	})

	DescribeTable("Validating command line args",
		func(query, time, start, end, step string, expectedErr error) {
			tc := setup(instantSuccessResp, 200)

			var args []string
			if query != "" {
				args = append(args, query)
			}
			if time != "" {
				args = append(args, "--time", time)
			}
			if start != "" {
				args = append(args, "--start", start)
			}
			if end != "" {
				args = append(args, "--end", end)
			}
			if step != "" {
				args = append(args, "--step", step)
			}
			err := tc.query(args)
			if expectedErr != nil {
				Expect(err).Should(MatchError(expectedErr))
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("when no args are provided", "", "", "", "", "", query.ErrNoQuery),
		Entry("when no query is provided but other flags are provided", "", "", "1657680966", "1657680966", "15s", query.ErrNoQuery),
		Entry("when only a query is provided", `egress{source_id="doppler"}`, "", "", "", "", nil),
		Entry("when a time flag is provided (Unix Timestamp)", `egress{source_id="doppler"}`, "1657680966", "", "", "", nil),
		Entry("when a time flag is provided (RFC3339 DateTime)", `egress{source_id="doppler"}`, "2019-10-12T07:20:50.52Z", "", "", "", nil),
		Entry("when a bad time flag is provided", `egress{source_id="doppler"}`, "badtime", "", "", "", query.ArgError{Arg: "-time"}),
		Entry("when time and start flags are provided", `egress{source_id="doppler"}`, "1657680966", "1657680966", "", "", query.ArgError{Arg: "-time", Msg: "cannot use flag along with -start, -end, or -step"}),
		Entry("when time and end flags are provided", `egress{source_id="doppler"}`, "1657680966", "", "1657680966", "", query.ArgError{Arg: "-time", Msg: "cannot use flag along with -start, -end, or -step"}),
		Entry("when time and step flags are provided", `egress{source_id="doppler"}`, "1657680966", "", "", "15s", query.ArgError{Arg: "-time", Msg: "cannot use flag along with -start, -end, or -step"}),
		Entry("when start, end, and step flags are provided (Unix Timestamp)", `egress{source_id="doppler"}`, "", "1657680966", "1657680970", "1ns", nil),
		Entry("when start, end, and step flags are provided (RFC3339 DateTime)", `egress{source_id="doppler"}`, "", "2019-10-12T07:20:50.52Z", "2019-11-12T07:20:50.52Z", "1h", nil),
		Entry("when a bad start flag is provided", `egress{source_id="doppler"}`, "", "badstart", "2019-11-12T07:20:50.52Z", "1m", query.ArgError{Arg: "-start"}),
		Entry("when a bad end flag is provided", `egress{source_id="doppler"}`, "", "2019-10-12T07:20:50.52Z", "badend", "1m", query.ArgError{Arg: "-end"}),
		// TODO: setup test for bad step flag
		// Entry("when a bad step flag is provided", `egress{source_id="doppler"}`, "", "2019-10-12T07:20:50.52Z", "2019-11-12T07:20:50.52Z", "badstep", query.ArgError{Arg: "-step"}),
	)

	Describe("Instant query", func() {
		var args = []string{
			`egress{source_id="doppler"}`,
		}

		It("makes an authenticated request to the Log Cache query endpoint", func() {
			tc := setup(instantSuccessResp, 200)

			err := tc.query(args)
			Expect(err).NotTo(HaveOccurred())

			Expect(tc.httpClient.requestHeaders).To(HaveLen(1))
			Expect(tc.httpClient.requestHeaders[0].Get("Authorization")).To(Equal("fake-token"))
			Expect(tc.cliConnection.accessTokenCount).To(Equal(1))

			Expect(tc.httpClient.requestURLs).To(HaveLen(1))
			requestURL, err := url.Parse(tc.httpClient.requestURLs[0])
			Expect(err).NotTo(HaveOccurred())

			Expect(requestURL.Path).To(Equal("/api/v1/query"))
		})

		It("puts the correct params in the URL", func() {
			tc := setup(instantSuccessResp, 200)

			err := tc.query(args)
			Expect(err).NotTo(HaveOccurred())

			Expect(tc.httpClient.requestURLs).To(HaveLen(1))
			requestURL, err := url.Parse(tc.httpClient.requestURLs[0])
			Expect(err).ToNot(HaveOccurred())

			params := requestURL.Query()
			Expect(params.Get("query")).To(Equal(`egress{source_id="doppler"}`))
			Expect(params.Has("time")).To(BeFalse())
		})

		It("prints out the result", func() {
			tc := setup(instantSuccessResp, 200)

			err := tc.query(args)
			Expect(err).NotTo(HaveOccurred())

			Expect(tc.writer.lines()).To(Equal([]string{`{"resultType":"scalar","result":[1.234,"2.5"]}`}))
			Expect(tc.cliConnection.accessTokenCount).To(Equal(1))
		})

		Context("when the -time flag is provided", func() {
			It("is included in the request to Log Cache", func() {
				tc := setup(instantSuccessResp, 200)

				args := make([]string, 0, 3)
				args = append(args, `egress{source_id="doppler"}`)
				args = append(args, "--time", "2019-10-12T07:20:50.52Z")
				err := tc.query(args)
				Expect(err).NotTo(HaveOccurred())

				Expect(tc.httpClient.requestURLs).To(HaveLen(1))
				requestURL, err := url.Parse(tc.httpClient.requestURLs[0])
				Expect(err).NotTo(HaveOccurred())

				Expect(requestURL.Path).To(Equal("/api/v1/query"))

				Expect(requestURL.Query().Get("time")).To(Equal("1570864850.520"))
			})
		})
	})

	Describe("Range query", func() {
		var args = []string{
			`egress{source_id="doppler"}`,
			"--start", "2019-10-12T07:20:50.52Z",
			"--end", "2019-10-12T08:20:50.52Z",
			"--step", "15s",
		}

		It("makes an authenticated request to the Log Cache query_range endpoint", func() {
			tc := setup(rangeSuccessResp, 200)

			err := tc.query(args)
			Expect(err).NotTo(HaveOccurred())

			Expect(tc.httpClient.requestURLs).To(HaveLen(1))
			requestURL, err := url.Parse(tc.httpClient.requestURLs[0])
			Expect(err).ToNot(HaveOccurred())

			Expect(tc.httpClient.requestHeaders).To(HaveLen(1))
			Expect(tc.httpClient.requestHeaders[0].Get("Authorization")).To(Equal("fake-token"))
			Expect(tc.cliConnection.accessTokenCount).To(Equal(1))

			Expect(requestURL.Path).To(Equal("/api/v1/query_range"))
		})

		It("puts the correct params in the URL", func() {
			tc := setup(rangeSuccessResp, 200)

			err := tc.query(args)
			Expect(err).NotTo(HaveOccurred())

			Expect(tc.httpClient.requestURLs).To(HaveLen(1))
			requestURL, err := url.Parse(tc.httpClient.requestURLs[0])
			Expect(err).ToNot(HaveOccurred())

			params := requestURL.Query()
			Expect(params.Get("query")).To(Equal(`egress{source_id="doppler"}`))
			Expect(params.Get("start")).To(Equal("1570864850.520"))
			Expect(params.Get("end")).To(Equal("1570868450.520"))
			Expect(params.Get("step")).To(Equal("15s"))
		})

		It("prints out the result", func() {
			tc := setup(rangeSuccessResp, 200)

			err := tc.query(args)
			Expect(err).NotTo(HaveOccurred())

			Expect(tc.writer.lines()).To(Equal([]string{`{"resultType":"matrix","result":[{"metric":{"__name__":"egress"},"values":[[1.234,"2.5"]]}]}`}))
		})
	})

	Describe("Error handling", func() {
		var (
			args = []string{
				"fake-query",
			}

			errResp = `{"status":"error","errorType":"bad_data","error": "query does not request any source_ids"}`
		)

		Context("when client fails to make request", func() {
			It("returns a ClientError", func() {
				tc := setup(errResp, 500)
				tc.httpClient.responseErr = errors.New("client error")

				err := tc.query(args)
				Expect(err).To(MatchError(query.ClientError{Msg: "client error"}))
			})
		})

		Context("when status code is not 200", func() {
			It("returns a RequestError", func() {
				tc := setup(errResp, 400)

				err := tc.query(args)
				Expect(err).To(MatchError(query.RequestError{Type: "bad_data", Msg: "query does not request any source_ids"}))
			})
		})

		// TODO: get this test working
		// Context("when request body fails to unmarshal", func() {
		// 	It("returns a UnMarshalError", func() {
		// 		tc := setup(`{"status":"success"}`, 200)

		// 		err := tc.query(args)
		// 		Expect(err).To(MatchError(query.MarshalError{Msg: ""}))
		// 	})
		// })
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

	writer := &stubWriter{}
	log.SetOutput(writer)

	return &testContext{
		cliConnection: newStubCliConnection(),
		httpClient:    httpClient,
		logger:        &stubLogger{},
		writer:        writer,
	}
}

func (tc *testContext) query(args []string) error {
	return command.Query(
		context.Background(),
		tc.cliConnection,
		args,
		tc.httpClient,
	)
}
