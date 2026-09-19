package server

import (
	"context"
	_ "embed"
	"net/http"
	"strconv"
	"time"

	"github.com/MikeO7/kinosail/packages/auditjournal"
	"github.com/MikeO7/kinosail/packages/catalogapi"
	"github.com/MikeO7/kinosail/packages/library"
	sharedmetadata "github.com/MikeO7/kinosail/packages/metadata"
	sharedplayback "github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/productapi"
	"github.com/MikeO7/kinosail/packages/remoteaccess"
)

//go:embed api_openapi.json
var openAPISpec []byte

type apiPlayback struct {
	Plan                  PlaybackPlan      `json:"plan"`
	CompatiblePlan        *PlaybackPlan     `json:"compatiblePlan,omitempty"`
	CompatibleLabel       string            `json:"compatibleLabel,omitempty"`
	CompatibleDescription string            `json:"compatibleDescription,omitempty"`
	Qualities             []PlaybackQuality `json:"qualities,omitempty"`
	Direct                string            `json:"direct,omitempty"`
	Compatible            string            `json:"compatible,omitempty"`
	Download              string            `json:"download,omitempty"`
	DirectType            string            `json:"directType,omitempty"`
	Summary               string            `json:"summary,omitempty"`
	Duration              float64           `json:"duration,omitempty"`
	Start                 float64           `json:"start,omitempty"`
	Audio                 []apiAudioTrack   `json:"audio"`
	Chapters              []chapter         `json:"chapters"`
	Markers               []playbackMarker  `json:"markers"`
	AutoSkip              []string          `json:"autoSkip"`
	Subtitles             []subtitleTrack   `json:"subtitles"`
	Next                  string            `json:"next,omitempty"`
	Trickplay             string            `json:"trickplay,omitempty"`
	ProgressToken         string            `json:"progressToken,omitempty"`
	ReplayGain            *apiReplayGain    `json:"replayGain,omitempty"`
}

func (result *apiPlayback) SetPlaybackTimeline(duration, start float64, chapters []sharedmetadata.Chapter, markers []playbackMarker, autoSkip []string, token string) {
	result.Duration, result.Start, result.Chapters, result.Markers, result.AutoSkip, result.ProgressToken = duration, start, chapters, markers, autoSkip, token
}

type apiAudioTrack struct {
	Index  int    `json:"index"`
	Label  string `json:"label"`
	Source string `json:"source,omitempty"`
}

func registerProductAPI(mux *http.ServeMux, api apiServices) {
	owner := func(pattern string, handler http.Handler) { mux.Handle(pattern, api.auth.owner(handler)) }
	mux.HandleFunc("GET /api/v1", apiRoot)
	mux.HandleFunc("GET /api/v1/me", productapi.Me(func(request *http.Request) productapi.MeState {
		return productapi.MeState{Server: api.settings.serverName(), Viewer: currentViewer(request), SSO: api.auth.sso, Language: preferredLanguage(request), LanguagePreference: languagePreference(request), Languages: languageOptions()}
	}))
	mux.HandleFunc("PUT /api/v1/me/language", apiLanguage)
	mux.HandleFunc("GET /api/v1/openapi.json", serveOpenAPI)
	mux.HandleFunc("GET /api/v1/items/{id}/playback", apiPlaybackInfo(api))
	mux.HandleFunc("GET /api/v1/items/{id}/watch-progress", sharedplayback.WatchProgressHandler(func(request *http.Request, id string) (float64, float64, bool) {
		item, found := visibleItem(request, api.index, id)
		if !found {
			return 0, 0, false
		}
		media := api.probe.inspect(request.Context(), item)
		return api.progress.Get(request, id).Seconds, media.Duration, true
	}))
	mux.HandleFunc("POST /api/v1/watch-rooms", productapi.CreateRoom(productRooms{api}))
	mux.HandleFunc("GET /api/v1/watch-rooms/{id}", apiRoom(api))
	owner("POST /api/v1/metadata/bulk", productapi.BulkEditMetadata(productapi.BulkMetadataFunc(func(ctx context.Context, ids []string, patch productapi.MetadataPatch) (int, error) {
		updates, err := api.metadata.applyPatch(ctx, api.index, ids, metadataPatch(patch))
		return len(updates), err
	}), errMetadataSelection, errMetadataFields))
	metadata := productMetadata{index: api.index, store: api.metadata}
	owner("PUT /api/v1/items/{id}/metadata", productapi.EditMetadata(metadata, metadata))
	owner("POST /api/v1/items/{id}/metadata/refresh", productapi.RefreshMetadata(metadata, metadata))
	owner("POST /api/v1/items/{id}/subtitles", apiFetchSubtitles(api))
	owner("GET /api/v1/remote-access", remoteaccess.StatusAPI(api.internet, func(status remoteaccess.Status) remoteReadiness {
		return securePublicReadiness(status, api.auth.profiles.list(), api.auth.profiles.err, time.Now())
	}, writeJSON))
	contractError := func(message string) error { return apiContractError(message) }
	owner("POST /api/v1/remote-access/kill", remoteaccess.KillAPI(api.internet, func() error {
		return revokePublicAuthorization(api.auth.profiles, api.auth.passkeys, api.quick, api.shares)
	}, contractError, apiError)) //nolint:contextcheck // Once the kill switch begins, revocation must finish despite request cancellation.
	owner("DELETE /api/v1/remote-access/kill", remoteaccess.ResetKillAPI(api.internet, contractError, apiError))
	owner("GET /api/v1/activity", auditjournal.ActivityAPI(api.auth.audit.HTTPTracker, func() []playbackView {
		return catalogapi.RecentAdminProgress(api.progress, api.index, api.auth.profiles.list())
	}, writeJSON))
	owner("GET /api/v1/activity/export", auditjournal.ExportAPI(api.auth.audit.HTTPTracker))
	mux.Handle("GET /api/v1/backup", api.auth.freshOwner(downloadBackup(api.settings)))
}

