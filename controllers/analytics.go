package controllers

import (
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/segmentio/analytics-go/v3"

	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
	"github.com/hybrid-cloud-patterns/patterns-operator/version"
)

type VpAnalyticsInterface interface {
	SendPatternInstallationInfo(p *api.Pattern) bool
	SendPatternStartEventInfo(p *api.Pattern) bool
	SendPatternEndEventInfo(p *api.Pattern) bool
}

//go:embed apikey.txt
var api_key string

const (
	// UpdateEvent is the name of the update event
	PatternStartEvent   = "Pattern started"
	PatternEndEvent     = "Pattern completed"
	PatternRefreshEvent = "Pattern refreshed"

	// RefreshIntervalMinutes is the minimum time between updates (4h)
	RefreshIntervalMinutes float64 = 240

	// AnalyticsSent is an int bit-field that stores which info has already been sent
	AnalyticsSentIdentify = 0x0
	AnalyticsSentStart    = 0x1
	AnalyticsSentEnd      = 0x2
	AnalyticsSentRefresh  = 0x3

	MinSubDomainParts = 3
)

type VpAnalytics struct {
	apiKey          string
	logger          logr.Logger
	lastEndEvent    time.Time
	sentInstallInfo bool
	sentStartEvent  bool
}

func getNewUUID(p *api.Pattern) string {
	var newuuid string
	// If the user specified a UUID that is what we will use
	if p.Spec.AnalyticsUUID != "" {
		newuuid = p.Spec.AnalyticsUUID
	} else {
		// If we saved a UUID in the status let's reuse that
		// otherwise use a generated one
		if p.Status.AnalyticsUUID != "" {
			newuuid = p.Status.AnalyticsUUID
		} else {
			newuuid = uuid.New().String()
			p.Status.AnalyticsUUID = newuuid
		}
	}
	return newuuid
}

func getSimpleDomain(p *api.Pattern) string {
	parts := strings.Split(p.Status.ClusterDomain, ".")
	if len(parts) <= MinSubDomainParts {
		return p.Status.ClusterDomain
	}
	simpleDomain := strings.Join(parts[len(parts)-3:], ".")
	return simpleDomain
}

func getDeviceHash(p *api.Pattern) string {
	d := p.Status.ClusterDomain
	h := sha256.New()
	h.Write([]byte(d))
	hash := hex.EncodeToString(h.Sum(nil))
	return hash
}

func getBaseGitRepo(p *api.Pattern) string {
	s, _ := extractRepositoryName(p.Spec.GitConfig.TargetRepo)
	return s
}

func getAnalyticsContext(p *api.Pattern) *analytics.Context {
	// If there was an error the counts are both set to zero
	apps, appsets, _ := countVPApplications(p)

	ctx := &analytics.Context{
		Extra: map[string]any{
			"Pattern":         p.Name,
			"Domain":          getSimpleDomain(p),
			"OperatorVersion": version.Version,
			"RepoBaseName":    getBaseGitRepo(p),
			"OCPVersion":      p.Status.ClusterVersion,
			"Platform":        p.Status.ClusterPlatform,
			"DeviceHash":      getDeviceHash(p),
			"AppCount":        strconv.Itoa(apps),
			"AppSetCount":     strconv.Itoa(appsets),
		},
		App: analytics.AppInfo{
			Version: version.Version,
		},
		OS: analytics.OSInfo{
			Name:    "OpenShift",
			Version: p.Status.ClusterVersion,
		},
		Device: analytics.DeviceInfo{
			Id:   getDeviceHash(p),
			Type: p.Status.ClusterPlatform,
		},
	}
	return ctx
}

func getAnalyticsProperties(p *api.Pattern) analytics.Properties {
	properties := analytics.NewProperties().
		Set("platform", p.Status.ClusterPlatform).
		Set("ocpversion", p.Status.ClusterVersion).
		Set("domain", getSimpleDomain(p)).
		Set("operatorversion", version.Version).
		Set("repobasename", getBaseGitRepo(p)).
		Set("pattern", p.Name)
	return properties
}

func getAnalyticsTrack(p *api.Pattern, event string) analytics.Track {
	return analytics.Track{
		UserId:     getNewUUID(p),
		Event:      event,
		Context:    getAnalyticsContext(p),
		Properties: getAnalyticsProperties(p),
	}
}

func setBit(n int, pos uint) int {
	n |= (1 << pos)
	return n
}

func hasBit(n int, pos uint) bool {
	val := n & (1 << pos)
	return (val > 0)
}

