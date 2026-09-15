package server

import (
	"net/http"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	sharedlive "github.com/MikeO7/kinosail/packages/liveevents"
)

type liveEventHub = sharedlive.Hub

func wireLiveEvents(index *libraryIndex, downloads *downloadManager, homeAssistant *homeAssistantIntegration) *liveEventHub {
	hub := sharedlive.New()
	index.AddAnalyzer(func([]library.Item) { hub.Publish("", "library.updated", "/api/v1/library") })
	downloads.SetPublisher(func(job downloadJob) { hub.Publish(job.Profile, "download.updated", "/api/v1/downloads/"+job.ID) })
	homeAssistant.SetPublisher(hub.Publish)
	return hub
}

func liveEventAccess(auth *authentication) sharedlive.Access {
	return sharedlive.Access{
		Profile: func(request *http.Request) string { return currentViewer(request).ID },
		Allowed: func(request *http.Request) bool {
			profile, found := auth.identity(request)
			return found && profile.Allowed(publicInternetRequest(request), time.Now()) && (!profile.APIKey || profileAllowsAPI(profile, "GET /api/v1/events"))
		},
		Error: func(writer http.ResponseWriter, _ *http.Request, err error, status int) {
			apiError(writer, err, status)
		},
	}
}