func apiRoot(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, map[string]any{"name": "Kinosail API", "version": "v1", "openapi": "/api/v1/openapi.json", "libraryContent": "direct-only"}, http.StatusOK)
}

func serveOpenAPI(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "application/vnd.oai.openapi+json")
	writer.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = writer.Write(openAPISpec)
}

func apiPlaybackInfo(api apiServices) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		videoCodecs, err := requestedVideoCodecs(request)
		if err != nil {
			apiError(writer, err, http.StatusBadRequest)
			return
		}
		audioCodecs, err := sharedplayback.RequestedAudioCodecs(request)
		if err != nil {
			apiError(writer, err, http.StatusBadRequest)
			return
		}
		item, found := visibleItem(request, api.index, request.PathValue("id"))
		if !found {
			apiNotFound(writer)
			return
		}
		media, viewer := api.probe.inspect(request.Context(), item), currentViewer(request)
		canStream, canTranscode := viewer.Permits("stream", true), item.Kind == "video" && viewer.Permits("stream", viewer.Owner || viewer.Transcode)
		facts := mediaFactsFor(item, media)
		intent, policy := playbackModeIntent(api.settings), viewerPlaybackPolicy(viewer)
		policy.AllowTranscode = policy.AllowTranscode && !intent.ForceDirect
		client := browserPlaybackCapabilities(api.settings, videoCodecs)
		if audioCodecs != nil {
			client.AudioCodecs = audioCodecs
		}
		plan := playbackWithAutomaticSkip(facts, client, policy, intent, media.Markers, api.settings.autoSkip())
		result := apiPlayback{Plan: plan, Summary: media.Summary, Duration: media.Duration, Start: api.progress.Get(request, item.ID).Seconds, Audio: apiAudioSources(item.ID, media.Audio, canTranscode), Chapters: media.Chapters, Markers: media.Markers, AutoSkip: automaticSkipSelection(media.Markers, api.settings.autoSkip()), Next: autoNext(request, api.settings, api.index, item), ReplayGain: apiReplayGainFor(media.ReplayGain)}
		sharedplayback.ApplyAPIPlaybackTimeline(&result, plan, result.Start, media.Duration, media.Chapters, media.Markers, api.settings.autoSkip(), result.AutoSkip, func() string { return recipeFor(plan).token() })
		api.applyPlaybackSources(&result, item, media, viewer, facts, client, plan, canStream, canTranscode)
		writeJSON(writer, result, http.StatusOK)
	}
}

func apiAudioSources(id string, tracks []audioTrack, enabled bool) []apiAudioTrack {
	result := make([]apiAudioTrack, 0, len(tracks))
	for _, track := range tracks {
		source := ""
		if enabled {
			source = "/hls/" + id + "/index.m3u8"
			if track.Index > 0 {
				source = "/hls/" + id + "/audio/" + strconv.Itoa(track.Index) + "/index.m3u8"
			}
		}
		result = append(result, apiAudioTrack{track.Index, track.Label, source})
	}
	return result
}

type apiContractError string

func (err apiContractError) Error() string { return string(err) }

func apiRoom(api apiServices) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		event, found := api.rooms.rooms.Snapshot(request.PathValue("id"), currentViewer(request).ID)
		if !found || !roomMediaVisible(request, api.index, event.Media) {
			apiNotFound(writer)
			return
		}
		writeJSON(writer, event, http.StatusOK)
	}
}

type productMetadata struct {
	index *libraryIndex
	store *metadataStore
}

type productRooms struct{ apiServices }

func (service productRooms) Visible(request *http.Request, id string) (library.Item, bool) {
	return visibleItem(request, service.index, id)
}

func (service productRooms) Viewer(request *http.Request) viewerProfile {
	return currentViewer(request)
}

func (service productRooms) Create(viewer viewerProfile, media string, seconds float64) (string, bool) {
	return service.rooms.create(viewer, media, seconds)
}

func (service productMetadata) Find(id string) (library.Item, bool) { return service.index.Find(id) }

func (service productMetadata) Refresh(ctx context.Context) error { return service.index.Refresh(ctx) }

func (service productMetadata) Record(id string) sharedmetadata.Record {
	service.store.mu.RLock()
	defer service.store.mu.RUnlock()
	return service.store.records[id]
}

func (service productMetadata) Save(id string, record sharedmetadata.Record) error {
	return service.store.set(id, record)
}

func (service productMetadata) Fetch(ctx context.Context, item library.Item) error {
	return service.store.fetch(ctx, item)
}

func apiFetchSubtitles(api apiServices) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var input struct{ Language string }
		if !readJSON(writer, request, &input) {
			return
		}
		item, found := api.index.Find(request.PathValue("id"))
		if input.Language == "" {
			input.Language = api.settings.subtitleLanguage()
		}
		if !found || item.Kind != "video" || !validLanguage(input.Language) {
			apiError(writer, apiContractError("subtitle request is invalid"), http.StatusBadRequest)
			return
		}
		if err := api.subtitles.fetch(request.Context(), item, input.Language); err != nil {
			apiError(writer, err, http.StatusBadGateway)
			return
		}
		writeJSON(writer, map[string]string{"source": "/subtitles/" + item.ID + "/" + input.Language}, http.StatusCreated)
	}
}
