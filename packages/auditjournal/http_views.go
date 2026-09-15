package auditjournal

import (
	"io"
	"net/http"
	"strconv"
)

type ActivityView[P any] struct {
	Events   []Event `json:"events"`
	Audit    []Event `json:"audit"`
	Playback []Event `json:"playback"`
	Progress P       `json:"progress"`
}

func (tracker *HTTPTracker) RecentLogs() []Event {
	if tracker == nil || tracker.journal == nil {
		return []Event{}
	}
	return tracker.journal.RecentLogs()
}

func (tracker *HTTPTracker) TripwireWarning() string {
	if tracker == nil || tracker.journal == nil {
		return ""
	}
	return tracker.journal.TripwireWarning()
}

func (tracker *HTTPTracker) Healthy() bool { return tracker.Status().Healthy }

func (tracker *HTTPTracker) Query(category string, limit int) []Event {
	if tracker == nil || tracker.journal == nil {
		return []Event{}
	}
	return tracker.journal.Query(category, limit)
}

func (tracker *HTTPTracker) Status() Status {
	if tracker == nil || tracker.journal == nil {
		return Status{}
	}
	return tracker.journal.Status()
}

func (tracker *HTTPTracker) WriteJSONL(writer io.Writer) error {
	if tracker == nil || tracker.journal == nil {
		return nil
	}
	return tracker.journal.WriteJSONL(writer)
}

func (tracker *HTTPTracker) Export(writer http.ResponseWriter) {
	if writer == nil {
		return
	}
	writer.Header().Set("Content-Type", "application/x-ndjson")
	writer.Header().Set("Content-Disposition", `attachment; filename="kinosail-activity.jsonl"`)
	_ = tracker.WriteJSONL(writer)
}

func ActivityAPI[P any](tracker *HTTPTracker, progress func() P, write func(http.ResponseWriter, any, int)) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		limit, _ := strconv.Atoi(request.URL.Query().Get("limit"))
		if limit == 0 {
			limit = 100
		}
		write(writer, ActivityView[P]{
			Events: tracker.Query(request.URL.Query().Get("category"), limit), Audit: tracker.Query("", limit),
			Playback: tracker.Query("playback", limit), Progress: progress(),
		}, http.StatusOK)
	}
}

func ExportAPI(tracker *HTTPTracker) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) { tracker.Export(writer) }
}
