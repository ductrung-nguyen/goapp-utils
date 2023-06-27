package utils_test

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestUtils(t *testing.T) {
	RegisterFailHandler(Fail)
	suiteConfig, repoterConfig := GinkgoConfiguration()
	suiteConfig.PollProgressAfter = 1 * time.Second
	repoterConfig.FullTrace = true
	RunSpecs(t, "Utils Test Suite")
}
