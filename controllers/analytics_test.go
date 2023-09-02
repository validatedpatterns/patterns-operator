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

var _ = Describe("VpAnalytics", func() {
	var (
		vpAnalytics *VpAnalytics
	)

	BeforeEach(func() {
		vpAnalytics = AnalyticsInit("test-uuid", false, logr.New(log.NullLogSink{}))
	})

	It("should send pattern installation info", func() {
		pattern := &api.Pattern{}
		vpAnalytics.SendPatternInstallationInfo(pattern)

		Expect(vpAnalytics.sentInstallInfo).To(BeTrue())
	})

	It("should send pattern update info", func() {
		// Add your test logic here

		pattern := &api.Pattern{}
		vpAnalytics.SendPatternUpdateInfo(pattern)

		Expect(vpAnalytics.lastUpdate).To(BeTemporally("~", time.Now(), time.Second))
	})
})