// This called at the beginning of the reconciliation loop and only once
// returns true if the status object in the crd should be updated
func (v *VpAnalytics) SendPatternInstallationInfo(p *api.Pattern) bool {
	// If we already sent this event skip it
	if v.apiKey == "" || v.sentInstallInfo || hasBit(p.Status.AnalyticsSent, AnalyticsSentIdentify) {
		return false
	}

	info := map[string]any{}
	properties := analytics.NewProperties()
	for k, v := range info {
		properties.Set(k, v)
	}
	properties.Set("pattern", p.Name)
	client := analytics.New(v.apiKey)
	defer client.Close()
	id := analytics.Identify{
		UserId:  getNewUUID(p),
		Context: getAnalyticsContext(p),
		Traits: analytics.NewTraits().
			SetName("VP User").
			Set("platform", p.Status.ClusterPlatform).
			Set("ocpversion", p.Status.ClusterVersion).
			Set("domain", getSimpleDomain(p)).
			Set("operatorversion", version.Version).
			Set("repobasename", getBaseGitRepo(p)).
			Set("pattern", p.Name),
	}
	err := retryAnalytics(v.logger, 2, 1, id, client.Enqueue)
	if err != nil {
		v.logger.Info("Sending Installation info failed:", "info", err)
		return false
	}
	v.logger.Info("Sent Identify Event")
	p.Status.AnalyticsSent = setBit(p.Status.AnalyticsSent, AnalyticsSentIdentify)
	v.sentInstallInfo = true
	return true
}

// returns true if the status object in the crd should be updated
func (v *VpAnalytics) SendPatternStartEventInfo(p *api.Pattern) bool {
	if v.apiKey == "" || v.sentStartEvent || hasBit(p.Status.AnalyticsSent, AnalyticsSentStart) {
		return false
	}

	client := analytics.New(v.apiKey)
	defer client.Close()
	err := retryAnalytics(v.logger, 2, 1, getAnalyticsTrack(p, PatternStartEvent), client.Enqueue)
	if err != nil {
		v.logger.Info("Sending update info failed:", "info", err)
		return false
	}
	v.logger.Info("Sent an update Info event:", "event", PatternStartEvent)
	p.Status.AnalyticsSent = setBit(p.Status.AnalyticsSent, AnalyticsSentStart)
	v.sentStartEvent = true
	return true
}

// Sends an EndEvent the first time it is invoked. Subsequent invocations
// will send a Refresh event
// returns true if the status object in the crd should be updated
func (v *VpAnalytics) SendPatternEndEventInfo(p *api.Pattern) bool {
	if v.apiKey == "" || !hasIntervalPassed(v.lastEndEvent) {
		return false
	}
	var event string
	client := analytics.New(v.apiKey)
	defer client.Close()

	// If we already sent the end event once, let's now call it refresh event from now on
	if hasBit(p.Status.AnalyticsSent, AnalyticsSentEnd) {
		event = PatternRefreshEvent
	} else {
		event = PatternEndEvent
	}
	err := retryAnalytics(v.logger, 2, 1, getAnalyticsTrack(p, event), client.Enqueue)
	if err != nil {
		v.logger.Info("Sending update info failed:", "info", err)
		return false
	}
	v.logger.Info("Sent an update Info event:", "event", event)
	v.lastEndEvent = time.Now()

	p.Status.AnalyticsSent = setBit(p.Status.AnalyticsSent, AnalyticsSentEnd)
	if event == PatternRefreshEvent {
		p.Status.AnalyticsSent = setBit(p.Status.AnalyticsSent, AnalyticsSentRefresh)
	}
	return true
}

func retryAnalytics(logger logr.Logger, attempts int, sleep time.Duration, m analytics.Message, f func(analytics.Message) error) (err error) {
	for i := 0; i < attempts; i++ {
		err = f(m)
		if err != nil {
			logger.Info("error occurred after attempt number %d: %s. Sleep for: %s", i+1, err.Error(), sleep.String())
			time.Sleep(sleep)
			sleep *= 2
			continue
		}
		break
	}
	return err
}
func hasIntervalPassed(lastUpdate time.Time) bool {
	now := time.Now()
	return now.Sub(lastUpdate).Minutes() >= RefreshIntervalMinutes
}

func decodeApiKey(k string) string {
	// If we can't base-64 decode the key we just set it to empty and noop everything
	rawKey, err := base64.StdEncoding.DecodeString(k)
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(rawKey), "\r\n")
}

func AnalyticsInit(disabled bool, logger logr.Logger) *VpAnalytics {
	v := VpAnalytics{}
	v.logger = logger

	if disabled {
		logger.Info("Analytics explicitly disabled")
		v.apiKey = ""
		return &v
	}

	s := decodeApiKey(api_key)
	if s != "" {
		logger.Info("Analytics enabled")
		v.apiKey = s
		v.sentInstallInfo = false
		v.sentStartEvent = false
		v.lastEndEvent = time.Date(1980, time.Month(1), 1, 0, 0, 0, 0, time.UTC)
	} else {
		logger.Info("Analytics enabled but no API key present")
		v.apiKey = ""
	}

	return &v
}
