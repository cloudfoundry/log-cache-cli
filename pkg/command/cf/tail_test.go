package cf_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"code.cloudfoundry.org/log-cache-cli/pkg/command/cf"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LogCache", func() {
	var (
		logger     *stubLogger
		writer     *stubWriter
		httpClient *stubHTTPClient
		cliConn    *stubCliConnection
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

		cliConn = newStubCliConnection()
	})

	It("removes headers when not printing to a tty", func() {
		cf.Tail(
			context.Background(),
			cliConn,
			[]string{"app-name"},
			httpClient,
			logger,
			writer,
			cf.WithTailNoHeaders(),
		)

		logFormat := "   %s [APP/PROC/WEB/0] %s log body"
		Expect(writer.lines()).To(Equal([]string{
			fmt.Sprintf(logFormat, startTime.Format(timeFormat), "ERR"),
			fmt.Sprintf(logFormat, startTime.Add(1*time.Second).Format(timeFormat), "OUT"),
			fmt.Sprintf(logFormat, startTime.Add(2*time.Second).Format(timeFormat), "OUT"),
		}))
	})

	Context("when the source is an app", func() {
		BeforeEach(func() {
			cliConn.cliCommandResult = [][]string{
				{"app-guid"},
			}
			cliConn.usernameResp = "a-user"
			cliConn.orgName = "organization"
			cliConn.spaceName = "space"
		})

		It("reports successful results", func() {
			cf.Tail(
				context.Background(),
				cliConn,
				[]string{"app-name"},
				httpClient,
				logger,
				writer,
			)
			Expect(httpClient.requestURLs).To(HaveLen(1))

			requestURL, err := url.Parse(httpClient.requestURLs[0])
			end, err := strconv.ParseInt(requestURL.Query().Get("end_time"), 10, 64)
			Expect(err).ToNot(HaveOccurred())
			Expect(end).To(BeNumerically("~", time.Now().UnixNano(), 10000000))

			logFormat := "   %s [APP/PROC/WEB/0] %s log body"
			Expect(writer.lines()).To(Equal([]string{
				fmt.Sprintf(
					"Retrieving logs for app %s in org %s / space %s as %s...",
					"app-name",
					cliConn.orgName,
					cliConn.spaceName,
					cliConn.usernameResp,
				),
				"",
				fmt.Sprintf(logFormat, startTime.Format(timeFormat), "ERR"),
				fmt.Sprintf(logFormat, startTime.Add(1*time.Second).Format(timeFormat), "OUT"),
				fmt.Sprintf(logFormat, startTime.Add(2*time.Second).Format(timeFormat), "OUT"),
			}))
		})

		It("reports successful results with deprecated tags", func() {
			httpClient.responseBody = []string{
				deprecatedTagsResponseBody(startTime),
			}
			cf.Tail(
				context.Background(),
				cliConn,
				[]string{"app-name"},
				httpClient,
				logger,
				writer,
			)

			Expect(httpClient.requestURLs).To(HaveLen(1))
			requestURL, err := url.Parse(httpClient.requestURLs[0])
			end, err := strconv.ParseInt(requestURL.Query().Get("end_time"), 10, 64)
			Expect(err).ToNot(HaveOccurred())
			Expect(end).To(BeNumerically("~", time.Now().UnixNano(), 10000000))
			logFormat := "   %s [APP/PROC/WEB/0] OUT log body"
			Expect(writer.lines()).To(Equal([]string{
				fmt.Sprintf(
					"Retrieving logs for app %s in org %s / space %s as %s...",
					"app-name",
					cliConn.orgName,
					cliConn.spaceName,
					cliConn.usernameResp,
				),
				"",
				fmt.Sprintf(logFormat, startTime.Format(timeFormat)),
				fmt.Sprintf(logFormat, startTime.Add(1*time.Second).Format(timeFormat)),
				fmt.Sprintf(logFormat, startTime.Add(2*time.Second).Format(timeFormat)),
			}))
		})

		It("reports successful results with counter envelopes", func() {
			httpClient.responseBody = []string{
				counterResponseBody(startTime),
			}
			cf.Tail(
				context.Background(),
				cliConn,
				[]string{"app-name"},
				httpClient,
				logger,
				writer,
			)

			Expect(httpClient.requestURLs).To(HaveLen(1))
			requestURL, err := url.Parse(httpClient.requestURLs[0])
			end, err := strconv.ParseInt(requestURL.Query().Get("end_time"), 10, 64)
			Expect(err).ToNot(HaveOccurred())
			Expect(end).To(BeNumerically("~", time.Now().UnixNano(), 10000000))
			logFormat := "   %s [%s/%s] COUNTER %s:%d"
			Expect(writer.lines()).To(Equal([]string{
				fmt.Sprintf(
					"Retrieving logs for app %s in org %s / space %s as %s...",
					"app-name",
					cliConn.orgName,
					cliConn.spaceName,
					cliConn.usernameResp,
				),
				"",
				fmt.Sprintf(logFormat, startTime.Format(timeFormat), "app-name", "0", "some-name", 99),
			}))
		})

		It("reports successful results with gauge envelopes", func() {
			httpClient.responseBody = []string{
				gaugeResponseBody(startTime),
			}
			cf.Tail(
				context.Background(),
				cliConn,
				[]string{"app-name"},
				httpClient,
				logger,
				writer,
			)

			Expect(httpClient.requestURLs).To(HaveLen(1))
			requestURL, err := url.Parse(httpClient.requestURLs[0])
			end, err := strconv.ParseInt(requestURL.Query().Get("end_time"), 10, 64)
			Expect(err).ToNot(HaveOccurred())
			Expect(end).To(BeNumerically("~", time.Now().UnixNano(), 10000000))
			logFormat := "   %s [%s/%s] GAUGE %s:%f %s %s:%f %s"
			Expect(writer.lines()).To(Equal([]string{
				fmt.Sprintf(
					"Retrieving logs for app %s in org %s / space %s as %s...",
					"app-name",
					cliConn.orgName,
					cliConn.spaceName,
					cliConn.usernameResp,
				),
				"",
				fmt.Sprintf(logFormat, startTime.Format(timeFormat), "app-name", "0", "some-name", 99.0, "my-unit", "some-other-name", 101.0, "my-unit"),
			}))
		})

		It("reports successful results with timer envelopes", func() {
			httpClient.responseBody = []string{
				timerResponseBody(startTime),
			}
			ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
			defer cancel()

			cf.Tail(
				ctx,
				cliConn,
				[]string{"app-name"},
				httpClient,
				logger,
				writer,
			)

			Expect(httpClient.requestURLs).To(HaveLen(1))
			requestURL, err := url.Parse(httpClient.requestURLs[0])
			end, err := strconv.ParseInt(requestURL.Query().Get("end_time"), 10, 64)
			Expect(err).ToNot(HaveOccurred())
			Expect(end).To(BeNumerically("~", time.Now().UnixNano(), 10000000))
			logFormat := "   %s [%s/%s] TIMER %s %f ms"
			Expect(writer.lines()).To(Equal([]string{
				fmt.Sprintf(
					"Retrieving logs for app %s in org %s / space %s as %s...",
					"app-name",
					cliConn.orgName,
					cliConn.spaceName,
					cliConn.usernameResp,
				),
				"",
				fmt.Sprintf(logFormat, startTime.Format(timeFormat), "app-name", "0", "http", float64(time.Second)/1000000.0),
			}))
		})

		It("doens't report the instance id if the envelopeDoesn't have one", func() {
			httpClient.responseBody = []string{
				mixedResponseBodyNoInstanceId(startTime),
			}
			ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
			defer cancel()

			cf.Tail(
				ctx,
				cliConn,
				[]string{"app-name"},
				httpClient,
				logger,
				writer,
			)

			lines := writer.lines()
			Expect(lines).To(HaveLen(7))
			for i := 2; i < len(lines); i++ { //Exclude the header
				Expect(lines[i]).To(SatisfyAny(
					ContainSubstring("[app-name]"),
					ContainSubstring("[APP/PROC/WEB]")))
			}
		})

		It("writes out json", func() {
			httpClient.responseBody = []string{
				mixedResponseBody(startTime),
			}
			ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
			defer cancel()

			args := []string{"--envelope-type", "any", "--json", "app-name"}
			cf.Tail(
				ctx,
				cliConn,
				args,
				httpClient,
				logger,
				writer,
			)

			Expect(writer.bytes).To(MatchJSON(fmt.Sprintf(`{"batch":[
				{"timestamp":"%d","source_id":"app-name","instance_id":"0","event":{"title":"some-title","body":"some-body"}},
				{"timestamp":"%d","source_id":"app-name","instance_id":"0","timer":{"name":"http","start":"1517940773000000000","stop":"1517940773000000000"}},
				{"timestamp":"%d","source_id":"app-name","instance_id":"0","gauge":{"metrics":{"some-name":{"unit":"my-unit","value":99}}}},
				{"timestamp":"%d","source_id":"app-name","instance_id":"0","counter":{"name":"some-name","total":"99"}},
				{"timestamp":"%d","source_id":"app-name","instance_id":"0","tags":{"source_type":"APP/PROC/WEB"},"log":{"payload":"bG9nIGJvZHk="}}
			]}`, startTime.UnixNano(), startTime.UnixNano(), startTime.UnixNano(), startTime.UnixNano(), startTime.UnixNano())))
		})

		It("only returns timer, gauge, and counter when type=metrics", func() {
			httpClient.responseBody = []string{
				mixedResponseBody(startTime),
			}
			ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
			defer cancel()

			args := []string{"--type", "metrics", "--json", "app-name"}
			cf.Tail(
				ctx,
				cliConn,
				args,
				httpClient,
				logger,
				writer,
			)

			Expect(writer.bytes).To(MatchJSON(fmt.Sprintf(`{"batch":[
				{"timestamp":"%d","source_id":"app-name","instance_id":"0","timer":{"name":"http","start":"1517940773000000000","stop":"1517940773000000000"}},
				{"timestamp":"%d","source_id":"app-name","instance_id":"0","gauge":{"metrics":{"some-name":{"unit":"my-unit","value":99}}}},
				{"timestamp":"%d","source_id":"app-name","instance_id":"0","counter":{"name":"some-name","total":"99"}}
			]}`, startTime.UnixNano(), startTime.UnixNano(), startTime.UnixNano())))

			Expect(httpClient.requestURLs).ToNot(BeEmpty())
			requestURL, err := url.Parse(httpClient.requestURLs[0])
			Expect(err).ToNot(HaveOccurred())
			envelopeType := requestURL.Query().Get("envelope_types")
			Expect(envelopeType).To(Equal("ANY"))
		})

		It("only returns logs and events when type=logs", func() {
			httpClient.responseBody = []string{
				mixedResponseBody(startTime),
			}
			ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
			defer cancel()

			args := []string{"--type", "logs", "--json", "app-name"}
			cf.Tail(
				ctx,
				cliConn,
				args,
				httpClient,
				logger,
				writer,
			)

			Expect(writer.bytes).To(MatchJSON(fmt.Sprintf(`{"batch":[
				{"timestamp":"%d","source_id":"app-name","instance_id":"0","event":{"title":"some-title","body":"some-body"}},
				{"timestamp":"%d","source_id":"app-name","instance_id":"0","tags":{"source_type":"APP/PROC/WEB"},"log":{"payload":"bG9nIGJvZHk="}}
			]}`, startTime.UnixNano(), startTime.UnixNano())))

			Expect(httpClient.requestURLs).ToNot(BeEmpty())
			requestURL, err := url.Parse(httpClient.requestURLs[0])
			Expect(err).ToNot(HaveOccurred())
			envelopeType := requestURL.Query().Get("envelope_types")
			Expect(envelopeType).To(Equal("ANY"))
		})

		It("filters when given gauge-name flag", func() {
			httpClient.responseBody = []string{
				mixedResponseBody(startTime),
			}
			ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
			defer cancel()

			args := []string{"--gauge-name", "some-name", "--json", "app-name"}

			cf.Tail(
				ctx,
				cliConn,
				args,
				httpClient,
				logger,
				writer,
			)

			Expect(writer.bytes).To(MatchJSON(
				fmt.Sprintf(`{"batch":[{"timestamp":"%d","source_id":"app-name","instance_id":"0","gauge":{"metrics":{"some-name":{"unit":"my-unit","value":99}}}}]}`, startTime.UnixNano()),
			))

			Expect(httpClient.requestURLs).ToNot(BeEmpty())
			requestURL, err := url.Parse(httpClient.requestURLs[0])
			Expect(err).ToNot(HaveOccurred())
			envelopeType := requestURL.Query().Get("envelope_types")
			Expect(envelopeType).To(Equal("GAUGE"))
		})

		It("filters when given counter-name flag", func() {
			httpClient.responseBody = []string{
				mixedResponseBody(startTime),
			}
			ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
			defer cancel()

			args := []string{"--counter-name", "some-name", "--json", "app-name"}
			cf.Tail(
				ctx,
				cliConn,
				args,
				httpClient,
				logger,
				writer,
			)

			Expect(writer.bytes).To(MatchJSON(
				fmt.Sprintf(`{"batch":[{"timestamp":"%d","source_id":"app-name","instance_id":"0","counter":{"name":"some-name","total":"99"}}]}`, startTime.UnixNano()),
			))

			Expect(httpClient.requestURLs).ToNot(BeEmpty())
			requestURL, err := url.Parse(httpClient.requestURLs[0])
			Expect(err).ToNot(HaveOccurred())
			envelopeType := requestURL.Query().Get("envelope_types")
			Expect(envelopeType).To(Equal("COUNTER"))
		})

		It("reports successful results when following", func() {
			httpClient.responseBody = []string{
				// Lines mode requests WithDescending
				responseBody(startTime.Add(-30 * time.Second)),
				// Walk uses ascending order
				responseBodyAsc(startTime),
				responseBodyAsc(startTime.Add(3 * time.Second)),
			}
			logFormat := "   %s [APP/PROC/WEB/0] %s log body"

			ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
			defer cancel()
			now := time.Now()
			cf.Tail(
				ctx,
				cliConn,
				[]string{"--follow", "app-name"},
				httpClient,
				logger,
				writer,
			)

			Expect(httpClient.requestURLs).ToNot(BeEmpty())
			requestURL, err := url.Parse(httpClient.requestURLs[0])

			start, err := strconv.ParseInt(requestURL.Query().Get("start_time"), 10, 64)
			Expect(err).ToNot(HaveOccurred())
			Expect(start).To(Equal(int64(0)))

			end, err := strconv.ParseInt(requestURL.Query().Get("end_time"), 10, 64)
			Expect(err).ToNot(HaveOccurred())
			Expect(end).To(BeNumerically("~", now.UnixNano(), time.Second))

			envelopeType := requestURL.Query().Get("envelope_types")
			Expect(envelopeType).To(Equal("ANY"))

			requestURL, err = url.Parse(httpClient.requestURLs[1])
			start, err = strconv.ParseInt(requestURL.Query().Get("start_time"), 10, 64)
			Expect(err).ToNot(HaveOccurred())
			Expect(start).To(Equal(startTime.Add(-28*time.Second).UnixNano() + 1))

			Expect(writer.lines()).To(ConsistOf(
				fmt.Sprintf(
					"Retrieving logs for app %s in org %s / space %s as %s...",
					"app-name",
					cliConn.orgName,
					cliConn.spaceName,
					cliConn.usernameResp,
				),
				"",
				fmt.Sprintf(logFormat, startTime.Add(-30*time.Second).Format(timeFormat), "ERR"),
				fmt.Sprintf(logFormat, startTime.Add(-29*time.Second).Format(timeFormat), "OUT"),
				fmt.Sprintf(logFormat, startTime.Add(-28*time.Second).Format(timeFormat), "OUT"),
				fmt.Sprintf(logFormat, startTime.Format(timeFormat), "OUT"),
				fmt.Sprintf(logFormat, startTime.Add(1*time.Second).Format(timeFormat), "OUT"),
				fmt.Sprintf(logFormat, startTime.Add(2*time.Second).Format(timeFormat), "ERR"),
				fmt.Sprintf(logFormat, startTime.Add(3*time.Second).Format(timeFormat), "OUT"),
				fmt.Sprintf(logFormat, startTime.Add(4*time.Second).Format(timeFormat), "OUT"),
				fmt.Sprintf(logFormat, startTime.Add(5*time.Second).Format(timeFormat), "ERR"),
			))

			Expect(cliConn.accessTokenCount).To(Equal(1))
		})

		It("respects short flag for following", func() {
			httpClient.responseBody = []string{
				// Lines mode requests WithDescending
				responseBody(startTime.Add(-30 * time.Second)),
				// Walk uses ascending order
				responseBodyAsc(startTime),
				responseBodyAsc(startTime.Add(3 * time.Second)),
			}
			logFormat := "   %s [APP/PROC/WEB/0] %s log body"

			ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
			defer cancel()
			now := time.Now()
			cf.Tail(
				ctx,
				cliConn,
				[]string{"-f", "app-name"},
				httpClient,
				logger,
				writer,
			)

			Expect(httpClient.requestURLs).ToNot(BeEmpty())
			requestURL, err := url.Parse(httpClient.requestURLs[0])

			start, err := strconv.ParseInt(requestURL.Query().Get("start_time"), 10, 64)
			Expect(err).ToNot(HaveOccurred())
			Expect(start).To(Equal(int64(0)))

			end, err := strconv.ParseInt(requestURL.Query().Get("end_time"), 10, 64)
			Expect(err).ToNot(HaveOccurred())
			Expect(end).To(BeNumerically("~", now.UnixNano(), time.Second))

			envelopeType := requestURL.Query().Get("envelope_types")
			Expect(envelopeType).To(Equal("ANY"))

			requestURL, err = url.Parse(httpClient.requestURLs[1])
			start, err = strconv.ParseInt(requestURL.Query().Get("start_time"), 10, 64)
			Expect(err).ToNot(HaveOccurred())
			Expect(start).To(Equal(startTime.Add(-28*time.Second).UnixNano() + 1))

			Expect(writer.lines()).To(ConsistOf(
				fmt.Sprintf(
					"Retrieving logs for app %s in org %s / space %s as %s...",
					"app-name",
					cliConn.orgName,
					cliConn.spaceName,
					cliConn.usernameResp,
				),
				"",
				fmt.Sprintf(logFormat, startTime.Add(-30*time.Second).Format(timeFormat), "ERR"),
				fmt.Sprintf(logFormat, startTime.Add(-29*time.Second).Format(timeFormat), "OUT"),
				fmt.Sprintf(logFormat, startTime.Add(-28*time.Second).Format(timeFormat), "OUT"),
				fmt.Sprintf(logFormat, startTime.Format(timeFormat), "OUT"),
				fmt.Sprintf(logFormat, startTime.Add(1*time.Second).Format(timeFormat), "OUT"),
				fmt.Sprintf(logFormat, startTime.Add(2*time.Second).Format(timeFormat), "ERR"),
				fmt.Sprintf(logFormat, startTime.Add(3*time.Second).Format(timeFormat), "OUT"),
				fmt.Sprintf(logFormat, startTime.Add(4*time.Second).Format(timeFormat), "OUT"),
				fmt.Sprintf(logFormat, startTime.Add(5*time.Second).Format(timeFormat), "ERR"),
			))

			Expect(cliConn.accessTokenCount).To(Equal(1))
		})

		It("filters when given counter-name flag while following", func() {
			httpClient.responseBody = []string{
				mixedResponseBody(startTime),
			}

			ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
			defer cancel()
			args := []string{"--counter-name", "some-name", "--json", "--follow", "app-name"}
			cf.Tail(
				ctx,
				cliConn,
				args,
				httpClient,
				logger,
				writer,
			)

			Expect(writer.lines()).To(ConsistOf(
				fmt.Sprintf(`{"timestamp":"%d","source_id":"app-name","instance_id":"0","counter":{"name":"some-name","total":"99"}}`, startTime.UnixNano()),
			))

			Expect(httpClient.requestURLs).ToNot(BeEmpty())
			requestURL, err := url.Parse(httpClient.requestURLs[0])
			Expect(err).ToNot(HaveOccurred())
			envelopeType := requestURL.Query().Get("envelope_types")
			Expect(envelopeType).To(Equal("COUNTER"))
		})

		It("uses the LOG_CACHE_ADDR environment variable", func() {
			os.Setenv("LOG_CACHE_ADDR", "https://different-log-cache:8080")
			defer os.Unsetenv("LOG_CACHE_ADDR")

			cf.Tail(
				context.Background(),
				cliConn,
				[]string{"app-name"},
				httpClient,
				logger,
				writer,
			)
			Expect(httpClient.requestURLs).To(HaveLen(1))

			u, err := url.Parse(httpClient.requestURLs[0])
			Expect(err).ToNot(HaveOccurred())
			Expect(u.Scheme).To(Equal("https"))
			Expect(u.Host).To(Equal("different-log-cache:8080"))
		})

		It("does not send Authorization header with LOG_CACHE_SKIP_AUTH", func() {
			os.Setenv("LOG_CACHE_SKIP_AUTH", "true")
			defer os.Unsetenv("LOG_CACHE_SKIP_AUTH")

			cf.Tail(
				context.Background(),
				cliConn,
				[]string{"app-name"},
				httpClient,
				logger,
				writer,
			)
			Expect(httpClient.requestHeaders[0]).To(BeEmpty())
		})

		It("follow retries for empty responses", func() {
			httpClient.responseBody = []string{emptyResponseBody()}

			go cf.Tail(
				context.Background(),
				cliConn,
				[]string{"--follow", "app-name"},
				httpClient,
				logger,
				writer,
			)

			Eventually(httpClient.requestCount).Should(BeNumerically(">", 3))
		})

		It("follow retries for an error", func() {
			httpClient.responseBody = nil
			httpClient.responseErr = errors.New("some-error")

			go cf.Tail(
				context.Background(),
				cliConn,
				[]string{"--follow", "app-name"},
				httpClient,
				logger,
				writer,
			)

			Eventually(httpClient.requestCount).Should(BeNumerically(">", 2))
		})

		It("reports successful results with event envelopes", func() {
			httpClient.responseBody = []string{
				eventResponseBody(startTime),
			}
			cf.Tail(
				context.Background(),
				cliConn,
				[]string{"app-name"},
				httpClient,
				logger,
				writer,
			)

			Expect(httpClient.requestURLs).To(HaveLen(1))
			requestURL, err := url.Parse(httpClient.requestURLs[0])
			end, err := strconv.ParseInt(requestURL.Query().Get("end_time"), 10, 64)
			Expect(err).ToNot(HaveOccurred())
			Expect(end).To(BeNumerically("~", time.Now().UnixNano(), 10000000))
			logFormat := "   %s [%s/%s] EVENT %s:%s"
			Expect(writer.lines()).To(Equal([]string{
				fmt.Sprintf(
					"Retrieving logs for app %s in org %s / space %s as %s...",
					"app-name",
					cliConn.orgName,
					cliConn.spaceName,
					cliConn.usernameResp,
				),
				"",
				fmt.Sprintf(logFormat, startTime.Format(timeFormat), "app-name", "0", "some-title", "some-body"),
			}))
		})

		It("accepts start-time, end-time, envelope-type, and lines flags", func() {
			args := []string{
				"--start-time", "100",
				"--end-time", "123",
				"--envelope-type", "gauge", // deliberately lowercase
				"--lines", "99",
				"app-name",
			}
			cf.Tail(
				context.Background(),
				cliConn,
				args,
				httpClient,
				logger,
				writer,
			)

			Expect(httpClient.requestURLs).To(HaveLen(1))
			requestURL, err := url.Parse(httpClient.requestURLs[0])
			Expect(err).ToNot(HaveOccurred())
			Expect(requestURL.Scheme).To(Equal("https"))
			Expect(requestURL.Host).To(Equal("log-cache.some-system.com"))
			Expect(requestURL.Path).To(Equal("/v1/read/app-guid"))
			Expect(requestURL.Query().Get("start_time")).To(Equal("100"))
			Expect(requestURL.Query().Get("end_time")).To(Equal("123"))
			Expect(requestURL.Query().Get("envelope_types")).To(Equal("GAUGE"))
			Expect(requestURL.Query().Get("descending")).To(Equal("true"))
			Expect(requestURL.Query().Get("limit")).To(Equal("99"))
		})

		It("accepts lines flags (short)", func() {
			args := []string{
				"-n", "99",
				"app-name",
			}
			cf.Tail(
				context.Background(),
				cliConn,
				args,
				httpClient,
				logger,
				writer,
			)

			Expect(httpClient.requestURLs).To(HaveLen(1))
			requestURL, err := url.Parse(httpClient.requestURLs[0])
			Expect(err).ToNot(HaveOccurred())
			Expect(requestURL.Query().Get("limit")).To(Equal("99"))
		})

		It("defaults lines flag to 10", func() {
			args := []string{
				"app-name",
			}
			cf.Tail(
				context.Background(),
				cliConn,
				args,
				httpClient,
				logger,
				writer,
			)

			Expect(httpClient.requestURLs).To(HaveLen(1))
			requestURL, err := url.Parse(httpClient.requestURLs[0])
			Expect(err).ToNot(HaveOccurred())
			Expect(requestURL.Query().Get("limit")).To(Equal("10"))
		})

		It("requests the app guid", func() {
			args := []string{"some-app"}
			cf.Tail(
				context.Background(),
				cliConn,
				args,
				httpClient,
				logger,
				writer,
			)

			Expect(cliConn.cliCommandArgs).To(HaveLen(1))
			Expect(cliConn.cliCommandArgs[0]).To(HaveLen(3))
			Expect(cliConn.cliCommandArgs[0][0]).To(Equal("app"))
			Expect(cliConn.cliCommandArgs[0][1]).To(Equal("some-app"))
			Expect(cliConn.cliCommandArgs[0][2]).To(Equal("--guid"))
		})

		It("places the auth token in the 'Authorization' header", func() {
			args := []string{"some-app"}
			cliConn.accessToken = "bearer some-token"
			cf.Tail(
				context.Background(),
				cliConn,
				args,
				httpClient,
				logger,
				writer,
			)

			Expect(httpClient.requestHeaders).To(HaveLen(1))
			Expect(httpClient.requestHeaders[0]).To(HaveLen(1))
			Expect(httpClient.requestHeaders[0].Get("Authorization")).To(Equal("bearer some-token"))
		})

		It("formats the output via text/template", func() {
			httpClient.responseBody = []string{responseBody(time.Unix(0, 1))}
			args := []string{
				"--output-format", `{{.Timestamp}} {{printf "%s" .GetLog.GetPayload}}`,
				"app-guid",
			}

			cf.Tail(
				context.Background(),
				cliConn,
				args,
				httpClient,
				logger,
				writer,
			)

			Expect(writer.lines()).To(ContainElement("1 log body"))
		})

		It("formats the output via text/template (short flag)", func() {
			httpClient.responseBody = []string{responseBody(time.Unix(0, 1))}
			args := []string{
				"-o", `{{.Timestamp}} {{printf "%s" .GetLog.GetPayload}}`,
				"app-guid",
			}

			cf.Tail(
				context.Background(),
				cliConn,
				args,
				httpClient,
				logger,
				writer,
			)

			Expect(writer.lines()).To(ContainElement("1 log body"))
		})

		It("allows for empty end time with populated start time", func() {
			args := []string{"--start-time", "1000", "app-name"}
			Expect(func() {
				cf.Tail(
					context.Background(),
					cliConn,
					args,
					httpClient,
					logger,
					writer,
				)
			}).ToNot(Panic())
		})

		It("fatally logs if envelope-type is invalid", func() {
			args := []string{"--envelope-type", "invalid", "some-app"}
			Expect(func() {
				cf.Tail(
					context.Background(),
					cliConn,
					args,
					httpClient,
					logger,
					writer,
				)
			}).To(Panic())

			Expect(logger.fatalfMessage).To(Equal("--envelope-type must be LOG, COUNTER, GAUGE, TIMER, EVENT or ANY"))
		})

		It("fatally logs if gauge-name and envelope-type flags are both set", func() {
			args := []string{
				"--gauge-name", "some-name",
				"--envelope-type", "LOG",
				"some-app",
			}
			Expect(func() {
				cf.Tail(
					context.Background(),
					cliConn,
					args,
					httpClient,
					logger,
					writer,
				)
			}).To(Panic())

			Expect(logger.fatalfMessage).To(Equal("--gauge-name cannot be used with --envelope-type"))
		})

		It("fatally logs when envelope-type and type are both present", func() {
			args := []string{
				"--type", "metrics",
				"--envelope-type", "counter",
				"--json",
				"app-name",
			}

			Expect(func() {
				cf.Tail(
					context.Background(),
					cliConn,
					args,
					httpClient,
					logger,
					writer,
				)
			}).To(Panic())

			Expect(logger.fatalfMessage).To(Equal("--envelope-type cannot be used with --type"))
		})

		It("fatally logs if counter-name and envelope-type flags are both set", func() {
			args := []string{
				"--counter-name", "some-name",
				"--envelope-type", "LOG",
				"some-app",
			}
			Expect(func() {
				cf.Tail(
					context.Background(),
					cliConn,
					args,
					httpClient,
					logger,
					writer,
				)
			}).To(Panic())

			Expect(logger.fatalfMessage).To(Equal("--counter-name cannot be used with --envelope-type"))
		})

		It("fatally logs if counter-name and gauge-name flags are both set", func() {
			args := []string{
				"--counter-name", "some-name",
				"--gauge-name", "some-name",
				"some-app",
			}
			Expect(func() {
				cf.Tail(
					context.Background(),
					cliConn,
					args,
					httpClient,
					logger,
					writer,
				)
			}).To(Panic())

			Expect(logger.fatalfMessage).To(Equal("--counter-name cannot be used with --gauge-name"))
		})

		It("fatally logs if output-format and json flags are given", func() {
			httpClient.responseBody = []string{responseBody(time.Unix(0, 1))}
			args := []string{
				"--output-format", `{{.Timestamp}} {{printf "%s" .GetLog.GetPayload}}`,
				"--json",
				"app-guid",
			}

			Expect(func() {
				cf.Tail(
					context.Background(),
					cliConn,
					args,
					httpClient,
					logger,
					writer,
				)
			}).To(Panic())

			Expect(logger.fatalfMessage).To(Equal("Cannot use output-format and json flags together"))
		})

		It("fatally logs if an output-format is malformed", func() {
			args := []string{"--output-format", "{{INVALID}}", "app-guid"}
			Expect(func() {
				cf.Tail(
					context.Background(),
					cliConn,
					args,
					httpClient,
					logger,
					writer,
				)
			}).To(Panic())

			Expect(logger.fatalfMessage).To(Equal(`template: OutputFormat:1: function "INVALID" not defined`))
		})

		It("fatally logs if an output-format won't execute", func() {
			httpClient.responseBody = []string{`{"envelopes":{"batch":[{"source_id": "a", "timestamp": 1},{"source_id":"b", "timestamp":2}]}}`}
			args := []string{
				"--output-format", "{{.invalid 9}}",
				"app-guid",
			}

			Expect(func() {
				cf.Tail(
					context.Background(),
					cliConn,
					args,
					httpClient,
					logger,
					writer,
				)
			}).To(Panic())

			Expect(logger.fatalfMessage).To(Equal(`Output template parsed, but failed to execute: template: OutputFormat:1:2: executing "OutputFormat" at <.invalid>: can't evaluate field invalid in type *loggregator_v2.Envelope`))
		})

		It("fatally logs if lines is greater than 1000", func() {
			args := []string{
				"--lines", "1001",
				"some-app",
			}
			Expect(func() {
				cf.Tail(
					context.Background(),
					cliConn,
					args,
					httpClient,
					logger,
					writer,
				)
			}).To(Panic())

			Expect(logger.fatalfMessage).To(Equal("Lines cannot be greater than 1000."))
		})

		It("accepts 0 for --lines", func() {
			args := []string{
				"--lines", "0",
				"some-app",
			}
			Expect(func() {
				cf.Tail(
					context.Background(),
					cliConn,
					args,
					httpClient,
					logger,
					writer,
				)
			}).ToNot(Panic())
		})

		It("fatally logs if username cannot be fetched", func() {
			cliConn.usernameErr = errors.New("unknown user")
			args := []string{"app-name"}

			Expect(func() {
				cf.Tail(
					context.Background(),
					cliConn,
					args,
					httpClient,
					logger,
					writer,
				)
			}).To(Panic())

			Expect(logger.fatalfMessage).To(Equal("unknown user"))
		})

		It("fatally logs if org name cannot be fetched", func() {
			cliConn.orgErr = errors.New("Organization could not be fetched")
			args := []string{"app-name"}

			Expect(func() {
				cf.Tail(
					context.Background(),
					cliConn,
					args,
					httpClient,
					logger,
					writer,
				)
			}).To(Panic())

			Expect(logger.fatalfMessage).To(Equal("Organization could not be fetched"))
		})

		It("fatally logs if space cannot be fetched", func() {
			cliConn.spaceErr = errors.New("unknown space")
			args := []string{"app-name"}

			Expect(func() {
				cf.Tail(
					context.Background(),
					cliConn,
					args,
					httpClient,
					logger,
					writer,
				)
			}).To(Panic())

			Expect(logger.fatalfMessage).To(Equal("unknown space"))
		})

		It("fatally logs if the start > end", func() {
			args := []string{"--start-time", "1000", "--end-time", "100", "app-name"}
			Expect(func() {
				cf.Tail(
					context.Background(),
					cliConn,
					args,
					httpClient,
					logger,
					writer,
				)
			}).To(Panic())

			Expect(logger.fatalfMessage).To(Equal("Invalid date/time range. Ensure your start time is prior or equal the end time."))
		})

		It("fatally logs if too many arguments are given", func() {
			Expect(func() {
				cf.Tail(
					context.Background(),
					cliConn,
					[]string{"one", "two"},
					httpClient,
					logger,
					writer,
				)
			}).To(Panic())

			Expect(logger.fatalfMessage).To(Equal("Expected 1 argument, got 2."))
		})

		It("fatally logs if not enough arguments are given", func() {
			Expect(func() {
				cf.Tail(
					context.Background(),
					cliConn,
					[]string{},
					httpClient,
					logger,
					writer,
				)
			}).To(Panic())

			Expect(logger.fatalfMessage).To(Equal("Expected 1 argument, got 0."))
		})

		It("fatally logs if there is an error while getting API endpoint", func() {
			cliConn.apiEndpointErr = errors.New("some-error")

			Expect(func() {
				cf.Tail(
					context.Background(),
					cliConn,
					[]string{"app-name"},
					httpClient,
					logger,
					writer,
				)
			}).To(Panic())

			Expect(logger.fatalfMessage).To(Equal("some-error"))
		})

		It("fatally logs if there is no API endpoint", func() {
			cliConn.hasAPIEndpoint = false

			Expect(func() {
				cf.Tail(
					context.Background(),
					cliConn,
					[]string{"app-name"},
					httpClient,
					logger,
					writer,
				)
			}).To(Panic())

			Expect(logger.fatalfMessage).To(Equal("No API endpoint targeted."))
		})

		It("fatally logs if there is an error while checking for API endpoint", func() {
			cliConn.hasAPIEndpoint = true
			cliConn.hasAPIEndpointErr = errors.New("some-error")

			Expect(func() {
				cf.Tail(
					context.Background(),
					cliConn,
					[]string{"app-name"},
					httpClient,
					logger,
					writer,
				)
			}).To(Panic())

			Expect(logger.fatalfMessage).To(Equal("some-error"))
		})

		It("fatally logs if the request returns an error", func() {
			httpClient.responseErr = errors.New("some-error")

			Expect(func() {
				cf.Tail(
					context.Background(),
					cliConn,
					[]string{"app-name"},
					httpClient,
					logger,
					writer,
				)
			}).To(Panic())

			Expect(logger.fatalfMessage).To(Equal("some-error"))
		})
	})

	Context("when the source is a service", func() {
		BeforeEach(func() {
			cliConn.usernameResp = "a-user"
			cliConn.orgName = "organization"
			cliConn.spaceName = "space"

			cliConn.cliCommandResult = [][]string{{""}, {"service-guid"}}

			httpClient.responseBody = []string{gaugeResponseBody(startTime)}

		})

		It("reports successful results", func() {
			cliConn.cliCommandResult = [][]string{
				{""},
				{"service-guid"},
			}
			args := []string{"service-name"}
			cf.Tail(
				context.Background(),
				cliConn,
				args,
				httpClient,
				logger,
				writer,
			)

			logFormat := "   %s [%s/%s] GAUGE %s:%f %s %s:%f %s"
			Expect(writer.lines()).To(Equal([]string{
				fmt.Sprintf(
					"Retrieving logs for service %s in org %s / space %s as %s...",
					"service-name",
					cliConn.orgName,
					cliConn.spaceName,
					cliConn.usernameResp,
				),
				"",
				fmt.Sprintf(logFormat, startTime.Format(timeFormat), "service-name", "0", "some-name", 99.0, "my-unit", "some-other-name", 101.0, "my-unit"),
			}))
		})

		It("requests the service guid when app --guid fails", func() {
			cliConn.cliCommandResult = [][]string{{"not", "an", "app"}, {"service-guid"}}
			cliConn.cliCommandErr = []error{errors.New("catch this instead")}

			args := []string{"app-name"}
			cf.Tail(
				context.Background(),
				cliConn,
				args,
				httpClient,
				logger,
				writer,
			)

			logFormat := "   %s [%s/%s] GAUGE %s:%f %s %s:%f %s"
			Expect(writer.lines()).To(Equal([]string{
				fmt.Sprintf(
					"Retrieving logs for service %s in org %s / space %s as %s...",
					"app-name",
					cliConn.orgName,
					cliConn.spaceName,
					cliConn.usernameResp,
				),
				"",
				fmt.Sprintf(logFormat, startTime.Format(timeFormat), "app-name", "0", "some-name", 99.0, "my-unit", "some-other-name", 101.0, "my-unit"),
			}))

			Expect(logger.printfMessages).To(ContainElement("catch this instead"))
		})

		It("calls the log cache api", func() {
			args := []string{"service-name"}
			cf.Tail(
				context.Background(),
				cliConn,
				args,
				httpClient,
				logger,
				writer,
			)

			Expect(httpClient.requestURLs).To(HaveLen(1))

			requestURL, err := url.Parse(httpClient.requestURLs[0])
			Expect(err).ToNot(HaveOccurred())

			end, err := strconv.ParseInt(requestURL.Query().Get("end_time"), 10, 64)
			Expect(err).ToNot(HaveOccurred())

			Expect(end).To(BeNumerically("~", time.Now().UnixNano(), 10000000))
		})

		It("requests the service guid", func() {
			args := []string{"some-service"}
			cf.Tail(
				context.Background(),
				cliConn,
				args,
				httpClient,
				logger,
				writer,
			)

			Expect(cliConn.cliCommandArgs).To(HaveLen(2))
			Expect(cliConn.cliCommandArgs[1]).To(HaveLen(3))
			Expect(cliConn.cliCommandArgs[1][0]).To(Equal("service"))
			Expect(cliConn.cliCommandArgs[1][1]).To(Equal("some-service"))
			Expect(cliConn.cliCommandArgs[1][2]).To(Equal("--guid"))
		})

	})

	Context("when the source is a component", func() {
		BeforeEach(func() {
			cliConn.usernameResp = "a-user"
			httpClient.responseBody = []string{counterResponseBody(startTime)}
		})

		It("requests as a source id", func() {
			cliConn.cliCommandResult = [][]string{{""}, {""}}
			cliConn.cliCommandErr = []error{errors.New("app not found"), errors.New("service not found")}

			args := []string{"app-name"}
			cf.Tail(
				context.Background(),
				cliConn,
				args,
				httpClient,
				logger,
				writer,
			)

			Expect(httpClient.requestURLs).To(HaveLen(1))

			requestURL, err := url.Parse(httpClient.requestURLs[0])
			end, err := strconv.ParseInt(requestURL.Query().Get("end_time"), 10, 64)
			Expect(err).ToNot(HaveOccurred())
			Expect(end).To(BeNumerically("~", time.Now().UnixNano(), 10000000))

			Expect(requestURL.Path).To(Equal("/v1/read/app-name"))

			counterFormat := "   %s [%s/%s] COUNTER %s:%d"
			Expect(writer.lines()).To(Equal([]string{
				fmt.Sprintf(
					"Retrieving logs for source %s as %s...",
					"app-name",
					cliConn.usernameResp,
				),
				"",
				fmt.Sprintf(counterFormat, startTime.Format(timeFormat), "app-name", "0", "some-name", 99),
			}))

			Expect(logger.printfMessages).To(ContainElement("app not found"))
			Expect(logger.printfMessages).To(ContainElement("service not found"))
		})

		It("uses the LOG_CACHE_ADDR environment variable", func() {
			os.Setenv("LOG_CACHE_ADDR", "https://different-log-cache:8080")
			defer os.Unsetenv("LOG_CACHE_ADDR")

			cliConn.cliCommandResult = [][]string{{""}, {""}}
			cliConn.cliCommandErr = []error{errors.New("app not found"), errors.New("service not found")}

			cf.Tail(
				context.Background(),
				cliConn,
				[]string{"app-name"},
				httpClient,
				logger,
				writer,
			)
			Expect(httpClient.requestURLs).To(HaveLen(1))

			u, err := url.Parse(httpClient.requestURLs[0])
			Expect(err).ToNot(HaveOccurred())
			Expect(u.Scheme).To(Equal("https"))
			Expect(u.Host).To(Equal("different-log-cache:8080"))
			Expect(u.Path).To(ContainSubstring("app-name"))
		})
	})
})

