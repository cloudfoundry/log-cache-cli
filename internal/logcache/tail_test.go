package logcache_test

import (
	"errors"
	"log"

	plugin_models "code.cloudfoundry.org/cli/plugin/models"
	"code.cloudfoundry.org/cli/plugin/pluginfakes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"code.cloudfoundry.org/log-cache-cli/v4/internal/command"
	"code.cloudfoundry.org/log-cache-cli/v4/internal/command/commandfakes"
)

var _ = Describe("tail", func() {
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
		fcc.GetCurrentOrgReturns(plugin_models.Organization{
			OrganizationFields: plugin_models.OrganizationFields{
				Guid: "test-org-guid",
				Name: "test-org",
			},
		}, nil)
		fcc.GetCurrentSpaceReturns(plugin_models.Space{
			SpaceFields: plugin_models.SpaceFields{
				Guid: "test-space-guid",
				Name: "test-space",
			},
		}, nil)
		fcc.UsernameReturns("test-username", nil)
		fcc.CliCommandWithoutTerminalOutputReturns([]string{"e3cd4348-adc0-49ee-a22f-749539a4ac45"}, nil)

		args = []string{"test-app-name"}

		buf = gbytes.NewBuffer()
		log.SetFlags(0)
		log.SetOutput(buf)
		log.SetPrefix("")

		command.DefaultClient = new(commandfakes.FakeLogCacheClient)
	})

	JustBeforeEach(func() {
		runErr = command.Run(fcc, append([]string{"tail"}, args...))
	})

	It("returns nil", func() {
		Expect(runErr).To(BeNil())
	})

	It("outputs headers", func() {
		Expect(buf).To(gbytes.Say(`Retrieving envelopes for app "test-app-name" in org "test-org" / space "test-space" as "test-username"...`))
	})

	Context("when getting the username from the cf CLI fails", func() {
		BeforeEach(func() {
			fcc.UsernameReturns("", errors.New("some-error"))
		})

		It("returns an error", func() {
			Expect(runErr).To(MatchError("could not retrieve username: some-error"))
		})

		It("does not print anything to stdout", func() {
			Expect(buf.Contents()).To(BeEmpty())
		})
	})

	Context("when getting the current org from the cf CLI fails", func() {
		BeforeEach(func() {
			fcc.GetCurrentOrgReturns(plugin_models.Organization{}, errors.New("error fetching org"))
		})

		It("returns an error", func() {
			Expect(runErr).To(MatchError("could not retrieve current org: error fetching org"))
		})

		It("does not print anything to stdout", func() {
			Expect(buf.Contents()).To(BeEmpty())
		})
	})

	Context("when getting the current space from the cf CLI fails", func() {
		BeforeEach(func() {
			fcc.GetCurrentSpaceReturns(plugin_models.Space{}, errors.New("error fetching space"))
		})

		It("returns an error", func() {
			Expect(runErr).To(MatchError("could not retrieve current space: error fetching space"))
		})

		It("does not print anything to stdout", func() {
			Expect(buf.Contents()).To(BeEmpty())
		})
	})
})
