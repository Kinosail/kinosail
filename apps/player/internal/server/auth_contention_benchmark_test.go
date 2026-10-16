package server

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/database"
)

func BenchmarkAuthenticatedRequestsParallel(b *testing.B) {
	for _, variant := range []struct {
		name  string
		stale bool
	}{{"fresh-sessions", false}, {"minute-boundary-sessions", true}} {
		b.Run(variant.name, func(b *testing.B) {
			auth, tokens := benchmarkAuthentication(b, 2000, variant.stale)
			serveAuthenticatedRequests(b, auth, tokens)
		})
	}
}

func BenchmarkAuthenticatedRequestsDuringLogins(b *testing.B) {
	auth, tokens := benchmarkAuthentication(b, 200, false)
	stop := make(chan struct{})
	var logins sync.WaitGroup
	var count, total atomic.Int64
	for range 4 {
		logins.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				started := time.Now()
				auth.profiles.authenticate("Owner", "incorrect-password-value")
				total.Add(int64(time.Since(started)))
				count.Add(1)
			}
		})
	}
	serveAuthenticatedRequests(b, auth, tokens)
	close(stop)
	logins.Wait()
	if count.Load() > 0 {
		b.ReportMetric(float64(total.Load())/float64(count.Load())/float64(time.Millisecond), "login-ms")
	}
}

func serveAuthenticatedRequests(b *testing.B, auth *authentication, tokens []string) {
	b.Helper()
	handler := auth.protect(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusNoContent) }), func(*http.Request) string { return "GET /api/v1/library" })
	var next atomic.Uint64
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			request := httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/api/v1/library", nil)
			request.Header.Set("Authorization", "Bearer "+tokens[next.Add(1)%uint64(len(tokens))])
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusNoContent {
				b.Errorf("status = %d", recorder.Code)
				return
			}
		}
	})
}

func benchmarkAuthentication(b *testing.B, sessions int, stale bool) (*authentication, []string) {
	b.Helper()
	data := b.TempDir()
	stateDB, err := database.Open(data, false)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = stateDB.Close() })
	auth := newAuthentication(b.Context(), data, true, "", newSettingsStore("", data, "", stateDB), NotificationConfig{}, stateDB)
	owner, err := newProfile("Owner", "owner-password", true)
	if err != nil {
		b.Fatal(err)
	}
	owner.TOTPSecret = "JBSWY3DPEHPK3PXP"
	now := time.Now().Unix()
	seen := now
	if stale {
		seen = now - 61
	}
	tokens := make([]string, sessions)
	auth.profiles.mu.Lock()
	auth.profiles.profiles = []viewerProfile{owner}
	for index := range tokens {
		tokens[index] = "benchmark-session-" + strconv.Itoa(index)
		auth.profiles.sessions[sessionKey(tokens[index])] = viewerSession{ProfileID: owner.ID, Browser: true, CreatedAt: now, LastSeen: seen, ExpiresAt: now + 3600}
	}
	err = auth.profiles.persist(auth.profiles.sessionFile, auth.profiles.sessions)
	auth.profiles.mu.Unlock()
	if err != nil {
		b.Fatal(err)
	}
	return auth, tokens
}
