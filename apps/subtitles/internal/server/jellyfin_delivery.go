package server

import (
	"net/http"
	"time"

	"github.com/MikeO7/kinosail/packages/downloads"
	sharedjellyfin "github.com/MikeO7/kinosail/packages/jellyfincompat"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

func (api *jellyfinAPI) stream(writer http.ResponseWriter, request *http.Request) {
	api.deliveryModule().Stream(writer, request)
}

func (api *jellyfinAPI) activePlaybackProfile(id string, revision uint64) bool {
	profile, active := api.auth.profiles.byID(id)
	return active && profile.Revision == revision
}

func (api *jellyfinAPI) subtitle(writer http.ResponseWriter, request *http.Request) {
	api.deliveryModule().Subtitle(writer, request)
}

func (api *jellyfinAPI) deliveryModule() sharedjellyfin.DeliveryHTTP {
	return sharedjellyfin.DeliveryHTTP{
		Playback: api.playbackModule(), Policy: sharedjellyfin.DeliveryPolicy{
			HLS: hlsPolicy(), SessionTimelineOnly: true, EmbeddedFailureStatus: http.StatusServiceUnavailable,
		},
		CurrentViewer: func(request *http.Request) sharedjellyfin.DeliveryViewer {
			return jellyfinDeliveryViewer(currentViewer(request))
		},
		FindItem: api.index.Find,
		FindViewer: func(id string) (sharedjellyfin.DeliveryViewer, bool) {
			profile, found := api.auth.profiles.byID(id)
			return jellyfinDeliveryViewer(profile), found
		},
		ViewerAllowed: func(request *http.Request, id string, now time.Time) bool {
			profile, found := api.auth.profiles.byID(id)
			return found && profile.Allowed(publicInternetRequest(request), now)
		},
		CanView: func(id string, item library.Item) bool {
			profile, found := api.auth.profiles.byID(id)
			return found && canView(profile, item)
		},
		Public:      publicInternetRequest,
		LoadSession: func(id string) (any, bool) { return playback.LoadJellyfinPlaySession(&api.plays, id, time.Now()) }, ActiveRevision: api.activePlaybackProfile,
		ServeHLS:    api.hls.serveSharedHLSRecipe,
		CanDownload: canDownload, DownloadsConfigured: api.downloads.Configured,
		StartDownload: func(profile string, item library.Item) (downloads.Job, error) {
			return api.downloads.Start(profile, item, "original")
		},
		WaitDownload:  api.downloads.Wait,
		WriteEmbedded: api.probe.writeEmbedded, ExtractEmbedded: api.probe.extractEmbedded,
		WriteSubtitle: func(writer http.ResponseWriter, request *http.Request, path string, timeline *playback.Timeline) {
			if timeline == nil {
				writeSubtitle(writer, request, path)
			} else {
				writeSubtitle(writer, request, path, *timeline)
			}
		},
	}
}

func jellyfinDeliveryViewer(profile viewerProfile) sharedjellyfin.DeliveryViewer {
	return sharedjellyfin.DeliveryViewer{ID: profile.ID, Revision: profile.Revision, Owner: profile.Owner, CanTranscode: profile.Transcode}
}
