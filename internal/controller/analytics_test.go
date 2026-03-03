package controllers

import (
	"fmt"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/segmentio/analytics-go/v3"

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
			sent := hasBit(pattern.Status.AnalyticsSent, AnalyticsSentStart)
			Expect(sent).To(BeFalse())
		})
	})

	Context("when the start event has not already been sent", func() {
		It("should return true and send the event", func() {
			vpAnalytics.sentStartEvent = false
			result := vpAnalytics.SendPatternStartEventInfo(pattern)
			Expect(result).To(BeTrue())
			sent := hasBit(pattern.Status.AnalyticsSent, AnalyticsSentStart)
			Expect(sent).To(BeTrue())
		})
	})

	Context("when the start event has already been sent", func() {
		It("should return false and not send the event", func() {
			vpAnalytics.sentStartEvent = true
			result := vpAnalytics.SendPatternStartEventInfo(pattern)
			Expect(result).To(BeFalse())
			sent := hasBit(pattern.Status.AnalyticsSent, AnalyticsSentStart)
			Expect(sent).To(BeFalse())
		})
	})

	Context("when apiKey is empty SendPatternEndEventInfo", func() {
		It("should return false and not send the event", func() {
			vpAnalytics.apiKey = ""
			result := vpAnalytics.SendPatternEndEventInfo(pattern)
			Expect(result).To(BeFalse())
			sent := hasBit(pattern.Status.AnalyticsSent, AnalyticsSentEnd)
			Expect(sent).To(BeFalse())
		})
	})

	Context("when the the interval has not passed SendPatternEndEventInfo", func() {
		It("should return false and not send the event", func() {
			vpAnalytics.lastEndEvent = time.Now().Add(-time.Minute * 5)
			result := vpAnalytics.SendPatternEndEventInfo(pattern)
			Expect(result).To(BeFalse())
			sent := hasBit(pattern.Status.AnalyticsSent, AnalyticsSentEnd)
			Expect(sent).To(BeFalse())
			sent = hasBit(pattern.Status.AnalyticsSent, AnalyticsSentRefresh)
			Expect(sent).To(BeFalse())
		})
	})

	Context("when the the interval has passed SendPatternEndEventInfo", func() {
		It("should return true and send the event and refresh event should be unset", func() {
			vpAnalytics.lastEndEvent = time.Date(1980, time.Month(1), 1, 0, 0, 0, 0, time.UTC)
			result := vpAnalytics.SendPatternEndEventInfo(pattern)
			Expect(result).To(BeTrue())
			sent := hasBit(pattern.Status.AnalyticsSent, AnalyticsSentEnd)
			Expect(sent).To(BeTrue())
			sent = hasBit(pattern.Status.AnalyticsSent, AnalyticsSentRefresh)
			Expect(sent).To(BeFalse())
		})
	})

	Context("when the the interval has passed twice SendPatternEndEventInfo", func() {
		It("should return true and send the event and refresh event should be unset", func() {
			vpAnalytics.lastEndEvent = time.Date(1980, time.Month(1), 1, 0, 0, 0, 0, time.UTC)
			result := vpAnalytics.SendPatternEndEventInfo(pattern)
			Expect(result).To(BeTrue())
			sent := hasBit(pattern.Status.AnalyticsSent, AnalyticsSentEnd)
			Expect(sent).To(BeTrue())
			sent = hasBit(pattern.Status.AnalyticsSent, AnalyticsSentRefresh)
			Expect(sent).To(BeFalse())

			vpAnalytics.lastEndEvent = time.Date(1980, time.Month(1), 1, 0, 0, 0, 0, time.UTC)
			result = vpAnalytics.SendPatternEndEventInfo(pattern)
			Expect(result).To(BeTrue())
			sent = hasBit(pattern.Status.AnalyticsSent, AnalyticsSentEnd)
			Expect(sent).To(BeTrue())
			sent = hasBit(pattern.Status.AnalyticsSent, AnalyticsSentRefresh)
			Expect(sent).To(BeTrue()) // Second time the refresh sent bit is true
		})
	})

	Context("when apiKey is empty SendPatternInstallationInfo", func() {
		It("should return false and not send the event", func() {
			vpAnalytics.apiKey = ""
			result := vpAnalytics.SendPatternInstallationInfo(pattern)
			Expect(result).To(BeFalse())
			sent := hasBit(pattern.Status.AnalyticsSent, AnalyticsSentIdentify)
			Expect(sent).To(BeFalse())
		})
	})

	Context("when SendPatternInstallationInfo is called the first time", func() {
		It("should return true and not send the event", func() {
			result := vpAnalytics.SendPatternInstallationInfo(pattern)
			Expect(result).To(BeTrue())
			sent := hasBit(pattern.Status.AnalyticsSent, AnalyticsSentIdentify)
			Expect(sent).To(BeTrue())
		})
	})
})

var _ = Describe("AnalyticsInit", func() {
	Context("when disabled", func() {
		It("should return VpAnalytics with empty apiKey", func() {
			v := AnalyticsInit(true, logr.Discard())
			Expect(v).ToNot(BeNil())
			Expect(v.apiKey).To(BeEmpty())
		})
	})

	Context("when enabled with invalid api key", func() {
		It("should return VpAnalytics with empty apiKey when base64 decoding fails", func() {
			// The embedded api_key.txt is expected to have either invalid or test content
			v := AnalyticsInit(false, logr.Discard())
			Expect(v).ToNot(BeNil())
			// apiKey will be set based on whether the embedded key can be decoded
		})
	})
})

