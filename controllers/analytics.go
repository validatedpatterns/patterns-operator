package controllers

import (
	_ "embed"
	"encoding/base64"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/segmentio/analytics-go/v3"

	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
	"github.com/hybrid-cloud-patterns/patterns-operator/version"
)

type VpAnalyticsInterface interface {
	SendPatternInstallationInfo(p *api.Pattern)
	SendPatternUpdateInfo(p *api.Pattern)
}

//go:embed apikey.txt
var api_key string

const (
	// UpdateEvent is the name of the update event
	UpdateEvent = "Update"

	// RefreshIntervalMinutes is the minimum time between updates (4h)
	RefreshIntervalMinutes float64 = 240
)

type VpAnalytics struct {
	apiKey          string
	logger          logr.Logger
	lastUpdate      time.Time
	sentInstallInfo bool
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

// This called at the beginning of the reconciliation loop and only once
func (v *VpAnalytics) SendPatternInstallationInfo(p *api.Pattern) {
	// If we already sent this event skip it
	if v.apiKey == "" || v.sentInstallInfo || p.Status.AnalyticsSent {
		return
	}

	info := map[string]any{}
	properties := analytics.NewProperties()
	for k, v := range info {
		properties.Set(k, v)
	}
	properties.Set("pattern", p.Name)
	baseGitRepo, _ := extractRepositoryName(p.Spec.GitConfig.TargetRepo)

	parts := strings.Split(p.Status.ClusterDomain, ".")
	simpleDomain := strings.Join(parts[len(parts)-2:], ".")
	client := analytics.New(v.apiKey)
	defer client.Close()
	err := client.Enqueue(analytics.Identify{
		UserId: getNewUUID(p),
		Traits: analytics.NewTraits().
			SetName("VP User").
			Set("platform", p.Status.ClusterPlatform).
			Set("ocpversion", p.Status.ClusterVersion).
			Set("domain", simpleDomain).
			Set("operatorversion", version.Version).
			Set("repobasename", baseGitRepo).
			Set("pattern", p.Name),
	})
	if err != nil {
		v.logger.Info("Sending Installation info failed:", "info", err)
		return
	}
	v.logger.Info("Sent Identify Event")
	p.Status.AnalyticsSent = true
	v.sentInstallInfo = true
}

func (v *VpAnalytics) SendPatternUpdateInfo(p *api.Pattern) {
	if v.apiKey == "" {
		return
	}

	if !hasIntervalPassed(v.lastUpdate) {
		return
	}

	baseGitRepo, _ := extractRepositoryName(p.Spec.GitConfig.TargetRepo)

	parts := strings.Split(p.Status.ClusterDomain, ".")
	simpleDomain := strings.Join(parts[len(parts)-2:], ".")

	client := analytics.New(v.apiKey)
	defer client.Close()
	err := client.Enqueue(analytics.Track{
		UserId: getNewUUID(p),
		Event:  UpdateEvent,
		Properties: analytics.NewProperties().
			Set("platform", p.Status.ClusterPlatform).
			Set("ocpversion", p.Status.ClusterVersion).
			Set("domain", simpleDomain).
			Set("operatorversion", version.Version).
			Set("repobasename", baseGitRepo).
			Set("pattern", p.Name),
	})
	if err != nil {
		v.logger.Info("Sending update info failed:", "info", err)
		return
	}
	v.logger.Info("Sent an update Info event")
	v.lastUpdate = time.Now()
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

	if disabled {
		logger.Info("Analytics explicitly disabled")
		v.apiKey = ""
		return &v
	}

	v.logger = logger
	s := decodeApiKey(api_key)
	if s != "" {
		logger.Info("Analytics enabled")
		v.apiKey = s
		v.sentInstallInfo = false
		v.lastUpdate = time.Date(1980, time.Month(1), 1, 0, 0, 0, 0, time.UTC)
	} else {
		logger.Info("Analytics enabled but no API key present")
		v.apiKey = ""
	}

	return &v
}
