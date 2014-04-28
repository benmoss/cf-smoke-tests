package smoke

import (
	"fmt"
	"strconv"
	"time"
	"regexp"
	"strings"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	. "github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf-experimental/cf-test-helpers/cf"
	"github.com/pivotal-cf-experimental/cf-test-helpers/generator"
	"github.com/pivotal-cf-experimental/cf-test-helpers/runner"
)

var _ = Describe("Runtime:", func() {
	var testConfig = GetConfig()
	var appName string
	var appUrl string

	BeforeEach(func() {
		appName = generator.RandomName()
		appUrl = "http://" + appName + "." + testConfig.AppsDomain
	})

	AfterEach(func() {
		Eventually(cf.Cf("delete", appName, "-f"), CF_TIMEOUT_IN_SECONDS).Should(Exit(0))
	})

	It("can be pushed, scaled and deleted", func() {
		Eventually(cf.Cf("push", appName, "-p", SIMPLE_RUBY_APP_BITS_PATH), CF_PUSH_TIMEOUT_IN_SECONDS).Should(Exit(0))

		Expect(runner.Curl(appUrl).Wait(CF_TIMEOUT_IN_SECONDS)).To(Say("It just needed to be restarted!"))

		instances := 2
		maxAttempts := 10

		ExpectAppToScale(appName, instances)

		ExpectAllAppInstancesToStart(appName, instances, maxAttempts)

		ExpectAllAppInstancesToBeReachable(appUrl, instances, maxAttempts)

		Eventually(cf.Cf("delete", appName, "-f"), CF_TIMEOUT_IN_SECONDS).Should(Exit(0))

		appStatusSession := cf.Cf("app", appName)
		Eventually(appStatusSession, CF_TIMEOUT_IN_SECONDS).Should(Exit(1))
		Expect(appStatusSession).To(Say("not found"))

		Expect(runner.Curl(appUrl).Wait(CF_TIMEOUT_IN_SECONDS)).To(Say("404"))
	})
})

func ExpectAppToScale(appName string, instances int) {
	Eventually(cf.Cf("scale", appName, "-i", strconv.Itoa(instances)), CF_SCALE_TIMEOUT_IN_SECONDS).Should(Exit(0))
}

// Gets app status (up to maxAttempts) until all instances are up
func ExpectAllAppInstancesToStart(appName string, instances int, maxAttempts int) {
	var found bool
	expectedOutput := fmt.Sprintf("instances: %d/%d", instances, instances)
	for i := 0; i < maxAttempts; i++ {
		session := cf.Cf("app", appName)
		Eventually(session, CF_APP_STATUS_TIMEOUT_IN_SECONDS).Should(Exit(0))

		found = strings.Contains(string(session.Out.Contents()), expectedOutput)
 		if found {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	Expect(found).To(BeTrue(), fmt.Sprintf("Wanted to see \"%s\" in %d attempts, but didn't", expectedOutput, maxAttempts))
}

// Curls the appUrl (up to maxAttempts) until all instances have been seen
func ExpectAllAppInstancesToBeReachable(appUrl string, instances int, maxAttempts int) {
	var matcher = regexp.MustCompile(`"instance_index":(\d+)`)

	branchesSeen := make([]bool, instances)
	var sawAll bool
	for i := 0; i < maxAttempts; i++ {
		session := runner.Curl(appUrl)
		Eventually(session, CF_TIMEOUT_IN_SECONDS).Should(Exit(0))

		output := string(session.Out.Contents())

		matches := matcher.FindStringSubmatch(output)
		if matches == nil {
			Fail("Expected app curl output to include an instance_index; got " + output)
		}
		indexString := matches[1]
		index, err := strconv.Atoi(indexString)
		if err != nil {
			Fail("Failed to parse instance index value " + indexString)
		}
		branchesSeen[index] = true

		if allTrue(branchesSeen) {
			sawAll = true
			break
		}
	}

	Expect(sawAll).To(BeTrue(), fmt.Sprintf("Expected to hit all %d app instances in %d attempts, but didn't", instances, maxAttempts))
}

func allTrue(bools []bool) bool {
	for _, curr := range bools {
		if !curr {
			return false
		}
	}
	return true
}