package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLogCacheCli(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "LogCacheCli Suite")
}
