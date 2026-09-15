package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/library"
)

func (probe *mediaProbe) writeEmbedded(writer http.ResponseWriter, request *http.Request, item library.Item, stream int) {
	if message, status := probe.core.ServeEmbedded(writer, request, item, stream, hlsPolicy(), probe.embeddedOptions()); status != 0 {
		localizedError(writer, request, message, status)
	}
}
