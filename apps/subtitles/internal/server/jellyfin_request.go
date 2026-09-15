package server

import "net/http"

const maxJellyfinRequestBytes = 1 << 20

func readJellyfinJSON(request *http.Request, target any) bool {
	return request.Body != nil && decodeExternalJSON(request.Body, maxJellyfinRequestBytes, target) == nil
}
