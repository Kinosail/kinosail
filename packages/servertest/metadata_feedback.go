package servertest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// RunAutomaticMetadataFeedback verifies real scan/provider behavior when
// optional credits fail, remain unchanged, and later become available.
func RunAutomaticMetadataFeedback(t *testing.T, newServer func(context.Context, string, string, string) http.Handler) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var credits atomic.Int32
	var available atomic.Bool
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/movie/101/credits" {
			if credits.Add(1) > 12 {
				<-request.Context().Done()
				return
			}
			if available.Load() {
				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write([]byte(`{"cast":[]}`))
			} else {
				http.Error(writer, "optional credits unavailable", http.StatusServiceUnavailable)
			}
			return
		}
		MetadataProviderFixture(writer, request)
	}))
	t.Cleanup(provider.Close)
	media, cache := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Arrival.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := newServer(ctx, media, cache, provider.URL)
	scan := func() {
		t.Helper()
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(ctx, http.MethodPost, "/settings/tasks/scan", nil))
		if response.Code != http.StatusSeeOther {
			t.Fatalf("scan = %d %s", response.Code, response.Body.String())
		}
	}
	waitMetadataCredits(t, &credits, 1)
	before := settleMetadataCredits(t, &credits, 0)
	scan()
	waitMetadataCredits(t, &credits, before+1)
	before = settleMetadataCredits(t, &credits, before)
	available.Store(true)
	scan()
	waitMetadataCredits(t, &credits, before+1)
	settleMetadataCredits(t, &credits, before)
}

func waitMetadataCredits(t *testing.T, credits *atomic.Int32, want int32) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for credits.Load() < want && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if credits.Load() < want {
		t.Fatalf("credits requests = %d, want at least %d", credits.Load(), want)
	}
}

func settleMetadataCredits(t *testing.T, credits *atomic.Int32, before int32) int32 {
	t.Helper()
	time.Sleep(250 * time.Millisecond)
	got := credits.Load()
	if got > before+2 {
		t.Fatalf("unchanged metadata retriggered itself: %d credits requests after %d", got, before)
	}
	return got
}
