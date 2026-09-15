package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/mediaprobe"
)

func (probe *mediaProbe) serveEmbedded(index *libraryIndex) http.HandlerFunc {
	return probe.core.EmbeddedHandler(mediaprobe.EmbeddedHTTPAdapter{
		Item:    func(request *http.Request, id string) (library.Item, bool) { return visibleItem(request, index, id) },
		Options: probe.embeddedOptions,
		Error:   localizedError,
	}, hlsPolicy())
}
