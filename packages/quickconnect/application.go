package quickconnect

import (
	"errors"
	"net/http"
	"time"

	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/identitycore"
)

const ApprovalMaximumAge = 10 * time.Minute

type Limiters struct {
	Requests, Starts, Polls *httpguard.Limiter
}

type Profiles struct {
	Find          func(string) (identitycore.Profile, bool)
	Compatibility func(identitycore.Profile) identitycore.Profile
	CreateLocal   func(string, string, uint64, bool) (string, error)
	CreatePublic  func(string, string, uint64) (string, error)
}

type Adapter struct {
	Profiles              Profiles
	Current               func(*http.Request) identitycore.Profile
	RecentlyAuthenticated func(*http.Request, time.Duration) bool
	SignInPublic          func(http.ResponseWriter, *http.Request, string, uint64) error
	CleanDevice           func(string) string
	Render                func(http.ResponseWriter, *http.Request, any) error
	Error                 func(http.ResponseWriter, *http.Request, string, int)
}

type Application struct {
	broker   *Broker
	limiters Limiters
	adapter  Adapter
	page     Page
}

func NewApplication(broker *Broker, limiters Limiters, adapter Adapter) *Application {
	return NewApplicationFor(broker, limiters, adapter, PlayerPage())
}

func NewApplicationFor(broker *Broker, limiters Limiters, adapter Adapter, page Page) *Application {
	return &Application{broker: broker, limiters: limiters, adapter: adapter, page: page}
}

func (application *Application) Consume(secret string) (string, identitycore.Profile, error) {
	return application.consumeSession(secret, false)
}

func (application *Application) ConsumeCompatibility(secret string) (string, identitycore.Profile, error) {
	return application.consumeSession(secret, true)
}

func (application *Application) consumeSession(secret string, compatibility bool) (string, identitycore.Profile, error) {
	grant, err := application.broker.Consume(secret)
	if err != nil {
		return "", identitycore.Profile{}, err
	}
	profile, found := application.adapter.Profiles.Find(grant.ProfileID)
	if !found || grant.ProfileRevision != profile.Revision {
		return "", identitycore.Profile{}, errors.New("viewer profile was not found")
	}
	if grant.Remote {
		token, err := application.adapter.Profiles.CreatePublic(profile.ID, grant.Device, grant.ProfileRevision)
		return token, profile, err
	}
	compatibility = compatibility || profile.Owner && !grant.Owner
	if compatibility {
		profile = application.adapter.Profiles.Compatibility(profile)
	}
	token, err := application.adapter.Profiles.CreateLocal(profile.ID, grant.Device, grant.ProfileRevision, compatibility)
	return token, profile, err
}

func (application *Application) Approve(profile identitycore.Profile, value string, strong bool) error {
	return application.broker.Approve(value, Viewer{
		ID: profile.ID, Revision: profile.Revision, Owner: profile.Owner, Remote: profile.Remote,
		Secured: profile.Secured(), StronglyVerified: strong,
	})
}

func (application *Application) RevokeRemote() {
	application.broker.RevokeRemote()
}

func approvalStatus(err error) int {
	if errors.Is(err, ErrCapacity) {
		return http.StatusTooManyRequests
	}
	if errors.Is(err, ErrRemoteViewer) {
		return http.StatusForbidden
	}
	return http.StatusBadRequest
}
