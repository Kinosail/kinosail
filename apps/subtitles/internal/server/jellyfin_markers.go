package server

import (
	"net/http"

	markerlogic "github.com/MikeO7/kinosail/packages/markers"
)

func (api *jellyfinAPI) jellyfinMarkers(request *http.Request) (string, []markerlogic.Marker, bool) {
	item, found := visibleItem(request, api.index, jellyfinRawID(request.PathValue("id")))
	if !found {
		return "", nil, false
	}
	media := api.probe.inspect(request.Context(), item)
	return item.ID, api.jellyfinMarkerTimeline(request, media), true
}
