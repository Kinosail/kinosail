// Package supporter adapts Dashboard persistence to Player's shared supporter module.
package supporter

import (
	"context"
	"net/http"
	"sync"
	"time"

	supporterengine "github.com/MikeO7/kinosail/packages/supporter"
)

const (
	AppID         = "kino-dashboard"
	Audience      = "com.kinosail.dashboard"
	stateDocument = "supporter.json"
)

var (
	ErrInvalid     = supporterengine.ErrInvalid
	ErrUnavailable = supporterengine.ErrUnavailable
	ErrUncertain   = supporterengine.ErrUncertain
	ErrConflict    = supporterengine.ErrConflict
)

type Store interface {
	LoadJSON(context.Context, string, any) (bool, error)
	SaveJSON(context.Context, string, any) error
}

type Config struct {
	ActivationURL string
	SupportURL    string
	HTTPClient    *http.Client
	Now           func() time.Time
}

type (
	Grant           = supporterengine.Grant
	Activation      = supporterengine.Activation
	State           = supporterengine.State
	BadgeCase       = supporterengine.BadgeCase
	ActivationInput = supporterengine.ActivationInput
)

type Badge struct {
	Family          string `json:"family"`
	Tier            string `json:"tier"`
	Name            string `json:"name"`
	SupporterID     string `json:"supporterId"`
	SupportedSince  string `json:"supportedSince"`
	ExpiresAt       string `json:"expiresAt,omitempty"`
	RecognitionName string `json:"recognitionName,omitempty"`
	Rank            int    `json:"rank"`
	Active          bool   `json:"active"`
	Expired         bool   `json:"expired"`
	Founding        bool   `json:"founding"`
}

type Status struct {
	AppID               string    `json:"appId"`
	Tier                string    `json:"tier"`
	Name                string    `json:"name"`
	SupporterID         string    `json:"supporterId,omitempty"`
	SupportedSince      string    `json:"supportedSince,omitempty"`
	ExpiresAt           string    `json:"expiresAt,omitempty"`
	Rank                int       `json:"rank"`
	Active              bool      `json:"active"`
	Expired             bool      `json:"expired"`
	Sustaining          bool      `json:"sustaining"`
	Founding            bool      `json:"founding"`
	RecognitionName     string    `json:"recognitionName,omitempty"`
	PatronOrder         *Badge    `json:"patronOrder"`
	LivingStandard      *Badge    `json:"livingStandard"`
	BadgeCase           BadgeCase `json:"badgeCase"`
	ActivationAvailable bool      `json:"activationAvailable"`
	SupportURL          string    `json:"supportUrl"`
}

type Service struct {
	mu     sync.Mutex
	store  Store
	state  State
	engine *supporterengine.Service
}

func New(ctx context.Context, store Store, config Config) (*Service, error) {
	if ctx == nil {
		return nil, ErrInvalid
	}
	engine, err := supporterengine.New(supporterengine.Config{
		App: supporterengine.App{
			ID: AppID, Name: "Kinosail Dashboard", Audience: Audience, MasterworkName: "Fleet Command", TrackActivations: true,
			RejectPatronDowngrade: true, AllowEmptyRecognitionField: true, ExposeActivationAvailable: true,
			AcceptInvalidRequestError: true, BindSupporterIDToInstallation: true,
		},
		ActivationURL: config.ActivationURL, SupportURL: config.SupportURL, HTTPClient: config.HTTPClient, Now: config.Now,
	})
	if err != nil {
		return nil, err
	}
	service := &Service{store: store, engine: engine}
	if store != nil {
		found, loadErr := store.LoadJSON(ctx, stateDocument, &service.state)
		if loadErr != nil || found && engine.ValidateState(service.state) != nil {
			return nil, ErrInvalid
		}
	}
	return service, nil
}

func (service *Service) Status() Status {
	service.mu.Lock()
	defer service.mu.Unlock()
	return dashboardStatus(service.engine.Status(service.state))
}

func (service *Service) Activate(ctx context.Context, input ActivationInput) (Status, error) {
	if service.engine.ValidateInput(input) != nil {
		return service.Status(), ErrInvalid
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.store == nil {
		return dashboardStatus(service.engine.Status(service.state)), ErrUnavailable
	}
	next, status, err := service.engine.Activate(ctx, service.state, input)
	if err != nil {
		return dashboardStatus(service.engine.Status(service.state)), err
	}
	if service.engine.ValidateState(next) != nil || service.store.SaveJSON(context.WithoutCancel(ctx), stateDocument, next) != nil {
		return dashboardStatus(service.engine.Status(service.state)), ErrUnavailable
	}
	service.state = next
	return dashboardStatus(status), nil
}

func dashboardStatus(status supporterengine.Status) Status {
	return Status{
		AppID: status.AppID, Tier: status.Tier, Name: status.Name, SupporterID: status.SupporterID, SupportedSince: status.SupportedSince,
		ExpiresAt: status.ExpiresAt, Rank: status.Rank, Active: status.Active, Expired: status.Expired, Sustaining: status.Sustaining,
		Founding: status.Founding, RecognitionName: status.RecognitionName, PatronOrder: dashboardBadge(status.PatronOrder),
		LivingStandard: dashboardBadge(status.LivingStandard), BadgeCase: status.BadgeCase, ActivationAvailable: status.ActivationAvailable, SupportURL: status.SupportURL,
	}
}

func dashboardBadge(badge *supporterengine.Badge) *Badge {
	if badge == nil {
		return nil
	}
	return &Badge{
		Family: badge.Family, Tier: badge.Tier, Name: badge.Name, SupporterID: badge.SupporterID, SupportedSince: badge.SupportedSince,
		ExpiresAt: badge.ExpiresAt, RecognitionName: badge.RecognitionName, Rank: badge.Rank, Active: badge.Active, Expired: badge.Expired, Founding: badge.Founding,
	}
}
