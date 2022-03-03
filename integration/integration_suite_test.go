package integration_test

import (
	"fmt"
	"math/rand"
	"os/exec"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var (
	path string
	org  string
	r    = rand.New(rand.NewSource(time.Now().UnixNano()))
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

var _ = BeforeSuite(func() {
	path = buildPlugin()
	installPlugin()
	org = createAndTargetOrg()
	createAndTargetSpace()
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
	deleteOrg()
})

// Build the plugin
func buildPlugin() string {
	p, err := gexec.Build("code.cloudfoundry.org/log-cache-cli/v4")
	Expect(err).To(BeNil())

	return p
}

// Install the plugin (by force)
func installPlugin() {
	cmd := fmt.Sprintf("cf install-plugin %s -f", path)
	session, err := gexec.Start(exec.Command(cmd), GinkgoWriter, GinkgoWriter)
	Expect(err).To(BeNil())
	Expect(session).Should(gexec.Exit(0))
}

// Create and target a random org
func createAndTargetOrg() string {
	org := fmt.Sprintf("TEST-ORG-%d", r.Intn(100))
	cmd := fmt.Sprintf("cf create-org %s", org)
	session, err := gexec.Start(exec.Command(cmd), GinkgoWriter, GinkgoWriter)
	Expect(err).To(BeNil())
	Expect(session).Should(gexec.Exit(0))

	cmd = fmt.Sprintf("cf target -o %s", org)
	session, err = gexec.Start(exec.Command(cmd), GinkgoWriter, GinkgoWriter)
	Expect(err).To(BeNil())
	Expect(session).Should(gexec.Exit(0))

	return org
}

// Delete the org (by force)
func deleteOrg() {
	cmd := fmt.Sprintf("cf delete-org %s -f", org)
	session, err := gexec.Start(exec.Command(cmd), GinkgoWriter, GinkgoWriter)
	Expect(err).To(BeNil())
	Expect(session).Should(gexec.Exit(0))
}

// Create and target a random space
func createAndTargetSpace() {
	space := fmt.Sprintf("TEST-SPACE-%d", r.Intn(100))
	cmd := fmt.Sprintf("cf create-space %s", space)
	session, err := gexec.Start(exec.Command(cmd), GinkgoWriter, GinkgoWriter)
	Expect(err).To(BeNil())
	Expect(session).Should(gexec.Exit(0))

	cmd = fmt.Sprintf("cf target -s %s", space)
	session, err = gexec.Start(exec.Command(cmd), GinkgoWriter, GinkgoWriter)
	Expect(err).To(BeNil())
	Expect(session).Should(gexec.Exit(0))
}
