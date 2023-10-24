package controllers

import (
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/log"

	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
)

var _ = Describe("hasIntervalPassed", func() {
	It("should return true when interval has passed", func() {
		// Set the last update time to a point in the past
		lastUpdate := time.Now().Add(-time.Minute * time.Duration(RefreshIntervalMinutes+5))
		result := hasIntervalPassed(lastUpdate)
		Expect(result).To(BeTrue())
	})

	It("should return false when interval has not passed", func() {
		// Set the last update time to yesterday
		lastUpdate := time.Now().Add(-time.Minute * 5)
		result := hasIntervalPassed(lastUpdate)
		Expect(result).To(BeFalse())
	})

	It("should return true when last update time is zero", func() {
		// Set the last update time to zero (the beginning of the epoch)
		lastUpdate := time.Time{}
		result := hasIntervalPassed(lastUpdate)
		Expect(result).To(BeTrue())
	})

	It("should return false when current time is zero", func() {
		// Set the current time to zero (the beginning of the epoch)
		currentTime := time.Time{}
		lastUpdate := time.Now()
		result := hasIntervalPassedWithCurrentTime(lastUpdate, currentTime)
		Expect(result).To(BeFalse())
	})
})

func hasIntervalPassedWithCurrentTime(lastUpdate, currentTime time.Time) bool {
	return currentTime.Sub(lastUpdate).Minutes() >= RefreshIntervalMinutes
}

var _ = Describe("decodeApiKey", func() {
	It("should decode a valid base64 encoded key", func() {
		base64Key := "SGVsbG8gd29ybGQ=" // "Hello world" encoded
		result := decodeApiKey(base64Key)
		Expect(result).To(Equal("Hello world"))
	})

	It("should return an empty string for an invalid base64 encoded key", func() {
		invalidBase64Key := "InvalidKey@123"
		result := decodeApiKey(invalidBase64Key)
		Expect(result).To(BeEmpty())
	})

	It("should return an empty string for an empty input", func() {
		emptyKey := ""
		result := decodeApiKey(emptyKey)
		Expect(result).To(BeEmpty())
	})
})

// The VpAnalytics tests are somewhat scarce due to the fact that we do not want to send
// spurious results around (i.e. disabled is true during unit testing)
var _ = Describe("VpAnalytics", func() {
	var (
		vpAnalytics *VpAnalytics
		pattern     *api.Pattern
	)

	BeforeEach(func() {
		vpAnalytics = AnalyticsInit(true, logr.New(log.NullLogSink{}))
		pattern = &api.Pattern{}
	})

	It("should not send pattern installation info as disabled is true", func() {
		vpAnalytics.SendPatternInstallationInfo(pattern)

		Expect(pattern.Status.AnalyticsSent).To(BeFalse())
	})

	It("should not send pattern update info as disabled is true", func() {
		vpAnalytics.SendPatternStartEventInfo(pattern)

		Expect(pattern.Status.AnalyticsSent).To(BeFalse())
	})
})