func responseBody(startTime time.Time) string {
	// NOTE: These are in descending order.
	return fmt.Sprintf(responseTemplate,
		startTime.Add(2*time.Second).UnixNano(),
		startTime.Add(1*time.Second).UnixNano(),
		startTime.UnixNano(),
	)
}

func responseBodyAsc(startTime time.Time) string {
	// NOTE: These are in ascending order.
	return fmt.Sprintf(responseTemplate,
		startTime.UnixNano(),
		startTime.Add(1*time.Second).UnixNano(),
		startTime.Add(2*time.Second).UnixNano(),
	)
}

func deprecatedTagsResponseBody(startTime time.Time) string {
	// NOTE: These are in descending order.
	return fmt.Sprintf(deprecatedTagsResponseTemplate,
		startTime.Add(2*time.Second).UnixNano(),
		startTime.Add(1*time.Second).UnixNano(),
		startTime.UnixNano(),
	)
}

func counterResponseBody(startTime time.Time) string {
	return fmt.Sprintf(counterResponseTemplate,
		startTime.UnixNano(),
	)
}

func gaugeResponseBody(startTime time.Time) string {
	return fmt.Sprintf(gaugeResponseTemplate,
		startTime.UnixNano(),
	)
}

func timerResponseBody(startTime time.Time) string {
	return fmt.Sprintf(timerResponseTemplate,
		startTime.UnixNano(),
		startTime.Add(1*time.Second).UnixNano(),
		startTime.Add(2*time.Second).UnixNano(),
	)
}

