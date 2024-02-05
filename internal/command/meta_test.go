package command_test

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"code.cloudfoundry.org/log-cache-cli/v4/internal/command"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Meta", func() {
	var (
		logger      *stubLogger
		httpClient  *stubHTTPClient
		cliConn     *stubCliConnection
		tableWriter *bytes.Buffer
	)

	BeforeEach(func() {
		logger = &stubLogger{}
		httpClient = newStubHTTPClient()
		cliConn = newStubCliConnection()
		cliConn.cliCommandResult = [][]string{{"app-guid"}}
		cliConn.usernameResp = "a-user"
		cliConn.orgName = "organization"
		cliConn.spaceName = "space"
		tableWriter = bytes.NewBuffer(nil)
	})

	Context("when specifying a sort by flag", func() {
		It("specifying `--sort-by rate` sorts by the rate column", func() {
			httpClient.responseBody = []string{
				variedMetaResponseInfo("source-1", "source-2", "source-3", "source-4"),
				variedMetaResponseInfoButHigher([]int{5 * 5, 3 * 5, 1 * 5, 2 * 5}, "source-1", "source-2", "source-3", "source-4"),
			}

			cliConn.cliCommandResult = [][]string{
				{
					capiAppsResponse(map[string]string{
						"source-1": "app-1",
						"source-4": "app-4",
					}),
				},
				{
					capiServiceInstancesResponse(map[string]string{
						"source-3": "service-3",
					}),
				},
			}
			cliConn.cliCommandErr = nil

			command.Meta(
				cliConn,
				[]string{"--noise", "--sort-by", "rate"},
				httpClient,
				logger,
				tableWriter,
				command.WithMetaNoiseSleepDuration(0),
			)

			Expect(strings.Split(tableWriter.String(), "\n")).To(Equal([]string{
				fmt.Sprintf(
					"Retrieving log cache metadata as %s...",
					cliConn.usernameResp,
				),
				"",
				"Waiting 5 minutes then comparing log output...",
				"",
				fmt.Sprintf(
					"Retrieving log cache metadata as %s...",
					cliConn.usernameResp,
				),
				"",
				fmt.Sprintf(
					"Retrieving app and service names as %s...",
					cliConn.usernameResp,
				),
				"",
				"Source     Source Type  Count   Expired  Cache Duration  Rate/minute",
				"service-3  service      100002  85003    9m0s            1",
				"app-4      application  100006  85004    13m30s          2",
				"source-2   platform     100017  84998    4m30s           3",
				"app-1      application  100026  84999    1s              5",
				"",
			}))

			Expect(httpClient.requestCount()).To(Equal(2))
		})

		It("specifying `--sort-by source-type` sorts by the source type column", func() {
			httpClient.responseBody = []string{
				variedMetaResponseInfo("source-1", "source-2", "source-3"),
			}

			cliConn.cliCommandResult = [][]string{
				{
					capiAppsResponse(map[string]string{
						"source-1": "app-1",
					}),
				},
				{
					capiServiceInstancesResponse(map[string]string{
						"source-3": "service-3",
					}),
				},
			}
			cliConn.cliCommandErr = nil

			command.Meta(
				cliConn,
				[]string{"--sort-by", "source-type"},
				httpClient,
				logger,
				tableWriter,
			)

			Expect(strings.Split(tableWriter.String(), "\n")).To(Equal([]string{
				fmt.Sprintf(
					"Retrieving log cache metadata as %s...",
					cliConn.usernameResp,
				),
				"",
				fmt.Sprintf(
					"Retrieving app and service names as %s...",
					cliConn.usernameResp,
				),
				"",
				"Source     Source Type  Count   Expired  Cache Duration",
				"app-1      application  100001  84999    1s",
				"source-2   platform     100002  84998    4m30s",
				"service-3  service      99997   85003    9m0s",
				"",
			}))

			Expect(httpClient.requestCount()).To(Equal(1))
		})

		It("specifying `--sort-by count` sorts by the count column", func() {
			httpClient.responseBody = []string{
				variedMetaResponseInfo("source-1", "source-2", "source-3", "source-4"),
			}

			cliConn.cliCommandResult = [][]string{
				{
					capiAppsResponse(map[string]string{
						"source-1": "app-1",
						"source-2": "app-2",
						"source-3": "app-3",
						"source-4": "app-4",
					}),
				},
			}
			cliConn.cliCommandErr = nil

			command.Meta(
				cliConn,
				[]string{"--sort-by", "count"},
				httpClient,
				logger,
				tableWriter,
			)

			Expect(strings.Split(tableWriter.String(), "\n")).To(Equal([]string{
				fmt.Sprintf(
					"Retrieving log cache metadata as %s...",
					cliConn.usernameResp,
				),
				"",
				fmt.Sprintf(
					"Retrieving app and service names as %s...",
					cliConn.usernameResp,
				),
				"",
				"Source  Source Type  Count   Expired  Cache Duration",
				"app-4   application  99996   85004    13m30s",
				"app-3   application  99997   85003    9m0s",
				"app-1   application  100001  84999    1s",
				"app-2   application  100002  84998    4m30s",
				"",
			}))

			Expect(httpClient.requestCount()).To(Equal(1))
		})

		It("specifying `--sort-by expired` sorts by the expired column", func() {
			httpClient.responseBody = []string{
				variedMetaResponseInfo("source-1", "source-2", "source-3", "source-4"),
			}

			cliConn.cliCommandResult = [][]string{
				{
					capiAppsResponse(map[string]string{
						"source-1": "app-1",
						"source-2": "app-2",
						"source-3": "app-3",
						"source-4": "app-4",
					}),
				},
			}
			cliConn.cliCommandErr = nil

			command.Meta(
				cliConn,
				[]string{"--sort-by", "expired"},
				httpClient,
				logger,
				tableWriter,
			)

			Expect(strings.Split(tableWriter.String(), "\n")).To(Equal([]string{
				fmt.Sprintf(
					"Retrieving log cache metadata as %s...",
					cliConn.usernameResp,
				),
				"",
				fmt.Sprintf(
					"Retrieving app and service names as %s...",
					cliConn.usernameResp,
				),
				"",
				"Source  Source Type  Count   Expired  Cache Duration",
				"app-2   application  100002  84998    4m30s",
				"app-1   application  100001  84999    1s",
				"app-3   application  99997   85003    9m0s",
				"app-4   application  99996   85004    13m30s",
				"",
			}))

			Expect(httpClient.requestCount()).To(Equal(1))
		})

		It("specifying `--sort-by cache-duration` sorts by the cache duration column", func() {
			httpClient.responseBody = []string{
				variedMetaResponseInfo("source-1", "source-2", "source-3", "source-4"),
			}

			cliConn.cliCommandResult = [][]string{
				{
					capiAppsResponse(map[string]string{
						"source-1": "app-1",
						"source-2": "app-2",
						"source-3": "app-3",
						"source-4": "app-4",
					}),
				},
			}
			cliConn.cliCommandErr = nil

			command.Meta(
				cliConn,
				[]string{"--sort-by", "cache-duration"},
				httpClient,
				logger,
				tableWriter,
			)

			Expect(strings.Split(tableWriter.String(), "\n")).To(Equal([]string{
				fmt.Sprintf(
					"Retrieving log cache metadata as %s...",
					cliConn.usernameResp,
				),
				"",
				fmt.Sprintf(
					"Retrieving app and service names as %s...",
					cliConn.usernameResp,
				),
				"",
				"Source  Source Type  Count   Expired  Cache Duration",
				"app-1   application  100001  84999    1s",
				"app-2   application  100002  84998    4m30s",
				"app-3   application  99997   85003    9m0s",
				"app-4   application  99996   85004    13m30s",
				"",
			}))

			Expect(httpClient.requestCount()).To(Equal(1))
		})

		It("fatally logs when --sort-by is not valid", func() {
			Expect(func() {
				command.Meta(
					cliConn,
					[]string{"--sort-by", "invalid"},
					httpClient,
					logger,
					tableWriter,
				)
			}).To(Panic())

			Expect(logger.fatalfMessage).To(Equal("Sort by must be 'source-id', 'source', 'source-type', 'count', 'expired', 'cache-duration', or 'rate'."))
		})

		It("fatally logs when --source-type other than 'platform' is used with --guid", func() {
			Expect(func() {
				command.Meta(
					cliConn,
					[]string{"--guid", "--source-type", "not-platform"},
					httpClient,
					logger,
					tableWriter,
				)
			}).To(Panic())

			Expect(logger.fatalfMessage).To(Equal("Source type must be 'platform' when using the --guid flag"))
		})

		It("fatally logs when --sort-by source is used with --guid", func() {
			Expect(func() {
				command.Meta(
					cliConn,
					[]string{"--guid", "--sort-by", "source"},
					httpClient,
					logger,
					tableWriter,
				)
			}).To(Panic())

			Expect(logger.fatalfMessage).To(Equal("When using --guid, sort by must be 'source-id', 'count', 'expired', 'cache-duration', or 'rate'."))
		})

		It("fatally logs when --sort-by source-type is used with --guid", func() {
			Expect(func() {
				command.Meta(
					cliConn,
					[]string{"--guid", "--sort-by", "source-type"},
					httpClient,
					logger,
					tableWriter,
				)
			}).To(Panic())

			Expect(logger.fatalfMessage).To(Equal("When using --guid, sort by must be 'source-id', 'count', 'expired', 'cache-duration', or 'rate'."))
		})

		It("fatally logs when --sort-by source-type is used with --guid", func() {
			Expect(func() {
				command.Meta(
					cliConn,
					[]string{"--guid", "--sort-by", "source-type"},
					httpClient,
					logger,
					tableWriter,
				)
			}).To(Panic())

			Expect(logger.fatalfMessage).To(Equal("When using --guid, sort by must be 'source-id', 'count', 'expired', 'cache-duration', or 'rate'."))
		})

		It("fatally logs when --sort-by rate is used without --noise", func() {
			Expect(func() {
				command.Meta(
					cliConn,
					[]string{"--sort-by", "rate"},
					httpClient,
					logger,
					tableWriter,
				)
			}).To(Panic())

			Expect(logger.fatalfMessage).To(Equal("Can't sort by rate column without --noise flag"))
		})
	})

	It("returns app source guids in alphabetical order with no source type", func() {
		httpClient.responseBody = []string{
			metaResponseInfo("source-1", "source-2"),
		}

		cliConn.cliCommandResult = [][]string{
			{
				capiAppsResponse(map[string]string{
					"source-1": "app-2",
					"source-2": "app-1",
				}),
			},
		}
		cliConn.cliCommandErr = nil

		command.Meta(
			cliConn,
			[]string{"--guid"},
			httpClient,
			logger,
			tableWriter,
		)

		Expect(cliConn.cliCommandArgs).To(HaveLen(0))

		Expect(strings.Split(tableWriter.String(), "\n")).To(Equal([]string{
			fmt.Sprintf(
				"Retrieving log cache metadata as %s...",
				cliConn.usernameResp,
			),
			"",
			"Source ID  Count   Expired  Cache Duration",
			"source-1   100000  85008    1s",
			"source-2   100000  85008    11m45s",
			"",
		}))

		Expect(httpClient.requestCount()).To(Equal(1))
	})

	It("removes headers when not printing to a tty", func() {
		httpClient.responseBody = []string{
			metaResponseInfo("source-1", "source-2"),
		}

		cliConn.cliCommandResult = [][]string{
			{
				capiAppsResponse(map[string]string{
					"source-1": "app-2",
					"source-2": "app-1",
				}),
			},
		}
		cliConn.cliCommandErr = nil

		command.Meta(
			cliConn,
			[]string{"--guid"},
			httpClient,
			logger,
			tableWriter,
			command.WithMetaNoHeaders(),
		)

		Expect(strings.Split(tableWriter.String(), "\n")).To(Equal([]string{
			"source-1  100000  85008  1s",
			"source-2  100000  85008  11m45s",
			"",
		}))
	})

	It("does not display the Source ID column by default", func() {
		httpClient.responseBody = []string{
			metaResponseInfo("source-1"),
		}

		cliConn.cliCommandResult = [][]string{
			{
				capiAppsResponse(map[string]string{"source-1": "app-1"}),
			},
		}
		cliConn.cliCommandErr = nil

		command.Meta(
			cliConn,
			nil,
			httpClient,
			logger,
			tableWriter,
		)

		Expect(cliConn.cliCommandArgs).To(HaveLen(1))
		Expect(cliConn.cliCommandArgs[0]).To(HaveLen(2))
		Expect(cliConn.cliCommandArgs[0][0]).To(Equal("curl"))
		Expect(cliConn.cliCommandArgs[0][1]).To(Equal("/v3/apps?guids=source-1"))

		Expect(strings.Split(tableWriter.String(), "\n")).To(Equal([]string{
			fmt.Sprintf(
				"Retrieving log cache metadata as %s...",
				cliConn.usernameResp,
			),
			"",
			fmt.Sprintf(
				"Retrieving app and service names as %s...",
				cliConn.usernameResp,
			),
			"",
			"Source  Source Type  Count   Expired  Cache Duration",
			"app-1   application  100000  85008    1s",
			"",
		}))

		Expect(httpClient.requestCount()).To(Equal(1))
	})

	It("displays the rate column for each service type", func() {
		httpClient.responseBody = []string{
			metaResponseInfo(
				"source-1",
				"source-2",
				"source-3",
			),
			metaResponseInfoButHigher(
				"source-1",
				"source-2",
				"source-3",
			),
		}

		cliConn.cliCommandResult = [][]string{
			{
				capiAppsResponse(map[string]string{"source-1": "app-1"}),
			},
			{
				capiServiceInstancesResponse(map[string]string{"source-3": "service-3"}),
			},
		}
		cliConn.cliCommandErr = nil

		command.Meta(
			cliConn,
			[]string{"--noise"},
			httpClient,
			logger,
			tableWriter,
			command.WithMetaNoiseSleepDuration(0),
		)

		Expect(strings.Split(tableWriter.String(), "\n")).To(Equal([]string{
			fmt.Sprintf(
				"Retrieving log cache metadata as %s...",
				cliConn.usernameResp,
			),
			"",
			"Waiting 5 minutes then comparing log output...",
			"",
			fmt.Sprintf(
				"Retrieving log cache metadata as %s...",
				cliConn.usernameResp,
			),
			"",
			fmt.Sprintf(
				"Retrieving app and service names as %s...",
				cliConn.usernameResp,
			),
			"",
			"Source     Source Type  Count   Expired  Cache Duration  Rate/minute",
			"app-1      application  100004  85009    1s              1",
			"service-3  service      100004  85009    11m45s          1",
			"source-2   platform     100004  85009    11m45s          1",
			"",
		}))

		Expect(httpClient.requestCount()).To(Equal(2))
	})

	It("prints source IDs without app names when CAPI doesn't return info", func() {
		httpClient.responseBody = []string{
			metaResponseInfo("source-1", "source-2"),
		}

		cliConn.cliCommandResult = [][]string{
			{
				capiAppsResponse(map[string]string{"source-1": "app-1"}),
			},
			{
				capiServiceInstancesResponse(nil),
			},
		}
		cliConn.cliCommandErr = nil

		command.Meta(
			cliConn,
			nil,
			httpClient,
			logger,
			tableWriter,
		)

		Expect(cliConn.cliCommandArgs).To(HaveLen(2))

		Expect(cliConn.cliCommandArgs[0]).To(HaveLen(2))
		Expect(cliConn.cliCommandArgs[0][0]).To(Equal("curl"))
		uri, err := url.Parse(cliConn.cliCommandArgs[0][1])
		Expect(err).ToNot(HaveOccurred())
		Expect(uri.Path).To(Equal("/v3/apps"))
		guidsParam, ok := uri.Query()["guids"]
		Expect(ok).To(BeTrue())
		Expect(len(guidsParam)).To(Equal(1))
		Expect(strings.Split(guidsParam[0], ",")).To(ConsistOf("source-1", "source-2"))

		Expect(cliConn.cliCommandArgs[1]).To(HaveLen(2))
		Expect(cliConn.cliCommandArgs[1][0]).To(Equal("curl"))
		Expect(cliConn.cliCommandArgs[1][1]).To(Equal("/v2/service_instances?guids=source-2"))

		Expect(httpClient.requestCount()).To(Equal(1))
		Expect(strings.Split(tableWriter.String(), "\n")).To(Equal([]string{
			fmt.Sprintf(
				"Retrieving log cache metadata as %s...",
				cliConn.usernameResp,
			),
			"",
			fmt.Sprintf(
				"Retrieving app and service names as %s...",
				cliConn.usernameResp,
			),
			"",
			"Source    Source Type  Count   Expired  Cache Duration",
			"app-1     application  100000  85008    1s",
			"source-2  platform     100000  85008    11m45s",
			"",
		}))
	})

	It("prints meta scoped to apps with guids after names", func() {
		httpClient.responseBody = []string{
			metaResponseInfo(
				"deadbeef-dead-dead-dead-deaddeafbeef",
				"source-2",
				"026fb323-6884-4978-a45f-da188dbf8ecd",
			),
		}

		cliConn.cliCommandResult = [][]string{
			{
				capiAppsResponse(map[string]string{
					"deadbeef-dead-dead-dead-deaddeafbeef": "app-1",
				}),
			},
			{
				capiServiceInstancesResponse(nil),
			},
		}
		cliConn.cliCommandErr = nil

		args := []string{"--source-type", "application"}
		command.Meta(
			cliConn,
			args,
			httpClient,
			logger,
			tableWriter,
		)

		Expect(strings.Split(tableWriter.String(), "\n")).To(Equal([]string{
			fmt.Sprintf(
				"Retrieving log cache metadata as %s...",
				cliConn.usernameResp,
			),
			"",
			fmt.Sprintf(
				"Retrieving app and service names as %s...",
				cliConn.usernameResp,
			),
			"",
			"Source  Source Type  Count   Expired  Cache Duration",
			"app-1   application  100000  85008    1s",
			"",
		}))
	})

	It("prints meta scoped to service", func() {
		httpClient.responseBody = []string{
			metaResponseInfo(
				"source-1",
				"source-2",
				"deadbeef-dead-dead-dead-deaddeafbeef",
			),
		}

		cliConn.cliCommandResult = [][]string{
			{
				capiAppsResponse(map[string]string{"source-1": "app-1"}),
			},
			{
				capiServiceInstancesResponse(map[string]string{"source-2": "service-2"}),
			},
		}
		cliConn.cliCommandErr = nil

		args := []string{"--source-type", "service"}
		command.Meta(
			cliConn,
			args,
			httpClient,
			logger,
			tableWriter,
		)

		Expect(strings.Split(tableWriter.String(), "\n")).To(Equal([]string{
			fmt.Sprintf(
				"Retrieving log cache metadata as %s...",
				cliConn.usernameResp,
			),
			"",
			fmt.Sprintf(
				"Retrieving app and service names as %s...",
				cliConn.usernameResp,
			),
			"",
			"Source     Source Type  Count   Expired  Cache Duration",
			"service-2  service      100000  85008    11m45s",
			"",
		}))
	})

	It("prints meta scoped to platform", func() {
		httpClient.responseBody = []string{
			metaResponseInfo(
				"source-1",
				"source-2",
				"deadbeef-dead-dead-dead-deaddeafbeef",
			),
		}

		cliConn.cliCommandResult = [][]string{
			{
				capiAppsResponse(map[string]string{"source-1": "app-1"}),
			},
			{
				capiServiceInstancesResponse(nil),
			},
		}
		cliConn.cliCommandErr = nil

		args := []string{"--source-type", "PLATFORM"}
		command.Meta(
			cliConn,
			args,
			httpClient,
			logger,
			tableWriter,
		)

		Expect(strings.Split(tableWriter.String(), "\n")).To(Equal([]string{
			fmt.Sprintf(
				"Retrieving log cache metadata as %s...",
				cliConn.usernameResp,
			),
			"",
			fmt.Sprintf(
				"Retrieving app and service names as %s...",
				cliConn.usernameResp,
			),
			"",
			"Source    Source Type  Count   Expired  Cache Duration",
			"source-2  platform     100000  85008    11m45s",
			"",
		}))
	})

	It("returns unknown when sourceid is guid and not found in CAPI", func() {
		httpClient.responseBody = []string{
			metaResponseInfo(
				"source-1",
				"11111111-1111-1111-1111-111111111111",
			),
		}

		cliConn.cliCommandResult = [][]string{
			{
				capiAppsResponse(nil),
			},
			{
				capiServiceInstancesResponse(nil),
			},
		}
		cliConn.cliCommandErr = nil

		args := []string{"--source-type", "all"}
		command.Meta(
			cliConn,
			args,
			httpClient,
			logger,
			tableWriter,
		)

		Expect(strings.Split(tableWriter.String(), "\n")).To(Equal([]string{
			fmt.Sprintf(
				"Retrieving log cache metadata as %s...",
				cliConn.usernameResp,
			),
			"",
			fmt.Sprintf(
				"Retrieving app and service names as %s...",
				cliConn.usernameResp,
			),
			"",
			"Source                                Source Type  Count   Expired  Cache Duration",
			"source-1                              platform     100000  85008    1s",
			"11111111-1111-1111-1111-111111111111  unknown      100000  85008    11m45s",
			"",
		}))
	})

	It("can filter to uknown", func() {
		httpClient.responseBody = []string{
			metaResponseInfo(
				"source-1",
				"11111111-1111-1111-1111-111111111111",
			),
		}

		cliConn.cliCommandResult = [][]string{
			{
				capiAppsResponse(nil),
			},
			{
				capiServiceInstancesResponse(nil),
			},
		}
		cliConn.cliCommandErr = nil

		args := []string{"--source-type", "unknown"}
		command.Meta(
			cliConn,
			args,
			httpClient,
			logger,
			tableWriter,
		)

		Expect(strings.Split(tableWriter.String(), "\n")).To(Equal([]string{
			fmt.Sprintf(
				"Retrieving log cache metadata as %s...",
				cliConn.usernameResp,
			),
			"",
			fmt.Sprintf(
				"Retrieving app and service names as %s...",
				cliConn.usernameResp,
			),
			"",
			"Source                                Source Type  Count   Expired  Cache Duration",
			"11111111-1111-1111-1111-111111111111  unknown      100000  85008    11m45s",
			"",
		}))
	})

	It("does not return unknown source id's when all isn't provided", func() {
		httpClient.responseBody = []string{
			metaResponseInfo(
				"source-1",
				"11111111-1111-1111-1111-111111111111",
			),
		}

		cliConn.cliCommandResult = [][]string{
			{
				capiAppsResponse(nil),
			},
			{
				capiServiceInstancesResponse(nil),
			},
		}
		cliConn.cliCommandErr = nil

		command.Meta(
			cliConn,
			nil,
			httpClient,
			logger,
			tableWriter,
		)

		Expect(strings.Split(tableWriter.String(), "\n")).To(Equal([]string{
			fmt.Sprintf(
				"Retrieving log cache metadata as %s...",
				cliConn.usernameResp,
			),
			"",
			fmt.Sprintf(
				"Retrieving app and service names as %s...",
				cliConn.usernameResp,
			),
			"",
			"Source    Source Type  Count   Expired  Cache Duration",
			"source-1  platform     100000  85008    1s",
			"",
		}))
	})

	It("prints unknown when guid is provided", func() {
		httpClient.responseBody = []string{
			metaResponseInfo(
				"source-1",
				"11111111-1111-1111-1111-111111111111",
			),
		}

		cliConn.cliCommandResult = [][]string{
			{
				capiAppsResponse(nil),
			},
			{
				capiServiceInstancesResponse(nil),
			},
		}
		cliConn.cliCommandErr = nil

		args := []string{"--guid"}
		command.Meta(
			cliConn,
			args,
			httpClient,
			logger,
			tableWriter,
		)

		Expect(strings.Split(tableWriter.String(), "\n")).To(Equal([]string{
			fmt.Sprintf(
				"Retrieving log cache metadata as %s...",
				cliConn.usernameResp,
			),
			"",
			"Source ID                             Count   Expired  Cache Duration",
			"source-1                              100000  85008    1s",
			"11111111-1111-1111-1111-111111111111  100000  85008    11m45s",
			"",
		}))
	})

	It("prints meta scoped to platform with source GUIDs", func() {
		httpClient.responseBody = []string{
			metaResponseInfo(
				"source-2",
				"deadbeef-dead-dead-dead-deaddeafbeef",
			),
		}

		cliConn.cliCommandResult = [][]string{
			{
				capiAppsResponse(nil),
			},
			{
				capiServiceInstancesResponse(nil),
			},
		}
		cliConn.cliCommandErr = nil

		args := []string{"--source-type", "PLATFORM", "--guid"}
		command.Meta(
			cliConn,
			args,
			httpClient,
			logger,
			tableWriter,
		)

		Expect(strings.Split(tableWriter.String(), "\n")).To(Equal([]string{
			fmt.Sprintf(
				"Retrieving log cache metadata as %s...",
				cliConn.usernameResp,
			),
			"",
			"Source ID  Count   Expired  Cache Duration",
			"source-2   100000  85008    1s",
			"",
		}))
	})

	It("does not request more than 50 guids at a time", func() {
		var guids []string
		for i := 0; i < 51; i++ {
			guids = append(guids, fmt.Sprintf("source-%d", i))
		}

		httpClient.responseBody = []string{
			metaResponseInfo(guids...),
		}

		cliConn.cliCommandResult = [][]string{
			{
				capiAppsResponse(map[string]string{}),
			},
			{
				capiServiceInstancesResponse(nil),
			},
			{
				capiAppsResponse(map[string]string{}),
			},
			{
				capiServiceInstancesResponse(nil),
			},
		}
		cliConn.cliCommandErr = nil

		command.Meta(
			cliConn,
			nil,
			httpClient,
			logger,
			tableWriter,
		)

		Expect(cliConn.cliCommandArgs).To(HaveLen(4))

		Expect(cliConn.cliCommandArgs[0]).To(HaveLen(2))
		Expect(cliConn.cliCommandArgs[0][0]).To(Equal("curl"))
		uri, err := url.Parse(cliConn.cliCommandArgs[0][1])
		Expect(err).ToNot(HaveOccurred())
		Expect(uri.Path).To(Equal("/v3/apps"))
		Expect(strings.Split(uri.Query().Get("guids"), ",")).To(HaveLen(50))

		Expect(cliConn.cliCommandArgs[1]).To(HaveLen(2))
		Expect(cliConn.cliCommandArgs[1][0]).To(Equal("curl"))
		uri, err = url.Parse(cliConn.cliCommandArgs[1][1])
		Expect(err).ToNot(HaveOccurred())
		Expect(uri.Path).To(Equal("/v3/apps"))
		Expect(strings.Split(uri.Query().Get("guids"), ",")).To(HaveLen(1))

		Expect(cliConn.cliCommandArgs[2]).To(HaveLen(2))
		Expect(cliConn.cliCommandArgs[2][0]).To(Equal("curl"))
		uri, err = url.Parse(cliConn.cliCommandArgs[2][1])
		Expect(err).ToNot(HaveOccurred())
		Expect(uri.Path).To(Equal("/v2/service_instances"))
		Expect(strings.Split(uri.Query().Get("guids"), ",")).To(HaveLen(50))

		Expect(cliConn.cliCommandArgs[3]).To(HaveLen(2))
		Expect(cliConn.cliCommandArgs[3][0]).To(Equal("curl"))
		uri, err = url.Parse(cliConn.cliCommandArgs[3][1])
		Expect(err).ToNot(HaveOccurred())
		Expect(uri.Path).To(Equal("/v2/service_instances"))
		Expect(strings.Split(uri.Query().Get("guids"), ",")).To(HaveLen(1))

		// 51 entries, 2 blank lines, "Retrieving..." preamble and table
		// header comes to 55 lines.
		Expect(strings.Split(tableWriter.String(), "\n")).To(HaveLen(57))
	})

	It("fatally logs when it receives too many arguments", func() {
		Expect(func() {
			command.Meta(
				cliConn,
				[]string{"extra-arg"},
				httpClient,
				logger,
				tableWriter,
			)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("Invalid arguments, expected 0, got 1."))
	})

	It("fatally logs when scope is not 'platform', 'application' or 'all'", func() {
		args := []string{"--source-type", "invalid"}
		Expect(func() {
			command.Meta(
				cliConn,
				args,
				httpClient,
				logger,
				tableWriter,
			)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal("Source type must be 'platform', 'application', 'service', or 'all'."))
	})

	It("fatally logs when getting ApiEndpoint fails", func() {
		cliConn.apiEndpointErr = errors.New("some-error")

		Expect(func() {
			command.Meta(
				cliConn,
				nil,
				httpClient,
				logger,
				tableWriter,
			)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(HavePrefix(`Could not determine Log Cache endpoint: some-error`))
	})

	It("fatally logs when CAPI request fails", func() {
		httpClient.responseBody = []string{
			metaResponseInfo("source-1", "source-2"),
		}

		cliConn.cliCommandResult = [][]string{nil}
		cliConn.cliCommandErr = []error{errors.New("some-error")}

		Expect(func() {
			command.Meta(
				cliConn,
				nil,
				httpClient,
				logger,
				tableWriter,
			)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(HavePrefix(`Failed to read application information: some-error`))
	})

	It("fatally logs when username cannot be found", func() {
		httpClient.responseBody = []string{
			metaResponseInfo("source-1"),
		}

		cliConn.cliCommandResult = [][]string{
			{
				capiAppsResponse(map[string]string{"source-1": "app-1"}),
			},
		}
		cliConn.cliCommandErr = nil

		cliConn.usernameErr = errors.New("some-error")

		Expect(func() {
			command.Meta(
				cliConn,
				nil,
				httpClient,
				logger,
				tableWriter,
			)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal(`Could not get username: some-error`))
	})

	It("fatally logs when CAPI response is not proper JSON", func() {
		httpClient.responseBody = []string{
			metaResponseInfo("source-1", "source-2"),
		}

		cliConn.cliCommandResult = [][]string{{"invalid"}}
		cliConn.cliCommandErr = nil

		Expect(func() {
			command.Meta(
				cliConn,
				nil,
				httpClient,
				logger,
				tableWriter,
			)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(HavePrefix(`Failed to read application information: `))
	})

	It("fatally logs when Meta fails", func() {
		httpClient.responseErr = errors.New("some-error")

		Expect(func() {
			command.Meta(
				cliConn,
				nil,
				httpClient,
				logger,
				tableWriter,
			)
		}).To(Panic())

		Expect(logger.fatalfMessage).To(Equal(`Failed to read Meta information: some-error`))
	})
})

func metaResponseInfo(sourceIDs ...string) string {
	var metaInfos []string
	metaInfos = append(metaInfos, fmt.Sprintf(`"%s": {
		  "count": "100000",
		  "expired": "85008",
		  "oldestTimestamp": "1519256863100000000",
		  "newestTimestamp": "1519256863110000000"
		}`, sourceIDs[0]))
	for _, sourceID := range sourceIDs[1:] {
		metaInfos = append(metaInfos, fmt.Sprintf(`"%s": {
		  "count": "100000",
		  "expired": "85008",
		  "oldestTimestamp": "1519256157847077020",
		  "newestTimestamp": "1519256863126668345"
		}`, sourceID))
	}
	return fmt.Sprintf(`{ "meta": { %s }}`, strings.Join(metaInfos, ","))
}

func metaResponseInfoButHigher(sourceIDs ...string) string {
	var metaInfos []string
	metaInfos = append(metaInfos, fmt.Sprintf(`"%s": {
		  "count": "100004",
		  "expired": "85009",
		  "oldestTimestamp": "1519256863100000000",
		  "newestTimestamp": "1519256863110000000"
		}`, sourceIDs[0]))
	for _, sourceID := range sourceIDs[1:] {
		metaInfos = append(metaInfos, fmt.Sprintf(`"%s": {
		  "count": "100004",
		  "expired": "85009",
		  "oldestTimestamp": "1519256157847077020",
		  "newestTimestamp": "1519256863126668345"
		}`, sourceID))
	}
	return fmt.Sprintf(`{ "meta": { %s }}`, strings.Join(metaInfos, ","))
}

func variedMetaResponseInfo(sourceIDs ...string) string {
	var metaInfos []string
	expiredBase := 85000
	countBase := 100000
	oldestTimestampBase := 1519256863100000000
	alternatingSign := -1

	for n, sourceID := range sourceIDs {
		if n%2 == 0 {
			alternatingSign *= -1
		}
		currentOffset := alternatingSign * (n + 1)
		expired := expiredBase - currentOffset
		count := countBase + currentOffset
		newestTimestamp := oldestTimestampBase + n*270000000000

		metaInfos = append(metaInfos, fmt.Sprintf(`"%s": {
		  "count": "%d",
		  "expired": "%d",
		  "oldestTimestamp": "%d",
		  "newestTimestamp": "%d"
		}`, sourceID, count, expired, oldestTimestampBase, newestTimestamp))
	}
	return fmt.Sprintf(`{ "meta": { %s }}`, strings.Join(metaInfos, ","))
}

func variedMetaResponseInfoButHigher(diffs []int, sourceIDs ...string) string {
	var metaInfos []string
	expiredBase := 85000
	countBase := 100000
	oldestTimestampBase := 1519256863100000000
	alternatingSign := -1
	for n, sourceID := range sourceIDs {
		if n%2 == 0 {
			alternatingSign *= -1
		}
		currentOffset := alternatingSign * (n + 1)
		expired := expiredBase - currentOffset
		count := countBase + currentOffset + diffs[n]
		newestTimestamp := oldestTimestampBase + n*270000000000

		metaInfos = append(metaInfos, fmt.Sprintf(`"%s": {
		  "count": "%d",
		  "expired": "%d",
		  "oldestTimestamp": "%d",
		  "newestTimestamp": "%d"
		}`, sourceID, count, expired, oldestTimestampBase, newestTimestamp))
	}
	return fmt.Sprintf(`{ "meta": { %s }}`, strings.Join(metaInfos, ","))
}

func capiAppsResponse(apps map[string]string) string {
	var resources []string
	for appID, appName := range apps {
		resources = append(resources, fmt.Sprintf(`{"guid": "%s", "name": "%s"}`, appID, appName))
	}
	return fmt.Sprintf(`{ "resources": [%s] }`, strings.Join(resources, ","))
}

func capiServiceInstancesResponse(services map[string]string) string {
	var resources []string
	for serviceID, serviceName := range services {
		resource := fmt.Sprintf(`{"metadata": {"guid": "%s"}, "entity": {"name": "%s"}}`, serviceID, serviceName)
		resources = append(resources, resource)
	}
	return fmt.Sprintf(`{ "resources": [%s] }`, strings.Join(resources, ","))
}
