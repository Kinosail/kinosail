package httpguard

import (
	"net/http"
	"time"
)

// WritePublicError writes one app-specific public error response.
type WritePublicError func(http.ResponseWriter, *http.Request, string, int)

// GuardPublicRequests quarantines repeated scanner and credential abuse.
func GuardPublicRequests(next http.Handler, public func(*http.Request) bool, writeError WritePublicError, writeNotFound func(http.ResponseWriter, *http.Request), audit func(*http.Request, string, bool)) http.Handler {
	tripwire := &PublicTripwire{}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !public(request) {
			next.ServeHTTP(writer, request)
			return
		}
		key, now := RemoteHost(request.RemoteAddr), time.Now()
		if tripwire.Blocked(key, now) {
			writer.Header().Set("Retry-After", "900")
			writeError(writer, request, "public source is temporarily quarantined", http.StatusTooManyRequests)
			return
		}
		if SuspiciousPublicPath(request.URL.Path) {
			if tripwire.Strike(key, now, 3) {
				audit(request, "scanner probe", true)
			}
			writeNotFound(writer, request)
			return
		}
		if !CredentialAttempt(request) {
			next.ServeHTTP(writer, request)
			return
		}
		capture := &statusWriter{ResponseWriter: writer}
		next.ServeHTTP(capture, request)
		if (capture.status == http.StatusUnauthorized || capture.status == http.StatusForbidden) && tripwire.Strike(key, now, 10) {
			audit(request, "repeated credential failure", true)
		}
	})
}

// LimitPublicRequests rejects unsafe bodies, ranges, and excess concurrency.
func LimitPublicRequests(next http.Handler, public func(*http.Request) bool, writeError WritePublicError) http.Handler {
	capacity := NewPublicCapacity()
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !public(request) {
			next.ServeHTTP(writer, request)
			return
		}
		if (request.Method == http.MethodGet || request.Method == http.MethodHead) && (request.ContentLength != 0 || len(request.TransferEncoding) != 0) {
			http.Error(writer, "request body is not allowed", http.StatusBadRequest)
			return
		}
		if (request.Method == http.MethodGet || request.Method == http.MethodHead) && !ValidPublicRange(request.Header.Values("Range")) {
			http.Error(writer, "requested range is not satisfiable", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		key := RemoteHost(request.RemoteAddr)
		if !capacity.Acquire(key) {
			writer.Header().Set("Retry-After", "1")
			writeError(writer, request, "public request capacity is temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		defer capacity.Release(key)
		next.ServeHTTP(writer, request)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (writer *statusWriter) WriteHeader(status int) {
	if writer.status != 0 {
		return
	}
	writer.status = status
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *statusWriter) Write(data []byte) (int, error) {
	if writer.status == 0 {
		writer.WriteHeader(http.StatusOK)
	}
	return writer.ResponseWriter.Write(data)
}

func (writer *statusWriter) Unwrap() http.ResponseWriter { return writer.ResponseWriter }
