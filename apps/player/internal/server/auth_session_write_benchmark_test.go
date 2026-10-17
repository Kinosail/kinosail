package server

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func BenchmarkAuthenticatedRequestsDuringSessionWrites(b *testing.B) {
	auth, tokens := benchmarkAuthentication(b, 2000, false)
	ownerID := auth.profiles.profiles[0].ID
	stop := make(chan struct{})
	var writers sync.WaitGroup
	var count, total atomic.Int64
	for range 4 {
		writers.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				started := time.Now()
				token, err := auth.profiles.createSessionKind(ownerID, "Benchmark", true, false)
				if err != nil {
					b.Error(err)
					return
				}
				request := httptest.NewRequestWithContext(b.Context(), http.MethodDelete, "/api/v1/session", nil)
				request.Header.Set("Authorization", "Bearer "+token)
				if err := auth.profiles.signOut(request); err != nil {
					b.Error(err)
					return
				}
				total.Add(int64(time.Since(started)))
				count.Add(2)
			}
		})
	}
	serveAuthenticatedRequests(b, auth, tokens)
	close(stop)
	writers.Wait()
	if count.Load() > 0 {
		b.ReportMetric(float64(total.Load())/float64(count.Load())/float64(time.Millisecond), "write-ms")
	}
}
