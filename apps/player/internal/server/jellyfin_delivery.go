package server

import (
	"context"
	"log/slog"
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

func jellyfinSourceHLSFile(path string) (string, bool) {
	return sharedjellyfin.SourceHLSFile(path)
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
			HLS: hlsPolicy(), NestedStream: true, SourceHLS: true, SingleQuality: true, RequireHLSCache: true,
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
		LoadSession: func(id string) (any, bool) { return api.plays.Load(id) }, ActiveRevision: api.activePlaybackProfile,
		Rejected: func(ctx context.Context, value sharedjellyfin.PlaybackRejection) {
			slog.WarnContext(ctx, "Jellyfin playback context rejected", "diagnostic", "[JELLYFIN-MAPPING]", "item_id", value.ItemID, "item_found", value.ItemFound, "session_found", value.SessionFound, "session_valid", value.SessionValid, "profile_active", value.ProfileActive, "profile_revision_match", value.ProfileRevisionMatch, "session_item_match", value.SessionItemMatch, "public_match", value.PublicMatch, "expired", value.Expired, "viewer_allowed", value.ViewerAllowed, "item_visible", value.ItemVisible)
		},
		ServeHLS: api.hls.serveSharedHLSRecipe, HLSConfigured: func() bool { return api.hls.cache != "" },
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
		SubtitleUnavailable: func(ctx context.Context, item library.Item, index int, err error) {
			slog.WarnContext(ctx, "Jellyfin embedded subtitle unavailable", "diagnostic", "[PLAYBACK-HLS]", "request_id", requestActivityID(ctx), "item_id", item.ID, "subtitle_index", index, "error", hlsDiagnostic(err, item.Path))
		},
	}
}

func jellyfinDeliveryViewer(profile viewerProfile) sharedjellyfin.DeliveryViewer {
	return sharedjellyfin.DeliveryViewer{ID: profile.ID, Revision: profile.Revision, Owner: profile.Owner, CanTranscode: profile.Transcode}
}
