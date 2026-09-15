package quickconnect

import (
	"errors"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/identitycore"
)

type applicationFixture struct {
	application  *Application
	broker       *Broker
	limits       Limiters
	profiles     map[string]identitycore.Profile
	sessions     map[string]identitycore.Session
	persistErr   error
	writes       int
	tokens       int
	renders      int
	rendered     Page
	renderErr    error
	errors       int
	errorMessage string
	errorStatus  int
	current      identitycore.Profile
	currentCalls int
	recentCalls  int
	strong       bool
}

func newApplicationFixture(t *testing.T) *applicationFixture {
	t.Helper()
	fixture := &applicationFixture{
		broker:   New(time.Minute),
		profiles: map[string]identitycore.Profile{"viewer": {ID: "viewer", Name: "Viewer", Remote: true, TOTPSecret: "secret", Revision: 7}},
		sessions: map[string]identitycore.Session{},
		current:  identitycore.Profile{ID: "viewer", Name: "Viewer", Remote: true, TOTPSecret: "secret", Revision: 7},
		strong:   true,
	}
	fixture.limits = Limiters{Requests: &httpguard.Limiter{}, Starts: &httpguard.Limiter{}, Polls: &httpguard.Limiter{}}
	var sessionMu sync.RWMutex
	sessions := identitycore.NewRequestSessions(identitycore.SessionConfig{
		Mutex: &sessionMu, Values: &fixture.sessions,
		Profiles: func() []identitycore.SessionProfile {
			profiles := make([]identitycore.SessionProfile, 0, len(fixture.profiles))
			for _, profile := range fixture.profiles {
				profiles = append(profiles, identitycore.SessionProfile{ID: profile.ID, Name: profile.Name, Owner: profile.Owner, Remote: profile.Remote, Secured: profile.Secured(), Revision: profile.Revision, Disabled: profile.Disabled, Deleted: profile.SCIMDeleted})
			}
			return profiles
		},
		Persist:  func(string, any) error { fixture.writes++; return fixture.persistErr },
		NewToken: func() string { fixture.tokens++; return "token" + strconv.Itoa(fixture.tokens) },
		Now:      func() time.Time { return time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC) },
	}, func(*http.Request) string { return "" })
	adapter := Adapter{
		Profiles: Profiles{
			Find: func(id string) (identitycore.Profile, bool) {
				profile, found := fixture.profiles[id]
				return profile, found
			},
			Compatibility: func(profile identitycore.Profile) identitycore.Profile {
				profile.Owner, profile.Downloads, profile.Transcode, profile.Remote = false, true, true, false
				profile.Rating, profile.Libraries = "all", []string{"all"}
				return profile
			},
			Create: func(id, name string) (string, error) { return sessions.CreateLocal(id, name, false, false) },
			CreatePublic: func(id, name string, revision uint64) (string, error) {
				return sessions.CreatePublicGrant(id, name, false, revision)
			},
			CreateCompat: sessions.CreateCompatibility,
		},
		Current: func(*http.Request) identitycore.Profile { fixture.currentCalls++; return fixture.current },
		RecentlyAuthenticated: func(*http.Request, time.Duration) bool {
			fixture.recentCalls++
			return fixture.strong
		},
		CleanDevice:  identitycore.CleanDeviceName,
		SignInPublic: sessions.SignInPublicGrant,
		Render: func(_ http.ResponseWriter, _ *http.Request, data any) error {
			fixture.renders++
			fixture.rendered = data.(Page)
			return fixture.renderErr
		},
		Error: func(_ http.ResponseWriter, _ *http.Request, message string, status int) {
			fixture.errors++
			fixture.errorMessage, fixture.errorStatus = message, status
		},
	}
	fixture.application = NewApplication(fixture.broker, fixture.limits, adapter)
	return fixture
}

func (fixture *applicationFixture) approved(t *testing.T, request Request, profile identitycore.Profile) string {
	t.Helper()
	secret, connection, err := fixture.broker.Create(request)
	if err != nil {
		t.Fatal(err)
	}
	viewer := Viewer{ID: profile.ID, Revision: profile.Revision, Owner: profile.Owner, Remote: profile.Remote, Secured: profile.Secured(), StronglyVerified: true}
	if err := fixture.broker.Approve(connection.Code, viewer); err != nil {
		t.Fatal(err)
	}
	return secret
}

