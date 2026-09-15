package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/library"
	markerlogic "github.com/MikeO7/kinosail/packages/markers"
)

func registerMarkerAdmin(mux *http.ServeMux, auth *authentication, index *libraryIndex, probe *mediaProbe, analyzer *markerAnalyzer) {
	markerlogic.RegisterAdmin(mux, markerlogic.Admin{
		Analyzer: analyzer, Owner: auth.owner, Snapshot: index.Snapshot, JSON: writeJSON, NotFound: localizedNotFound,
		Find: func(request *http.Request, id string) (library.Item, bool) { return visibleItem(request, index, id) },
		Duration: func(request *http.Request, item library.Item) float64 {
			return probe.inspect(request.Context(), item).Duration
		},
		Error: localizedError,
	})
}
