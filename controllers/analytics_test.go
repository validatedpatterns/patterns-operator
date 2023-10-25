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

var _ = Describe("getDeviceHash", func() {
	var (
		pattern *api.Pattern
	)

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
	var (
		pattern *api.Pattern
	)

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
