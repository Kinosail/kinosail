package server

import "net/http"

const (
	maxJellyfinRequestBytes     = 1 << 20
	maxJellyfinStreamingBitrate = 1_000_000_000_000
)

func readJellyfinJSON(request *http.Request, target any) bool {
	return request.Body != nil && decodeExternalJSON(request.Body, maxJellyfinRequestBytes, target) == nil
}
