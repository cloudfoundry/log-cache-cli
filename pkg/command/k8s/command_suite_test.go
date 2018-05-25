package k8s_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"code.cloudfoundry.org/log-cache-cli/pkg/command/k8s"
	homedir "github.com/mitchellh/go-homedir"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	yaml "gopkg.in/yaml.v2"
)

func TestCommand(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Command Suite")
}

func init() {
	homedir.DisableCache = true
}

func patchEnv(key, value string) func() {
	orig := os.Getenv(key)
	err := os.Setenv(key, value)
	Expect(err).ToNot(HaveOccurred())

	return func() {
		err := os.Setenv(key, orig)
		Expect(err).ToNot(HaveOccurred())
	}
}

func writeTmpConfig(c k8s.Config) func() {
	dir, err := ioutil.TempDir("", "")
	Expect(err).ToNot(HaveOccurred())

	f, err := os.OpenFile(filepath.Join(dir, ".lc.yml"), os.O_RDWR|os.O_CREATE, 0755)
	Expect(err).ToNot(HaveOccurred())
	defer f.Close()
	enc := yaml.NewEncoder(f)
	err = enc.Encode(c)
	Expect(err).ToNot(HaveOccurred())

	origHome := os.Getenv("HOME")
	err = os.Setenv("HOME", dir)
	Expect(err).ToNot(HaveOccurred())

	return func() {
		err = os.Setenv("HOME", origHome)
		Expect(err).ToNot(HaveOccurred())
	}
}

func writeInvalidTmpConfig() func() {
	dir, err := ioutil.TempDir("", "")
	Expect(err).ToNot(HaveOccurred())

	f, err := os.OpenFile(filepath.Join(dir, ".lc.yml"), os.O_RDWR|os.O_CREATE, 0755)
	Expect(err).ToNot(HaveOccurred())
	defer f.Close()
	fmt.Fprint(f, "!@$^*!^!$)%@")

	origHome := os.Getenv("HOME")
	err = os.Setenv("HOME", dir)
	Expect(err).ToNot(HaveOccurred())

	return func() {
		err = os.Setenv("HOME", origHome)
		Expect(err).ToNot(HaveOccurred())
	}
}
