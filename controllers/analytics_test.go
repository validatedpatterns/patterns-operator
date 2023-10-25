package controllers

import (
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

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
		// Set the last update time to five minutes ago
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
		vpAnalytics = AnalyticsInit(true, logr.Discard())
		vpAnalytics.apiKey = "123"
		pattern = &api.Pattern{}
	})

	Context("when apiKey is empty SendPatternStartEventInfo()", func() {
		It("should return false and not send the event", func() {
			vpAnalytics.apiKey = ""
			result := vpAnalytics.SendPatternStartEventInfo(pattern)
			Expect(result).To(BeFalse())
		})
	})

	Context("when the start event has not already been sent", func() {
		It("should return true and  send the event", func() {
			vpAnalytics.sentStartEvent = false
			result := vpAnalytics.SendPatternStartEventInfo(pattern)
			Expect(result).To(BeTrue())
		})
	})

	Context("when the start event has already been sent", func() {
		It("should return false and not send the event", func() {
			vpAnalytics.sentStartEvent = true
			result := vpAnalytics.SendPatternStartEventInfo(pattern)
			Expect(result).To(BeFalse())
		})
	})

	Context("when apiKey is empty SendPatternEndEventInfo", func() {
		It("should return false and not send the event", func() {
			vpAnalytics.apiKey = ""
			result := vpAnalytics.SendPatternEndEventInfo(pattern)
			Expect(result).To(BeFalse())
		})
	})

	Context("when the the interval has not passed", func() {
		It("should return false and not send the event", func() {
			vpAnalytics.lastEndEvent = time.Now().Add(-time.Minute * 5)
			result := vpAnalytics.SendPatternEndEventInfo(pattern)
			Expect(result).To(BeFalse())
		})
	})

	Context("when the the interval has passed", func() {
		It("should return true and send the event", func() {
			vpAnalytics.lastEndEvent = time.Date(1980, time.Month(1), 1, 0, 0, 0, 0, time.UTC)
			result := vpAnalytics.SendPatternEndEventInfo(pattern)
			Expect(result).To(BeTrue())
		})
	})

})

var _ = Describe("getDeviceHash", func() {
	var pattern *api.Pattern

	BeforeEach(func() {
		pattern = &api.Pattern{
			Status: api.PatternStatus{
				ClusterDomain: "example.com",
			},
		}
	})

	Context("with valid input", func() {
		It("should return the expected hash", func() {
			expectedHash := "a379a6f6eeafb9a55e378c118034e2751e682fab9f2d30ab13d2125586ce1947"
			actualHash := getDeviceHash(pattern)
			Expect(actualHash).To(Equal(expectedHash))
		})
	})

	Context("with empty input", func() {
		It("should return a default hash", func() {
			pattern.Status.ClusterDomain = ""
			expectedHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
			actualHash := getDeviceHash(pattern)
			Expect(actualHash).To(Equal(expectedHash))
		})
	})
})

var _ = Describe("getSimpleDomain", func() {
	var pattern *api.Pattern

	BeforeEach(func() {
		pattern = &api.Pattern{
			Status: api.PatternStatus{
				ClusterDomain: "example.com",
			},
		}
	})

	Context("with valid input", func() {
		It("should return the simple domain", func() {
			expectedSimpleDomain := "example.com"
			actualSimpleDomain := getSimpleDomain(pattern)
			Expect(actualSimpleDomain).To(Equal(expectedSimpleDomain))
		})
	})

	Context("with subdomains", func() {
		It("should return the simple domain for subdomains", func() {
			pattern.Status.ClusterDomain = "subdomain.example.com"
			expectedSimpleDomain := "example.com"
			actualSimpleDomain := getSimpleDomain(pattern)
			Expect(actualSimpleDomain).To(Equal(expectedSimpleDomain))
		})
	})

	Context("with single part domain", func() {
		It("should return the input domain", func() {
			pattern.Status.ClusterDomain = "localhost"
			expectedSimpleDomain := "localhost"
			actualSimpleDomain := getSimpleDomain(pattern)
			Expect(actualSimpleDomain).To(Equal(expectedSimpleDomain))
		})
	})

	Context("with empty input", func() {
		It("should return an empty string", func() {
			pattern.Status.ClusterDomain = ""
			expectedSimpleDomain := ""
			actualSimpleDomain := getSimpleDomain(pattern)
			Expect(actualSimpleDomain).To(Equal(expectedSimpleDomain))
		})
	})
})

var _ = Describe("getNewUUID", func() {
	var pattern *api.Pattern

	BeforeEach(func() {
		pattern = &api.Pattern{
			Spec: api.PatternSpec{
				AnalyticsUUID: "user-specified-uuid",
			},
			Status: api.PatternStatus{
				AnalyticsUUID: "status-saved-uuid",
			},
		}
	})

	Context("when user specifies an AnalyticsUUID", func() {
		It("should return the user-specified UUID", func() {
			expectedUUID := "user-specified-uuid"
			actualUUID := getNewUUID(pattern)
			Expect(actualUUID).To(Equal(expectedUUID))
		})
	})

	Context("when user doesn't specify an AnalyticsUUID and status contains a saved UUID", func() {
		It("should return the status-saved UUID", func() {
			pattern.Spec.AnalyticsUUID = ""
			expectedUUID := "status-saved-uuid"
			actualUUID := getNewUUID(pattern)
			Expect(actualUUID).To(Equal(expectedUUID))
		})
	})

	Context("when both user-specified and status-saved UUIDs are empty", func() {
		It("should generate a new UUID and save it in the status", func() {
			pattern.Spec.AnalyticsUUID = ""
			pattern.Status.AnalyticsUUID = ""
			actualUUID := getNewUUID(pattern)
			Expect(actualUUID).NotTo(BeEmpty())
			Expect(pattern.Status.AnalyticsUUID).To(Equal(actualUUID))
		})
	})
})
