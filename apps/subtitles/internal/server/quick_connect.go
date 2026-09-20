package server

import (
	"net/http"
	"time"

	"github.com/MikeO7/kinosail/packages/httpguard"
	sharedjellyfin "github.com/MikeO7/kinosail/packages/jellyfincompat"
	"github.com/MikeO7/kinosail/packages/quickconnect"
	"github.com/MikeO7/kinosail/packages/webassets"
)

const (
	quickConnectHTML               = quickconnect.HTML
	quickConnectApprovalMaximumAge = quickconnect.ApprovalMaximumAge
	quickConnectRequestBodyMaximum = quickconnect.RequestBodyMaximum
)

var quickConnectView = newLocalizedTemplate("quick-connect", quickConnectHTML)

type quickConnectBroker struct {
	connections *quickconnect.Broker
	requests    httpguard.Limiter
	starts      httpguard.Limiter
	polls       httpguard.Limiter
	consume     func(*profileStore, string) (string, viewerProfile, error)
}

func newQuickConnect(ttl time.Duration) *quickConnectBroker {
	broker := &quickConnectBroker{connections: quickconnect.New(ttl)}
	broker.consume = func(profiles *profileStore, secret string) (string, viewerProfile, error) {
		return broker.application(profiles).Consume(secret)
	}
	return broker
}

func (broker *quickConnectBroker) application(profiles *profileStore) *quickconnect.Application {
	adapter := quickconnect.Adapter{Current: currentViewer, CleanDevice: cleanDeviceName, Render: quickConnectView.Execute, Error: localizedError}
	if profiles != nil {
		adapter.Profiles = quickconnect.Profiles{
			Find: profiles.byID, Compatibility: compatibilityProfile, CreateLocal: profiles.sessionModule().CreateLocalGrant,
			CreatePublic: func(id, name string, revision uint64) (string, error) {
				return profiles.sessionModule().CreatePublicGrant(id, name, false, revision)
			},
		}
		adapter.RecentlyAuthenticated = profiles.recentlyAuthenticated
		adapter.SignInPublic = profiles.sessionModule().SignInPublicGrant
	}
	return quickconnect.NewApplicationFor(broker.connections, quickconnect.Limiters{Requests: &broker.requests, Starts: &broker.starts, Polls: &broker.polls}, adapter, quickconnect.SubtitlesPage())
}

func (broker *quickConnectBroker) register(mux *http.ServeMux, profiles *profileStore) {
	mux.HandleFunc("GET /static/public-login.js", serveScript(webassets.PublicLogin))
	browser := broker.application(profiles)
	mux.HandleFunc("POST /auth/quick-connect", browser.StartBrowser)
	mux.HandleFunc("POST /auth/quick-connect/token", browser.PollBrowser)
	mux.HandleFunc("POST /auth/quick-connect/cancel", browser.CancelBrowser)
	mux.HandleFunc("POST /api/v1/quick-connect", broker.start)
	mux.HandleFunc("POST /api/v1/quick-connect/token", broker.poll(profiles))
	mux.HandleFunc("POST /api/v1/quick-connect/{code}", broker.approveAPI(profiles))
	mux.HandleFunc("GET /quick-connect", broker.page)
	mux.HandleFunc("POST /quick-connect", broker.approve(profiles))
}

func (broker *quickConnectBroker) registerJellyfin(mux *http.ServeMux, api *jellyfinAPI) {
	sharedjellyfin.RegisterQuickConnect(mux, sharedjellyfin.QuickConnectHandlers{
		Start: broker.jellyfinStart, Status: broker.jellyfinStatus,
		Approve: broker.jellyfinApprove(api.auth.profiles), Authenticate: broker.jellyfinAuthenticate(api),
	})
}

func (broker *quickConnectBroker) start(writer http.ResponseWriter, request *http.Request) {
	broker.application(nil).Start(writer, request)
}

func (broker *quickConnectBroker) poll(profiles *profileStore) http.HandlerFunc {
	return broker.application(profiles).Poll
}

func (broker *quickConnectBroker) consumeCompatibility(profiles *profileStore, secret string) (string, viewerProfile, error) {
	return broker.application(profiles).ConsumeCompatibility(secret)
}

func (broker *quickConnectBroker) page(writer http.ResponseWriter, request *http.Request) {
	broker.application(nil).Page(writer, request)
}

func (broker *quickConnectBroker) approve(profiles *profileStore) http.HandlerFunc {
	return broker.application(profiles).ApprovePage
}

func (broker *quickConnectBroker) approveAPI(profiles *profileStore) http.HandlerFunc {
	return broker.application(profiles).ApproveAPI
}

func (broker *quickConnectBroker) approveCode(profile viewerProfile, value string, strong bool) error {
	return broker.application(nil).Approve(profile, value, strong)
}

func (broker *quickConnectBroker) revokeRemote() {
	if broker != nil {
		broker.application(nil).RevokeRemote()
	}
}