func eventResponseBody(startTime time.Time) string {
	return fmt.Sprintf(eventResponseTemplate,
		startTime.UnixNano(),
	)
}

func mixedResponseBody(startTime time.Time) string {
	return fmt.Sprintf(mixedResponseTemplate,
		startTime.UnixNano(),
		"0",
	)
}

func mixedResponseBodyNoInstanceId(startTime time.Time) string {
	return fmt.Sprintf(mixedResponseTemplate,
		startTime.UnixNano(),
		"",
	)
}

func emptyResponseBody() string {
	return `{ "envelopes": { "batch": [] } }`
}

var responseTemplate = `{
	"envelopes": {
		"batch": [
			{
				"timestamp":"%d",
				"source_id": "app-name",
				"instance_id":"0",
				"tags":{
					"source_type":"APP/PROC/WEB"
				},
				"log":{
					"payload":"bG9nIGJvZHk="
				}
			},
			{
				"timestamp":"%d",
				"source_id": "app-name",
				"instance_id":"0",
				"tags":{
					"source_type":"APP/PROC/WEB"
				},
				"log":{
					"payload":"bG9nIGJvZHk="
				}
			},
			{
				"timestamp":"%d",
				"source_id": "app-name",
				"instance_id":"0",
				"tags":{
					"source_type":"APP/PROC/WEB"
				},
				"log":{
					"payload":"bG9nIGJvZHk=",
					"type": "ERR"
				}
			}
		]
	}
}`

