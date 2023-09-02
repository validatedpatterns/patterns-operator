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
	vpUUID          string
	apiKey          string
	client          analytics.Client
	logger          logr.Logger
	sentInstallInfo bool
	lastUpdate      time.Time
}

// This called at the beginning of the reconciliation loop and only once
func (v *VpAnalytics) SendPatternInstallationInfo(p *api.Pattern) {
	// If we already sent this event skip it
	if v.client == nil || v.sentInstallInfo {
		return
	}

	info := map[string]interface{}{}
	properties := analytics.NewProperties()
	for k, v := range info {
		properties.Set(k, v)
	}
	properties.Set("pattern", p.Name)
	baseGitRepo, _ := extractRepositoryName(p.Spec.GitConfig.TargetRepo)
	v.client.Enqueue(analytics.Identify{
		UserId: v.vpUUID,
		Traits: analytics.NewTraits().
			SetName("VP User").
			Set("platform", p.Status.ClusterPlatform).
			Set("ocpversion", p.Status.ClusterVersion).
			Set("operatorversion", version.Version).
			Set("repobasename", baseGitRepo).
			Set("pattern", p.Name),
	})
	v.sentInstallInfo = true
}

func (v *VpAnalytics) SendPatternUpdateInfo(p *api.Pattern) {
	if v.client == nil {
		return
	}

	if !hasIntervalPassed(v.lastUpdate) {
		return
	}
	v.logger.Info("Sending an update Info event")
	base, _ := extractRepositoryName(p.Spec.GitConfig.TargetRepo)
	v.client.Enqueue(analytics.Track{
		UserId: v.vpUUID,
		Event:  UpdateEvent,
		Properties: analytics.NewProperties().
			Set("pattern", p.Name).
			Set("ocpversion", p.Status.ClusterVersion).
			Set("gitbase", base),
	})
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

func AnalyticsInit(vp_uuid string, disabled bool, logger logr.Logger) *VpAnalytics {
	v := VpAnalytics{}

	if disabled {
		logger.Info("Analytics explicitely disabled")
		v.client = nil
		v.apiKey = ""
		return &v
	}

	if vp_uuid == "" {
		v.vpUUID = uuid.New().String()
	} else {
		v.vpUUID = vp_uuid
	}
	v.logger = logger
	s := decodeApiKey(api_key)
	if s != "" {
		logger.Info("Analytics enabled")
		v.apiKey = s
		v.client = analytics.New(s)
		v.sentInstallInfo = false
		v.lastUpdate = time.Date(1980, time.Month(1), 1, 0, 0, 0, 0, time.UTC)
	} else {
		logger.Info("Analytics enabled but no API key present")
		v.client = nil
		v.apiKey = ""
	}

	return &v
}
