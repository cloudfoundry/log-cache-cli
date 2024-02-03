package logcache_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestLogCache(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "log-cache Plugin Suite")
}
