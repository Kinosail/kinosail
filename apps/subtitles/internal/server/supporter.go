package server

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"sync"
	"time"

	supporterengine "github.com/MikeO7/kinosail/packages/supporter"
)

const (
	defaultSupporterActivationURL = "https://kinosail-cloud.workers.dev/v1/supporters/activate"
	defaultSupportURL             = "https://github.com/MikeO7/kinosail/tree/main/apps/subtitles"
	supporterAppID                = "kino-subtitles"
	supporterAppName              = "Kinosail Subtitles"
	supporterAudience             = "com.kinosail.subtitles"
	patronOrderFamily             = supporterengine.FamilyPatron
	livingStandardFamily          = supporterengine.FamilyLiving
	completeFleetID               = "complete-fleet"
)

var supporterTiers = supporterengine.Tiers()

// SupporterConfig configures the optional, user-initiated supporter activation adapter.
type SupporterConfig struct {
	ActivationURL string
	SupportURL    string
	HTTPClient    *http.Client
	Now           func() time.Time
}

type supporterProgram struct {
	mu       sync.Mutex
	settings *settingsStore
	service  *supporterengine.Service
}

type (
	supporterState            = supporterengine.State
	supporterCertificate      = supporterengine.Certificate
	supporterCollectionStatus = supporterengine.Collection
	supporterBadgeCase        = supporterengine.BadgeCase
)

type supporterGrantStatus struct {
	Family          string                     `json:"family"`
	Title           string                     `json:"title"`
	Tier            string                     `json:"tier"`
	Name            string                     `json:"name"`
	SupporterID     string                     `json:"supporterId,omitempty"`
	SupportedSince  string                     `json:"supportedSince,omitempty"`
	ExpiresAt       string                     `json:"expiresAt,omitempty"`
	Rank            int                        `json:"rank"`
	ServiceMonths   int                        `json:"serviceMonths"`
	ServiceMarks    []int                      `json:"serviceMarks,omitempty"`
	Active          bool                       `json:"active"`
	Expired         bool                       `json:"expired"`
	Archived        bool                       `json:"archived"`
	Founding        bool                       `json:"founding"`
	RecognitionName string                     `json:"recognitionName,omitempty"`
	Collection      *supporterCollectionStatus `json:"collection,omitempty"`
}

type supporterAppStatus struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type supporterStatus struct {
	Tier            string               `json:"tier"`
	Name            string               `json:"name"`
	SupporterID     string               `json:"supporterId,omitempty"`
	SupportedSince  string               `json:"supportedSince,omitempty"`
	ExpiresAt       string               `json:"expiresAt,omitempty"`
	Rank            int                  `json:"rank"`
	Active          bool                 `json:"active"`
	Expired         bool                 `json:"expired"`
	Sustaining      bool                 `json:"sustaining"`
	Founding        bool                 `json:"founding"`
	RecognitionName string               `json:"recognitionName,omitempty"`
	App             supporterAppStatus   `json:"app"`
	LivingStandard  supporterGrantStatus `json:"livingStandard"`
	PatronOrder     supporterGrantStatus `json:"patronOrder"`
	BadgeCase       supporterBadgeCase   `json:"badgeCase"`
	SupportURL      string               `json:"supportUrl"`
}

func newSupporterProgram(settings *settingsStore, config SupporterConfig) *supporterProgram {
	if config.ActivationURL == "" {
		config.ActivationURL = defaultSupporterActivationURL
	}
	if config.SupportURL == "" {
		config.SupportURL = defaultSupportURL
	}
	app := supporterengine.App{ID: supporterAppID, Name: supporterAppName, Audience: supporterAudience, MasterworkName: "Perfect Sync", EmptyBadges: true, NestedApp: true, Legacy: supporterengine.LegacySubtitles}
	service, err := supporterengine.New(supporterengine.Config{App: app, ActivationURL: config.ActivationURL, SupportURL: config.SupportURL, HTTPClient: config.HTTPClient, Now: config.Now})
	if err != nil {
		service, _ = supporterengine.New(supporterengine.Config{App: app})
	}
	return &supporterProgram{settings: settings, service: service}
}

func (program *supporterProgram) activate(ctx context.Context, key, recognitionName string) error {
	input, err := supporterengine.NewActivationInput(key, recognitionName)
	if errors.Is(err, supporterengine.ErrInvalidKey) {
		return errors.New("supporter key must contain 8 to 128 letters, numbers, dashes, or underscores")
	}
	if err != nil {
		return errors.New("public recognition must contain 1 to 80 characters without padding or control marks")
	}
	program.mu.Lock()
	defer program.mu.Unlock()
	_, _, err = program.service.ActivateAndSave(ctx, program.settings.supporter(), input, program.settings.setSupporter)
	return err
}

func (program *supporterProgram) status() supporterStatus {
	status := program.service.Status(program.settings.supporter())
	return supporterStatus{
		Tier: status.Tier, Name: status.Name, SupporterID: status.SupporterID, SupportedSince: status.SupportedSince, ExpiresAt: status.ExpiresAt,
		Rank: status.Rank, Active: status.Active, Expired: status.Expired, Sustaining: status.Sustaining, Founding: status.Founding,
		RecognitionName: status.RecognitionName, App: supporterAppStatus{ID: supporterAppID, Name: supporterAppName}, LivingStandard: subtitleBadge(status.LivingStandard),
		PatronOrder: subtitleBadge(status.PatronOrder), BadgeCase: status.BadgeCase, SupportURL: status.SupportURL,
	}
}

func (program *supporterProgram) certificateForFamily(family string) (supporterCertificate, supporterGrantStatus, bool) {
	certificate, badge, ok := program.service.CertificateForFamily(program.settings.supporter(), family)
	if !ok {
		return supporterCertificate{}, supporterGrantStatus{}, false
	}
	return certificate, subtitleBadge(badge), true
}

func subtitleBadge(badge *supporterengine.Badge) supporterGrantStatus {
	if badge == nil {
		return supporterGrantStatus{}
	}
	return supporterGrantStatus{
		Family: badge.Family, Title: badge.Title, Tier: badge.Tier, Name: badge.Name, SupporterID: badge.SupporterID, SupportedSince: badge.SupportedSince,
		ExpiresAt: badge.ExpiresAt, Rank: badge.Rank, ServiceMonths: badge.ServiceMonths, ServiceMarks: badge.ServiceMarks, Active: badge.Active,
		Expired: badge.Expired, Archived: badge.Archived, Founding: badge.Founding, RecognitionName: badge.RecognitionName, Collection: badge.Collection,
	}
}

func validIntegrationEndpoint(endpoint *url.URL) bool {
	return endpoint != nil && supporterengine.ValidEndpoint(endpoint.String(), true)
}

func supporterName(tier string) string { return supporterengine.Name(tier) }

func cloneSupporterState(state supporterState) supporterState {
	clone := state
	if state.PatronOrder != nil {
		grant := *state.PatronOrder
		clone.PatronOrder = &grant
	}
	if state.LivingStandard != nil {
		grant := *state.LivingStandard
		clone.LivingStandard = &grant
	}
	return clone
}
