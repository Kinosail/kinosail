package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type requestActivityKey struct{}

type requestActivity struct {
	id               string
	playbackSession  string
	viewer           viewerProfile
	securityRecorded bool
}

func observeRequests(activity *auditStore, route func(*http.Request) string, next http.Handler) http.Handler { //nolint:contextcheck // Recovery and final logging intentionally retain the request context.
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		facts := &requestActivity{id: acceptedRequestID(request.Header.Get("X-Request-ID")), playbackSession: acceptedPlaybackSession(request)}
		request = request.WithContext(context.WithValue(request.Context(), requestActivityKey{}, facts))
		writer.Header().Set("X-Request-ID", facts.id)
		capture := &auditWriter{ResponseWriter: writer}
		observation := requestObservation{activity: activity, route: route, request: request, facts: facts, capture: capture, started: time.Now()}
		defer observation.finish(request.Context()) //nolint:contextcheck // The deferred audit uses the request context created above.
		next.ServeHTTP(capture, request)
	})
}

func setAuditViewer(request *http.Request, viewer viewerProfile) {
	if facts, ok := request.Context().Value(requestActivityKey{}).(*requestActivity); ok {
		facts.viewer = viewer
	}
}

func auditViewer(request *http.Request) viewerProfile {
	if facts, ok := request.Context().Value(requestActivityKey{}).(*requestActivity); ok {
		return facts.viewer
	}
	return viewerProfile{}
}

func activityRequestID(request *http.Request) string {
	return requestActivityID(request.Context())
}

func requestActivityID(ctx context.Context) string {
	if facts, ok := ctx.Value(requestActivityKey{}).(*requestActivity); ok {
		return facts.id
	}
	return ""
}

func setPlaybackSession(request *http.Request, session string) {
	if facts, ok := request.Context().Value(requestActivityKey{}).(*requestActivity); ok && validPlaybackSession(session) {
		facts.playbackSession = session
	}
}

func requestPlaybackSession(ctx context.Context) string {
	if facts, ok := ctx.Value(requestActivityKey{}).(*requestActivity); ok {
		return facts.playbackSession
	}
	return ""
}

func acceptedPlaybackSession(request *http.Request) string {
	for _, value := range []string{request.Header.Get("X-Playback-Session"), request.URL.Query().Get("playbackSession")} {
		if validPlaybackSession(value) {
			return value
		}
	}
	return ""
}

func validPlaybackSession(value string) bool {
	return len(value) >= 8 && len(value) <= 64 && strings.IndexFunc(value, func(character rune) bool {
		return character != '-' && character != '_' && (character < '0' || character > '9') && (character < 'A' || character > 'Z') && (character < 'a' || character > 'z')
	}) == -1
}

func markSecurityRecorded(request *http.Request) {
	if facts, ok := request.Context().Value(requestActivityKey{}).(*requestActivity); ok {
		facts.securityRecorded = true
	}
}

func acceptedRequestID(value string) string { //nolint:cyclop // The inline allowlist makes accepted request-ID characters explicit.
	if value != "" && len(value) <= 64 && strings.IndexFunc(value, func(character rune) bool {
		return character != '-' && character != '_' && (character < '0' || character > '9') && (character < 'A' || character > 'Z') && (character < 'a' || character > 'z')
	}) == -1 {
		return value
	}
	return randID()
}

func randID() string {
	data := make([]byte, 12)
	if _, err := rand.Read(data); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return hex.EncodeToString(data)
}

func remoteIP(request *http.Request) string {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err == nil {
		return host
	}
	return truncate(request.RemoteAddr, 64)
}
