package cmd

import (
	"errors"

	"code.cloudfoundry.org/cli/plugin/pluginfakes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Meta", func() {
	var (
		opts   *MetaOpts
		conn   *pluginfakes.FakeCliConnection
		buffer *gbytes.Buffer
	)

	BeforeEach(func() {
		opts = &MetaOpts{}
		conn = &pluginfakes.FakeCliConnection{}
		buffer = gbytes.NewBuffer()
		writer = buffer
	})

	JustBeforeEach(func() {
		Meta(conn, opts)
	})

	It("prints out a table of meta information for the current contents of Log-Cache", func() {
		Eventually(buffer).Should(gbytes.Say(`Source\s+Source Type\s+Count\s+Expired\s+Cache Duration\s+Rate/minute`))
	})

	Context("Fails to retrieve CAPI endpoint", func() {
		BeforeEach(func() {
			conn.ApiEndpointReturns("", errors.New("some-err"))
		})

		It("prints out an error message", func() {
			Expect(string(buffer.Contents())).To(Equal("Could not retrieve Log-Cache endpoint.\nError: some-err\n"))
		})
	})

	PContext("Fails to retrieve meta information from Log-Cache", func() {
		It("prints out an error message", func() {
			Expect(string(buffer.Contents())).To(Equal("Could not retrieve meta information from Log-Cache.\nError: some-err\n"))
		})
	})
})
