package command_test

import (
	"errors"
	"fmt"
	"log"

	"code.cloudfoundry.org/cli/plugin"
	"code.cloudfoundry.org/cli/plugin/pluginfakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"code.cloudfoundry.org/log-cache-cli/v4/internal/command"
	"code.cloudfoundry.org/log-cache-cli/v4/internal/command/commandfakes"
)

var _ = Describe("command", func() {
	var (
		testCmd *command.Command

		runCalls int
	)

	BeforeEach(func() {
		testCmd = &command.Command{
			Name:     "test",
			HelpText: "some help text.",
			UsageDetails: plugin.Usage{
				Usage: "test [options]",
				Options: map[string]string{
					"testflag":  "does nothing",
					"otherflag": "also does nothing",
				},
			},
			Run: func(cmd *command.Command, c command.LogCacheClient, args []string) error {
				runCalls = 1
				log.Println("testing command")
				return nil
			},
		}
		runCalls = 0
		command.Add(&command.Command{
			Name:     "other",
			HelpText: "more help text.",
			UsageDetails: plugin.Usage{
				Usage: "other",
			},
		})
	})

	JustBeforeEach(func() {
		command.Add(testCmd)
	})

	Describe("Run", func() {
		var (
			fcc *pluginfakes.FakeCliConnection

			args []string

			buf    *gbytes.Buffer
			runErr error
		)

		BeforeEach(func() {
			fcc = new(pluginfakes.FakeCliConnection)
			fcc.HasAPIEndpointReturns(true, nil)
			fcc.ApiEndpointReturns("https://api.some-system.com", nil)

			args = []string{"test"}

			buf = gbytes.NewBuffer()
			log.SetFlags(0)
			log.SetOutput(buf)
			log.SetPrefix("")

			command.DefaultClient = new(commandfakes.FakeLogCacheClient)
		})

		JustBeforeEach(func() {
			runErr = command.Run(fcc, args)
		})

		It("runs the correct command", func() {
			Expect(runCalls).To(Equal(1))
			Expect(buf).To(gbytes.Say("testing command"))
		})

		Context("when the flag set fails to parse", func() {
			BeforeEach(func() {
				args = append(args, "--testflag")
			})

			It("returns an error", func() {
				Expect(runErr).To(MatchError("incorrect usage: unknown flag: --testflag"))
			})

			It("does not run the command", func() {
				Expect(runCalls).To(Equal(0))
			})

			It("does not output anthing", func() {
				Expect(buf.Contents()).To(BeEmpty())
			})
		})

		Context("too many positional args", func() {
			BeforeEach(func() {
				args = append(args, "testarg")
			})

			It("returns an error", func() {
				Expect(runErr).To(MatchError("incorrect usage: accepts at most 0 arg(s), received 1"))
			})

			It("does not run the command", func() {
				Expect(runCalls).To(Equal(0))
			})

			It("does not output anthing", func() {
				Expect(buf.Contents()).To(BeEmpty())
			})
		})

		Context("not enough positional args", func() {
			BeforeEach(func() {
				testCmd.PositionalArgs = 2
				args = append(args, "testarg")
			})

			It("returns an error", func() {
				Expect(runErr).To(MatchError("incorrect usage: requires at least 2 arg(s), only received 1"))
			})

			It("does not run the command", func() {
				Expect(runCalls).To(Equal(0))
			})

			It("does not output anthing", func() {
				Expect(buf.Contents()).To(BeEmpty())
			})
		})

		Context("when conn.HasAPIEndpoint returns an error", func() {
			BeforeEach(func() {
				fcc.HasAPIEndpointReturns(false, fmt.Errorf("some-error"))
			})

			It("returns an error", func() {
				Expect(runErr).To(MatchError("no API endpoint set: some-error"))
			})

			It("does not print anything to stdout", func() {
				Expect(buf.Contents()).To(BeEmpty())
			})
		})

		Context("when conn.HasAPIEndpoint returns false", func() {
			BeforeEach(func() {
				fcc.HasAPIEndpointReturns(false, nil)
			})

			It("returns an error", func() {
				Expect(runErr).To(MatchError("no API endpoint set"))
			})

			It("does not print anything to stdout", func() {
				Expect(buf.Contents()).To(BeEmpty())
			})
		})

		Context("when conn.ApiEndpoint returns an error", func() {
			BeforeEach(func() {
				fcc.ApiEndpointReturns("", errors.New("some-error"))
			})

			It("returns an error", func() {
				Expect(runErr).To(MatchError("could not retrieve API endpoint: some-error"))
			})

			It("does not print anything to stdout", func() {
				Expect(buf.Contents()).To(BeEmpty())
			})
		})
	})

	Describe("Commands", func() {
		It("returns all the added commands as a plugin.Command slice", func() {
			cmds := command.Commands()
			Expect(cmds).To(HaveLen(2))
			Expect(cmds).To(ContainElement(plugin.Command{
				Name:     "test",
				HelpText: "some help text.",
				UsageDetails: plugin.Usage{
					Usage: "test [options]",
					Options: map[string]string{
						"testflag":  "does nothing",
						"otherflag": "also does nothing",
					},
				},
			}))
			Expect(cmds).To(ContainElement(plugin.Command{
				Name:     "other",
				HelpText: "more help text.",
				UsageDetails: plugin.Usage{
					Usage: "other",
				},
			}))
		})
	})
})
