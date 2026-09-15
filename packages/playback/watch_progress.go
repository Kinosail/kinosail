package playback

import (
	"encoding/json"
	"math"
	"net/http"
	"strings"
)

// WatchProgressHandler reads the visible item's original runtime and the current
// viewer's saved position. It does not prepare or start a playback session.
func WatchProgressHandler(lookup func(*http.Request, string) (float64, float64, bool)) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request == nil || lookup == nil {
			http.Error(writer, "watch progress is unavailable", http.StatusServiceUnavailable)
			return
		}
		id := request.PathValue("id")
		if !validWatchProgressID(id) || request.URL.RawQuery != "" || request.URL.ForceQuery {
			http.Error(writer, "invalid watch progress request", http.StatusBadRequest)
			return
		}
		seconds, duration, found := lookup(request, id)
		if !found {
			http.NotFound(writer, request)
			return
		}

		seconds, duration = cleanWatchTime(seconds), cleanWatchTime(duration)
		if duration > 0 {
			seconds = min(seconds, duration)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("Cache-Control", "private, no-store")
		_ = json.NewEncoder(writer).Encode(struct {
			Seconds  float64 `json:"seconds"`
			Duration float64 `json:"duration"`
		}{seconds, duration})
	}
}

func cleanWatchTime(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 315360000 {
		return 0
	}
	return value
}

func validWatchProgressID(id string) bool {
	return len(id) != 0 && len(id) <= 128 && strings.IndexFunc(id, invalidWatchProgressRune) < 0
}

func invalidWatchProgressRune(r rune) bool {
	return (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' && r != '_'
}
