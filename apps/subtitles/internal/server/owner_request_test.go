package server

import (
	"context"
	"net/http"
	"net/http/httptest"
)

func ownerRequest(target string) *http.Request {
	request := httptest.NewRequestWithContext(context.Background(), "GET", target, nil)
	return request.WithContext(context.WithValue(request.Context(), viewerContextKey{}, viewerProfile{ID: "owner", Owner: true}))
}
