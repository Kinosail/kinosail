package server

import (
	"context"
	"net/http"
	"time"

	sharedjellyfin "github.com/MikeO7/kinosail/packages/jellyfincompat"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

func (api *jellyfinAPI) registerPlayback(mux *http.ServeMux) {
	err := api.playbackModule().Register(mux, playback.JellyfinPlaybackHandlers{Stream: api.stream, Subtitle: api.subtitle, Progress: api.playbackProgress, UserData: api.userData, Played: api.played, Favorite: api.favorite})
	if err != nil {
		panic(err)
	}
}

func (api *jellyfinAPI) newPlaySession(itemID string, viewer viewerProfile, plan PlaybackPlan, public bool) (string, error) {
	now := time.Now()
	session := jellyfinPlaySession{itemID: itemID, profileID: viewer.ID, profileRevision: viewer.Revision, expires: now.Add(8 * time.Hour), plan: plan, public: public}
	return playback.StoreJellyfinPlaySession(&api.plays, now, sharedjellyfin.NewID, session)
}

func jellyfinMediaSource(item library.Item, facts MediaFacts, plan PlaybackPlan, playIDs ...string) map[string]any {
	playID := ""
	if len(playIDs) > 0 {
		playID = playIDs[0]
	}
	result, _ := sharedjellyfin.MediaSource(item, facts, plan, subtitlesMediaSourcePolicy(), playID, "")
	return result
}

func subtitlesMediaSourcePolicy() sharedjellyfin.MediaSourcePolicy {
	return sharedjellyfin.MediaSourcePolicy{TranscodingProtocolField: "TranscodingProtocol"}
}

func (api *jellyfinAPI) playbackModule() sharedjellyfin.PlaybackHTTP {
	return sharedjellyfin.PlaybackHTTP{
		VisibleItem: func(request *http.Request, id string) (library.Item, bool) {
			return visibleItem(request, api.index, id)
		},
		VisibleLibrary: func(request *http.Request) ([]library.Item, error) { return visibleLibrary(request, api.index) },
		PrepareImageRequest: func(request *http.Request) *http.Request {
			if currentViewer(request).ID == "" {
				viewer := viewerProfile{Libraries: []string{"all"}, Rating: "all"}
				return request.WithContext(context.WithValue(request.Context(), viewerContextKey{}, viewer))
			}
			return request
		},
		Inspect: func(ctx context.Context, item library.Item) sharedjellyfin.PlaybackMedia {
			media := api.probe.inspect(ctx, item)
			return sharedjellyfin.PlaybackMedia{Facts: mediaFactsFor(item, media), Markers: media.Markers}
		},
		PreferredVideoCodec: api.settings.preferredVideoCodec,
		ViewerPolicy:        func(request *http.Request) ViewerPolicy { return viewerPlaybackPolicy(currentViewer(request)) },
		Decide:              playbackWithAutomaticSkip, AutomaticSkip: api.settings.autoSkip,
		NewSession: func(request *http.Request, itemID string, plan PlaybackPlan) (string, error) {
			return api.newPlaySession(itemID, currentViewer(request), plan, publicInternetRequest(request))
		},
		SourcePolicy: subtitlesMediaSourcePolicy(),
	}
}
