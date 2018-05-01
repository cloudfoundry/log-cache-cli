package command_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/log-cache-cli/pkg/command"
)

var _ = Describe("Config", func() {
	It("Loads config from file", func() {
		expectedConfig := command.Config{
			Addr: "test-file-addr",
		}
		cleanup := writeTmpConfig(expectedConfig)
		defer cleanup()
		cleanup = patchEnv("LOG_CACHE_ADDR", "")
		defer cleanup()
		cleanup = patchEnv("LOG_CACHE_SKIP_AUTH", "")
		defer cleanup()

		c, err := command.BuildConfig()

		Expect(err).ToNot(HaveOccurred())
		Expect(c).To(Equal(expectedConfig))
	})

	It("Loads config from env", func() {
		expectedConfig := command.Config{
			Addr: "test-env-addr",
		}
		cleanup := patchEnv("LOG_CACHE_ADDR", expectedConfig.Addr)
		defer cleanup()
		cleanup = patchEnv("LOG_CACHE_SKIP_AUTH", "")
		defer cleanup()

		c, err := command.BuildConfig()

		Expect(err).ToNot(HaveOccurred())
		Expect(c).To(Equal(expectedConfig))
	})

	It("Merges config file and env", func() {
		expectedConfig := command.Config{
			Addr:     "test-addr",
			SkipAuth: true,
		}
		fileConfig := command.Config{
			Addr: "test-addr",
		}
		cleanup := writeTmpConfig(fileConfig)
		defer cleanup()
		cleanup = patchEnv("LOG_CACHE_ADDR", "")
		defer cleanup()
		cleanup = patchEnv("LOG_CACHE_SKIP_AUTH", "true")
		defer cleanup()

		c, err := command.BuildConfig()

		Expect(err).ToNot(HaveOccurred())
		Expect(c).To(Equal(expectedConfig))
	})

	It("Prefers env over config file", func() {
		fileConfig := command.Config{
			Addr:     "some-bad-value",
			SkipAuth: false,
		}
		expectedConfig := command.Config{
			Addr:     "test-env-addr",
			SkipAuth: true,
		}
		cleanup := writeTmpConfig(fileConfig)
		defer cleanup()
		cleanup = patchEnv("LOG_CACHE_ADDR", expectedConfig.Addr)
		defer cleanup()
		cleanup = patchEnv("LOG_CACHE_SKIP_AUTH", "true")
		defer cleanup()

		c, err := command.BuildConfig()

		Expect(err).ToNot(HaveOccurred())
		Expect(c).To(Equal(expectedConfig))
	})

	It("returns an error when home dir is not valid", func() {
		cleanup := patchEnv("HOME", "/does/not/exist")
		defer cleanup()

		_, err := command.BuildConfig()

		Expect(err).To(HaveOccurred())
	})

	It("returns an error when config file is not valid yaml", func() {
		cleanup := writeInvalidTmpConfig()
		defer cleanup()

		_, err := command.BuildConfig()

		Expect(err).To(HaveOccurred())
	})
})
