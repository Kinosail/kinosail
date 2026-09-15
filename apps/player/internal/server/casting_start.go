package server

import (
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strconv"

	"github.com/MikeO7/kinosail/packages/casting"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

func (service *castService) authorizeCast(request *http.Request, itemID string, input casting.Start) (library.Item, viewerProfile, string, error) {
	viewer := currentViewer(request)
	if !viewer.Permits("stream", true) || viewer.ID == "" {
		return library.Item{}, viewerProfile{}, "", errors.New("TV playback is not allowed") //nolint:staticcheck // This complete sentence is displayed directly in the TV picker.
	}
	item, found := visibleItem(request, service.index, itemID)
	if !found || !slices.Contains([]string{"video", "music", "audio", "audiobook"}, item.Kind) {
		return library.Item{}, viewerProfile{}, "", errors.New("This title cannot play on a TV") //nolint:staticcheck // This complete sentence is displayed directly in the TV picker.
	}
	if !validCastBase(service.base) {
		return library.Item{}, viewerProfile{}, "", errors.New("A reachable Kinosail Server address is required for TV playback") //nolint:staticcheck // This complete sentence is displayed directly in the TV picker.
	}
	deviceName := ""
	if input.Protocol == "dlna" {
		device, ok := service.renderers.Device(input.DeviceID)
		if !ok {
			return library.Item{}, viewerProfile{}, "", errors.New("TV is no longer available; search for devices again") //nolint:staticcheck // This complete sentence is displayed directly in the TV picker.
		}
		deviceName = device.Name
	}
	return item, viewer, deviceName, nil
}

func validCastBase(raw string) bool {
	base, err := url.Parse(raw)
	return err == nil && base.Host != "" && (base.Scheme == "http" || base.Scheme == "https") && base.User == nil && base.RawQuery == "" && base.Fragment == "" && (base.Path == "" || base.Path == "/")
}

func castSourcePosition(input casting.Start, duration float64) (float64, error) {
	if !casting.Position(duration) {
		return 0, casting.ErrInvalid
	}
	position := input.Position
	if input.PlaybackToken != "" {
		timeline, parseErr := timelineFromPlaybackToken(input.PlaybackToken)
		if parseErr != nil {
			return 0, casting.ErrInvalid
		}
		timeline.SourceDuration = duration
		timeline.Duration = duration
		for _, part := range timeline.Omitted {
			if part.End > duration {
				return 0, casting.ErrInvalid
			}
			timeline.Duration -= part.End - part.Start
		}
		if position > timeline.Duration {
			return 0, casting.ErrInvalid
		}
		position = timeline.SourceTime(position)
	}
	if position > duration {
		return 0, casting.ErrInvalid
	}
	return position, nil
}

func (service *castService) planCast(facts MediaFacts, viewer viewerProfile, kind, protocol string) (PlaybackPlan, error) {
	capabilities := browserCapabilities()
	capabilities.Containers, capabilities.VideoCodecs, capabilities.AudioCodecs = []string{"mp4", "mp3", "aac", "wav"}, []string{"h264"}, []string{"aac", "mp3"}
	capabilities.TranscodeVideoCodecs, capabilities.MaxWidth, capabilities.MaxHeight = []string{"h264"}, 1920, 1080
	capabilities.SupportsExternalSubtitles = false
	policy := viewerPlaybackPolicy(viewer)
	intent := playbackModeIntent(service.settings)
	policy.AllowTranscode = policy.AllowTranscode && !intent.ForceDirect && kind == "video"
	// Use the original timeline on receivers; automatic skip remains a sender feature.
	plan := playback.Decide(facts, capabilities, policy, intent, playback.DecisionPolicy{PreferCompatibleAudio: true})
	if !plan.Allowed {
		return PlaybackPlan{}, errors.New("This title is not compatible with the TV and this profile's playback permissions") //nolint:staticcheck // This complete sentence is displayed directly in the TV picker.
	}
	if protocol == "dlna" && plan.Mode != "direct" {
		return PlaybackPlan{}, errors.New("This TV needs a directly playable file; use AirPlay or Google Cast for this title") //nolint:staticcheck // This complete sentence is displayed directly in the TV picker.
	}
	return plan, nil
}

func (service *castService) attachCastTracks(session *castSession, item library.Item, media probeResult, token string) {
	session.Tracks = []castTrack{}
	if session.Protocol == "google-cast" {
		for index, track := range playbackSubtitles(item, media, service.settings.subtitleLanguage(), service.settings.subtitlesDefault()) {
			if index >= 64 {
				break
			}
			session.Tracks = append(session.Tracks, castTrack{ID: index + 1, URL: service.base + "/cast/" + session.ID + "/subtitles/" + strconv.Itoa(index+1) + "?ticket=" + token, Label: track.Label, Language: track.Language, Default: track.Default, source: track.Source})
		}
	}
}

func (service *castService) rememberCast(session castSession) error {
	service.mu.Lock()
	count := 0
	for key, existing := range service.sessions {
		if !existing.ExpiresAt.After(service.now()) {
			delete(service.sessions, key)
		} else if existing.profileID == session.profileID {
			count++
		}
	}
	if count >= 4 || len(service.sessions) >= 128 {
		service.mu.Unlock()
		return errors.New("Stop an existing TV session before starting another") //nolint:staticcheck // This complete sentence is displayed directly in the TV picker.
	}
	stored := session
	stored.Tracks = append([]castTrack(nil), session.Tracks...)
	for index := range stored.Tracks {
		stored.Tracks[index].URL = ""
	}
	stored.URL = "" // Never retain the receiver bearer capability in the session registry.
	if session.Protocol == "dlna" {
		for _, existing := range service.sessions {
			if existing.DeviceID == session.DeviceID {
				service.mu.Unlock()
				return errors.New("This TV already has an active Kinosail session") //nolint:staticcheck // This complete sentence is displayed directly in the TV picker.
			}
		}
	}
	service.sessions[session.ID] = stored
	service.mu.Unlock()
	return nil
}
