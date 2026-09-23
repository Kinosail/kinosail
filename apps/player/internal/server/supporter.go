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
	defaultSupporterActivationURL = "https://kinosail-supporter-prod.pvw-7m4q2x9.workers.dev/v1/supporters/activate"
	productionSupporterPublicKey  = "j-Zo0nvjgd13yuQyZ0bfGpObhf-AQYtdKlFxyFZOL6I"
	defaultSupportURL             = "https://buy.polar.sh/polar_cl_qnIK97BxBRJpKtqw0na3QocqgA3IyY0KlM6wZ36YiP8"
	yearlySupportURL              = "https://buy.polar.sh/polar_cl_WwzGYfc354pJm13xvL90r1qbP0bfSJo0FEDpI3xKiRn"
	oneTimeSupportURL             = "https://buy.polar.sh/polar_cl_U5woq81CkwRbf0B8P9vpyo2mZkxgPD2qIkFsJ2685XU"
	supporterAppID                = "kino-player"
	supporterAudience             = "com.kinosail.player"
	patronOrderFamily             = supporterengine.FamilyPatron
	livingStandardFamily          = supporterengine.FamilyLiving
)

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
	supporterState       = supporterengine.State
	supporterCollection  = supporterengine.Collection
	supporterBadgeStatus = supporterengine.Badge
	supporterBadgeCase   = supporterengine.BadgeCase
	supporterStatus      = supporterengine.Status
)

var supporterTiers = supporterengine.Tiers()

func newSupporterProgram(settings *settingsStore, config SupporterConfig) *supporterProgram {
	trustedPublicKey := ""
	if config.ActivationURL == "" {
		config.ActivationURL = defaultSupporterActivationURL
		trustedPublicKey = productionSupporterPublicKey
	}
	if config.SupportURL == "" {
		config.SupportURL = defaultSupportURL
	}
	app := supporterengine.App{ID: supporterAppID, Name: "Kinosail Player", Audience: supporterAudience, MasterworkName: "Full Sail", GrantPublicKey: true, Legacy: supporterengine.LegacyPlayer}
	service, err := supporterengine.New(supporterengine.Config{App: app, ActivationURL: config.ActivationURL, SupportURL: config.SupportURL, TrustedPublicKey: trustedPublicKey, HTTPClient: config.HTTPClient, Now: config.Now})
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
		return errors.New("recognition name must be 1 to 80 characters without hidden formatting")
	}
	program.mu.Lock()
	defer program.mu.Unlock()
	_, _, err = program.service.ActivateAndSave(ctx, program.settings.supporter(), input, program.settings.setSupporter)
	return err
}

func (program *supporterProgram) status() supporterStatus {
	return program.service.Status(program.settings.supporter())
}

func validIntegrationEndpoint(endpoint *url.URL) bool {
	return endpoint != nil && supporterengine.ValidEndpoint(endpoint.String(), true)
}

func supporterName(tier string) string { return supporterengine.Name(tier) }