func TestApplicationConsumesEverySessionKind(t *testing.T) { //nolint:cyclop,gocognit // The three session modes stay together and remain below the repository complexity limit.
	t.Parallel()
	tests := []struct {
		name          string
		request       Request
		compatibility bool
		channel       string
		strong        bool
	}{
		{"local", Request{Device: "TV"}, false, "", false},
		{"compatibility", Request{Device: "TV"}, true, "compatibility", false},
		{"public", Request{Device: "TV", Remote: true}, true, "public", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newApplicationFixture(t)
			secret := fixture.approved(t, test.request, fixture.profiles["viewer"])
			var token string
			var profile identitycore.Profile
			var err error
			if test.compatibility {
				token, profile, err = fixture.application.ConsumeCompatibility(secret)
			} else {
				token, profile, err = fixture.application.Consume(secret)
			}
			if err != nil || token == "" || profile.ID != "viewer" {
				t.Fatalf("consume = %q, %#v, %v", token, profile, err)
			}
			state := fixture.sessions[identitycore.SessionKey(token)]
			if state.Channel != test.channel || (state.StrongAt != 0) != test.strong {
				t.Fatalf("session = %#v", state)
			}
			if test.name == "compatibility" && (profile.Owner || !profile.Downloads || !profile.Transcode || profile.Rating != "all") {
				t.Fatalf("compatibility profile = %#v", profile)
			}
		})
	}
}

func TestApplicationConsumeFailures(t *testing.T) {
	t.Parallel()
	fixture := newApplicationFixture(t)
	secret, _, err := fixture.broker.Create(Request{})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = fixture.application.Consume(secret); !errors.Is(err, ErrPending) {
		t.Fatalf("pending error = %v", err)
	}
	missing := fixture.approved(t, Request{}, fixture.profiles["viewer"])
	delete(fixture.profiles, "viewer")
	if _, _, err = fixture.application.Consume(missing); err == nil || err.Error() != "viewer profile was not found" {
		t.Fatalf("missing profile error = %v", err)
	}
	fixture = newApplicationFixture(t)
	remote := fixture.approved(t, Request{Remote: true}, fixture.profiles["viewer"])
	profile := fixture.profiles["viewer"]
	profile.Revision++
	fixture.profiles["viewer"] = profile
	if _, _, err = fixture.application.Consume(remote); err == nil || err.Error() != "viewer profile was not found" {
		t.Fatalf("revision error = %v", err)
	}
	fixture = newApplicationFixture(t)
	fixture.persistErr = errors.New("disk")
	local := fixture.approved(t, Request{}, fixture.profiles["viewer"])
	if _, profile, err := fixture.application.Consume(local); !errors.Is(err, fixture.persistErr) || profile.ID != "viewer" {
		t.Fatalf("session error = %#v, %v", profile, err)
	}
}

func TestApplicationApprovalAndRevocation(t *testing.T) {
	t.Parallel()
	fixture := newApplicationFixture(t)
	secret, connection, err := fixture.broker.Create(Request{Remote: true})
	if err != nil {
		t.Fatal(err)
	}
	owner := identitycore.Profile{ID: "owner", Owner: true}
	if err := fixture.application.Approve(owner, connection.Code, true); !errors.Is(err, ErrRemoteViewer) || approvalStatus(err) != http.StatusForbidden {
		t.Fatalf("owner approval = %v", err)
	}
	if approvalStatus(ErrInvalidRequest) != http.StatusBadRequest {
		t.Fatal("invalid approval status")
	}
	fixture.application.RevokeRemote()
	if _, found := fixture.broker.Status(secret); found {
		t.Fatal("remote request survived revocation")
	}
	derived := NewApplicationFor(fixture.broker, fixture.limits, fixture.application.adapter, SubtitlesPage())
	if derived.page.Product != "Kinosail Subtitles" || !NewApplication(fixture.broker, fixture.limits, fixture.application.adapter).page.AutoSubmit {
		t.Fatalf("product pages = %#v %#v", derived.page, PlayerPage())
	}
}