var deprecatedTagsResponseTemplate = `{
	"envelopes": {
		"batch": [
			{
				"timestamp":"%d",
				"instance_id":"0",
				"deprecated_tags": {
					"source_type":{"text":"APP/PROC/WEB"}
				},
				"log":{"payload":"bG9nIGJvZHk="}
			},
			{
				"timestamp":"%d",
				"instance_id":"0",
				"deprecated_tags": {
					"source_type":{"text":"APP/PROC/WEB"}
				},
				"log":{"payload":"bG9nIGJvZHk="}
			},
			{
				"timestamp":"%d",
				"instance_id":"0",
				"deprecated_tags": {
					"source_type":{"text":"APP/PROC/WEB"}
				},
				"log":{"payload":"bG9nIGJvZHk="}
			}
		]
	}
}`

var counterResponseTemplate = `{
	"envelopes": {
		"batch": [
			{
				"source_id": "app-name",
				"instance_id":"0",
				"timestamp":"%d",
				"counter":{"name":"some-name","total":99}
			}
		]
	}
}`

var gaugeResponseTemplate = `{
	"envelopes": {
		"batch": [
			{
				"source_id": "app-name",
				"instance_id":"0",
				"timestamp": "%d",
				"gauge": {
				  "metrics": {
					"some-name": {
					  "value": 99,
					  "unit":"my-unit"
					},
					"some-other-name": {
					  "value": 101,
					  "unit":"my-unit"
					}
				  }
				}
			  }
		]
	}
}`

