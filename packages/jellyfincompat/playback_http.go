package jellyfincompat

import (
	"context"
	"errors"
	"net/http"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/markers"
	"github.com/MikeO7/kinosail/packages/playback"
)

// PlaybackMedia contains normalized facts needed by Player's Jellyfin planner.
type PlaybackMedia struct {
	Facts   playback.MediaFacts
	Markers []markers.Marker
}

// MediaSourcePolicy preserves one app's Jellyfin delivery contract.
type MediaSourcePolicy struct {
	IncludeToken             bool
	QueryOnDirectPath        bool
	TranscodingProtocolField string
}

// PlaybackHTTP contains true app adapters for Player's playback flow.
type PlaybackHTTP struct {
	VisibleItem         func(*http.Request, string) (library.Item, bool)
	VisibleLibrary      func(*http.Request) ([]library.Item, error)
	PrepareImageRequest func(*http.Request) *http.Request
	Inspect             func(context.Context, library.Item) PlaybackMedia
	PreferredVideoCodec func([]string) string
	ViewerPolicy        func(*http.Request) playback.ViewerPolicy
	Decide              func(playback.MediaFacts, playback.ClientCapabilities, playback.ViewerPolicy, playback.NetworkIntent, []markers.Marker, []string) playback.PlaybackPlan
	AutomaticSkip       func() []string
	NewSession          func(*http.Request, string, playback.PlaybackPlan) (string, error)
	SessionToken        func(*http.Request) (string, string)
	SourcePolicy        MediaSourcePolicy
	Resolved            func(context.Context, library.Item)
}

// Register installs all stable Jellyfin playback routes.
func (module PlaybackHTTP) Register(mux *http.ServeMux, handlers playback.JellyfinPlaybackHandlers) error {
	handlers.PlaybackInfo, handlers.Image = module.PlaybackInfo, module.Image
	return playback.RegisterJellyfinPlayback(mux, handlers)
}

// PlaybackInfo validates, plans, stores, and projects one playback request.
func (module PlaybackHTTP) PlaybackInfo(writer http.ResponseWriter, request *http.Request) { //nolint:cyclop // Ordered validation maps each Jellyfin protocol failure to its wire response.
	id := request.PathValue("id")
	if !validRequiredValue(id, 128) {
		http.NotFound(writer, request)
		return
	}
	item, found := module.VisibleItem(request, RawID(id))
	if !found || item.Kind == "photo" {
		http.NotFound(writer, request)
		return
	}
	input, profileProvided, valid := playback.ReadJellyfinPlaybackRequest(request)
	if !valid {
		http.Error(writer, "invalid playback request", http.StatusBadRequest)
		return
	}
	token := ""
	if module.SourcePolicy.IncludeToken {
		candidate, source := module.SessionToken(request)
		if source != "kinosail-session-cookie" {
			if !validOptionalValue(candidate, 256) {
				http.Error(writer, "invalid playback request", http.StatusBadRequest)
				return
			}
			token = candidate
		}
	}
	if module.Resolved != nil {
		module.Resolved(request.Context(), item)
	}
	media := module.Inspect(request.Context(), item)
	client := playback.JellyfinCapabilities(input.DeviceProfile, media.Facts)
	client.TranscodeVideoCodecs = []string{module.PreferredVideoCodec(client.TranscodeVideoCodecs)}
	policy := module.ViewerPolicy(request)
	if profileProvided && len(input.DeviceProfile.TranscodingProfiles) == 0 {
		policy.AllowTranscode = false
	}
	intent := playback.NetworkIntent{MaxBitrate: input.MaxStreamingBitrate, AudioIndex: input.AudioStreamIndex, SubtitleIndex: input.SubtitleStreamIndex}
	plan := module.Decide(media.Facts, client, policy, intent, media.Markers, module.AutomaticSkip())
	playID, err := module.NewSession(request, item.ID, plan)
	if err != nil {
		http.Error(writer, "could not create playback session", http.StatusInternalServerError)
		return
	}
	source, err := MediaSource(item, media.Facts, plan, module.SourcePolicy, playID, token)
	if err != nil {
		http.Error(writer, "could not create playback session", http.StatusInternalServerError)
		return
	}
	JSON(writer, map[string]any{"MediaSources": []any{source}, "PlaySessionId": playID})
}

// MediaSource projects one validated Player or Subtitles source.
func MediaSource(item library.Item, facts playback.MediaFacts, plan playback.PlaybackPlan, policy MediaSourcePolicy, playID, token string) (map[string]any, error) {
	if !validRequiredValue(playID, 256) || !validOptionalValue(token, 256) {
		return nil, errors.New("jellyfin media source identity is invalid")
	}
	return playback.JellyfinMediaSource(item, facts, plan, playback.JellyfinMediaSourceOptions{
		PlaySessionID: playID, Token: token, QueryOnDirectPath: policy.QueryOnDirectPath,
		TranscodingProtocolField: policy.TranscodingProtocolField,
	})
}

// Image finds and serves artwork through Player's public-image policy.
func (module PlaybackHTTP) Image(writer http.ResponseWriter, request *http.Request) {
	id := request.PathValue("id")
	if !validRequiredValue(id, 128) {
		http.NotFound(writer, request)
		return
	}
	if module.PrepareImageRequest != nil {
		request = module.PrepareImageRequest(request)
	}
	item, found := module.ArtworkItem(request, id)
	if !found || item.Artwork == "" {
		http.NotFound(writer, request)
		return
	}
	http.ServeFile(writer, request, item.Artwork)
}

// ArtworkItem selects item, series, or season artwork from visible content.
func (module PlaybackHTTP) ArtworkItem(request *http.Request, id string) (library.Item, bool) {
	if item, found := module.VisibleItem(request, RawID(id)); found {
		return item, true
	}
	items, _ := module.VisibleLibrary(request)
	_, shows := library.Organize(items)
	for _, show := range shows {
		if ID(show.ID) == id {
			item, found := findItem(items, show.ArtworkID)
			item.Artwork = show.Artwork
			return item, found
		}
		for _, episode := range show.Episodes {
			if SeasonID(show.ID, episode.Season) == id && episode.Artwork != "" {
				return episode, true
			}
		}
	}
	return library.Item{}, false
}

func validRequiredValue(value string, maximum int) bool {
	return value != "" && validOptionalValue(value, maximum)
}
