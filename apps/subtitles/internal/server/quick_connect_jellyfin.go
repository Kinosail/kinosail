package server

import (
	"net/http"

	sharedjellyfin "github.com/MikeO7/kinosail/packages/jellyfincompat"
)

func (broker *quickConnectBroker) jellyfinAuthenticate(api *jellyfinAPI) http.HandlerFunc {
	return sharedjellyfin.QuickConnectAuthentication(&broker.polls,
		func(secret string) (string, viewerProfile, error) {
			return broker.consumeCompatibility(api.auth.profiles, secret)
		},
		setAuditViewer, api.jellyfinUser,
	)
}

func (broker *quickConnectBroker) jellyfinApprove(profiles *profileStore) http.HandlerFunc {
	recent := func(request *http.Request) bool {
		return profiles.recentlyAuthenticated(request, quickConnectApprovalMaximumAge)
	}
	return sharedjellyfin.QuickConnectApproval(publicInternetRequest, currentViewer,
		func(profile viewerProfile) string { return profile.ID }, recent, broker.approveCode,
	)
}

func (broker *quickConnectBroker) jellyfinStatus(writer http.ResponseWriter, request *http.Request) {
	sharedjellyfin.QuickConnectStatus(writer, request, broker.connections, &broker.polls)
}

func (broker *quickConnectBroker) jellyfinStart(writer http.ResponseWriter, request *http.Request) {
	sharedjellyfin.StartQuickConnect(writer, request, broker.connections, &broker.starts, publicInternetRequest(request))
}
