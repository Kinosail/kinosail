package server_test

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/database"
	"github.com/MikeO7/kinosail-player/internal/server"
)

func persistedProgressSeconds(t *testing.T, data, item string) float64 {
	t.Helper()
	store, err := database.Open(data, false)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var values map[string]struct {
		Seconds float64 `json:"seconds"`
	}
	if _, err := store.LoadJSON("progress.json", &values); err != nil {
		t.Fatal(err)
	}
	for key, value := range values {
		if strings.HasSuffix(key, ":"+item) {
			return value.Seconds
		}
	}
	return 0
}

func TestProgressHeartbeatsArePersistedAtShutdown(t *testing.T) {
	media, data := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Movie.mp4"), []byte("movie"), 0o600); err != nil {
		t.Fatal(err)
	}
	lifecycle, stop := context.WithCancel(context.Background())
	defer stop()
	handler := server.New(server.Config{Lifecycle: lifecycle, MediaDir: media, DataDir: data, CacheDir: t.TempDir(), RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	item := firstAPIItemID(t, handler, owner.Value)
	for _, seconds := range []int{10, 20, 30} {
		assertAPIBody(t, apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/items/"+item+"/progress", map[string]any{"seconds": seconds}), http.StatusOK)
	}
	if seconds := persistedProgressSeconds(t, data, item); seconds != 10 {
		t.Fatalf("heartbeats were not coalesced: persisted %v", seconds)
	}
	stop()
	deadline := time.Now().Add(10 * time.Second)
	for persistedProgressSeconds(t, data, item) != 30 {
		if time.Now().After(deadline) {
			t.Fatalf("last heartbeat was lost at shutdown: persisted %v", persistedProgressSeconds(t, data, item))
		}
		time.Sleep(20 * time.Millisecond)
	}
}