var timerResponseTemplate = `{
	"envelopes": {
		"batch": [
			{
				"source_id": "app-name",
				"timestamp": "%d",
				"instance_id":"0",
				"timer": {
					"name": "http",
					"start": "%d",
					"stop": "%d"
				}
			}
		]
	}
}`

var eventResponseTemplate = `{
	"envelopes": {
		"batch": [
			{
				"source_id": "app-name",
				"instance_id":"0",
				"timestamp": "%d",
				"event": {
					"title": "some-title",
					"body": "some-body"
				}
			}
		]
	}
}`

var invalidTimestampResponse = `{
	"envelopes": {
		"batch": [
			{
				"source_id": "app-name",
				"timestamp":"not-a-timestamp",
				"instance_id":"0",
				"deprecated_tags": {
					"source_type":{"text":"APP/PROC/WEB"}
				},
				"log":{"payload":"bG9nIGJvZHk="}
			}
		]
	}
}`

var invalidPayloadResponse = `{
	"envelopes": {
		"batch": [
			{
				"source_id": "app-name",
				"timestamp":"0",
				"instance_id":"0",
				"deprecated_tags": {
					"source_type":{"text":"APP/PROC/WEB"}
				},
				"log":{"payload":"~*&#"}
			}
		]
	}
}`

var mixedResponseTemplate = `{
	"envelopes": {
		"batch": [
			{
				"source_id": "app-name",
				"timestamp":"%[1]d",
				"instance_id":"%[2]s",
				"tags":{
					"source_type":"APP/PROC/WEB"
				},
				"log":{
					"payload":"bG9nIGJvZHk="
				}
			},
			{
				"source_id": "app-name",
				"instance_id":"%[2]s",
				"timestamp":"%[1]d",
				"instance_id":"%[2]s",
				"counter":{"name":"some-name","total":99}
			},
			{
				"source_id": "app-name",
				"instance_id":"%[2]s",
				"timestamp":"%[1]d",
				"gauge": {
					"metrics": {
						"some-name": {
							"value": 99,
							"unit":"my-unit"
						}
					}
				}
			},
			{
				"source_id": "app-name",
				"instance_id":"%[2]s",
				"timestamp":"%[1]d",
				"timer": {
					"name": "http",
					"start": "1517940773000000000",
					"stop": "1517940773000000000"
				}
			},
			{
				"source_id": "app-name",
				"instance_id":"%[2]s",
				"timestamp":"%[1]d",
				"event": {
					"title": "some-title",
					"body": "some-body"
				}
			}
		]
	}
}`
