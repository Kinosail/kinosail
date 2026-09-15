package playback

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

const maxPlaybackTraceMilliseconds = 31_622_400_000

type TraceEvent struct {
	Session         string `json:"session"`
	Event           string `json:"event"`
	Method          string `json:"method"`
	Detail          string `json:"detail"`
	Quality         string `json:"quality"`
	Visibility      string `json:"visibility"`
	Sequence        int64  `json:"sequence"`
	ElapsedMS       int64  `json:"elapsedMs"`
	PositionMS      int64  `json:"positionMs"`
	DurationMS      int64  `json:"durationMs"`
	BufferedAheadMS int64  `json:"bufferedAheadMs"`
	ReadyState      int    `json:"readyState"`
	NetworkState    int    `json:"networkState"`
	ErrorCode       int    `json:"errorCode"`
	DroppedFrames   int64  `json:"droppedFrames"`
	TotalFrames     int64  `json:"totalFrames"`
	Paused          bool   `json:"paused"`
}

var playbackTraceEvents = stringSet("session-start", "capability", "capability-direct", "first-frame", "first-moving-frame", "frame-after-seek", "loadstart", "loadedmetadata", "canplay", "play-request", "play-rejected", "play", "playing", "waiting", "stalled", "seeking", "seeked", "pause", "ended", "error", "progress", "source-direct", "source-compatible", "fallback-offered", "fallback-selected", "hls-manifest", "hls-level", "hls-error", "policy-change", "heartbeat", "visibility-hidden", "visibility-visible", "session-end")

// ReadTrace decodes one strict, bounded trace without causing application side effects.
func ReadTrace(writer http.ResponseWriter, request *http.Request, validSession func(string) bool) (TraceEvent, error) {
	if writer == nil || request == nil || request.Body == nil || validSession == nil {
		return TraceEvent{}, errors.New("invalid playback trace")
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 4096)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var event TraceEvent
	if err := decoder.Decode(&event); err != nil {
		return TraceEvent{}, errors.New("invalid playback trace")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) || !ValidTrace(event, validSession) {
		return TraceEvent{}, errors.New("invalid playback trace")
	}
	return event, nil
}

func ValidTrace(event TraceEvent, validSession func(string) bool) bool { //nolint:cyclop // Every trace field is validated before logging.
	return validSession != nil && validSession(event.Session) && playbackTraceEvents[event.Event] && event.Sequence > 0 && event.Sequence <= 1_000_000 &&
		oneOf(event.Method, "", "direct", "remux", "audio-transcode", "transcode", "native-hls", "offline") && SafeTraceText(event.Detail) && SafeTraceText(event.Quality) &&
		oneOf(event.Visibility, "", "visible", "hidden") && boundedTraceNumber(event.ElapsedMS, maxPlaybackTraceMilliseconds) && boundedTraceNumber(event.PositionMS, maxPlaybackTraceMilliseconds) &&
		boundedTraceNumber(event.DurationMS, maxPlaybackTraceMilliseconds) && boundedTraceNumber(event.BufferedAheadMS, maxPlaybackTraceMilliseconds) && event.ReadyState >= 0 && event.ReadyState <= 4 &&
		event.NetworkState >= 0 && event.NetworkState <= 3 && event.ErrorCode >= 0 && event.ErrorCode <= 4 && boundedTraceNumber(event.DroppedFrames, 1_000_000_000) && boundedTraceNumber(event.TotalFrames, 1_000_000_000)
}

func SafeTraceText(value string) bool { //nolint:cyclop // The allowlist is deliberately explicit at this logging boundary.
	return len(value) <= 64 && strings.IndexFunc(value, func(character rune) bool {
		return character != '-' && character != '_' && character != '.' && character != ':' && (character < '0' || character > '9') && (character < 'A' || character > 'Z') && (character < 'a' || character > 'z')
	}) == -1
}

func boundedTraceNumber(value, maximum int64) bool { return value >= 0 && value <= maximum }

func stringSet(values ...string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}