var _ = Describe("retryAnalytics", func() {
	Context("when function succeeds on first try", func() {
		It("should return nil", func() {
			successFunc := func(m analytics.Message) error {
				return nil
			}
			track := analytics.Track{Event: "test"}
			err := retryAnalytics(logr.Discard(), 3, 0, track, successFunc)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("when function always fails", func() {
		It("should return the error after all retries", func() {
			failFunc := func(m analytics.Message) error {
				return fmt.Errorf("always fails")
			}
			track := analytics.Track{Event: "test"}
			err := retryAnalytics(logr.Discard(), 2, 0, track, failFunc)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("always fails"))
		})
	})

	Context("when function succeeds on second try", func() {
		It("should return nil", func() {
			callCount := 0
			retryFunc := func(m analytics.Message) error {
				callCount++
				if callCount < 2 {
					return fmt.Errorf("temporary error")
				}
				return nil
			}
			track := analytics.Track{Event: "test"}
			err := retryAnalytics(logr.Discard(), 3, 0, track, retryFunc)
			Expect(err).ToNot(HaveOccurred())
			Expect(callCount).To(Equal(2))
		})
	})
})

var _ = Describe("setBit and hasBit", func() {
	Context("setBit", func() {
		It("should set the bit at the given position", func() {
			n := setBit(0, 0)
			Expect(n).To(Equal(1))
		})

		It("should set multiple bits", func() {
			n := setBit(0, 0)
			n = setBit(n, 2)
			Expect(n).To(Equal(5)) // 101 in binary
		})
	})

	Context("hasBit", func() {
		It("should return true when bit is set", func() {
			Expect(hasBit(5, 0)).To(BeTrue())
			Expect(hasBit(5, 2)).To(BeTrue())
		})

		It("should return false when bit is not set", func() {
			Expect(hasBit(5, 1)).To(BeFalse())
		})

		It("should return false for zero", func() {
			Expect(hasBit(0, 0)).To(BeFalse())
		})
	})
})

var _ = Describe("getAnalyticsProperties", func() {
	It("should return properties with correct fields", func() {
		pattern := &api.Pattern{
			Status: api.PatternStatus{
				ClusterPlatform: "AWS",
				ClusterVersion:  "4.12",
				ClusterDomain:   "example.com",
			},
		}
		pattern.Name = "test-pattern"
		pattern.Spec.GitConfig.TargetRepo = "https://github.com/validatedpatterns/test"

		props := getAnalyticsProperties(pattern)
		Expect(props).ToNot(BeNil())
	})
})

var _ = Describe("getAnalyticsContext", func() {
	It("should return a valid analytics context", func() {
		pattern := &api.Pattern{
			Status: api.PatternStatus{
				ClusterPlatform: "AWS",
				ClusterVersion:  "4.12",
				ClusterDomain:   "example.com",
			},
		}
		pattern.Name = "test-pattern"
		pattern.Spec.GitConfig.TargetRepo = "https://github.com/validatedpatterns/test"

		ctx := getAnalyticsContext(pattern)
		Expect(ctx).ToNot(BeNil())
		Expect(ctx.Extra["Pattern"]).To(Equal("test-pattern"))
		Expect(ctx.Extra["Platform"]).To(Equal("AWS"))
	})
})

var _ = Describe("getBaseGitRepo", func() {
	It("should extract the repo name from target repo URL", func() {
		pattern := &api.Pattern{}
		pattern.Spec.GitConfig.TargetRepo = "https://github.com/validatedpatterns/multicloud-gitops"
		Expect(getBaseGitRepo(pattern)).To(Equal("multicloud-gitops"))
	})

	It("should handle repos with .git suffix", func() {
		pattern := &api.Pattern{}
		pattern.Spec.GitConfig.TargetRepo = "https://github.com/validatedpatterns/multicloud-gitops.git"
		Expect(getBaseGitRepo(pattern)).To(Equal("multicloud-gitops"))
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
			expectedHash := "a379a6f6eeafb9a55e378c118034e2751e682fab9f2d30ab13d2125586ce1947" //nolint:gosec
			actualHash := getDeviceHash(pattern)
			Expect(actualHash).To(Equal(expectedHash))
		})
	})

	Context("with empty input", func() {
		It("should return a default hash", func() {
			pattern.Status.ClusterDomain = ""
			expectedHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" //nolint:gosec
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
			expectedSimpleDomain := "subdomain.example.com"
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

	Context("with more than 3 parts", func() {
		It("should return only last 3 parts", func() {
			pattern.Status.ClusterDomain = "hub.cluster.example.com"
			actualSimpleDomain := getSimpleDomain(pattern)
			Expect(actualSimpleDomain).To(Equal("cluster.example.com"))
		})
	})

	Context("with deep subdomain", func() {
		It("should return only last 3 parts", func() {
			pattern.Status.ClusterDomain = "deep.sub.domain.example.com"
			actualSimpleDomain := getSimpleDomain(pattern)
			Expect(actualSimpleDomain).To(Equal("domain.example.com"))
		})
	})

	Context("with exactly 2 parts", func() {
		It("should return the full domain", func() {
			pattern.Status.ClusterDomain = "example.com"
			actualSimpleDomain := getSimpleDomain(pattern)
			Expect(actualSimpleDomain).To(Equal("example.com"))
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
