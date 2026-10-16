package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func BenchmarkCopiedHLSMasterPlaylist(b *testing.B) {
	for _, state := range []struct {
		name     string
		modified time.Duration
	}{{"verified-timeline-cached", -time.Hour}, {"verified-timeline-rehashed", time.Hour}} {
		b.Run(state.name, func(b *testing.B) {
			manager, item, recipe, directory, policy, timeline := copiedRecoveryFixture(b)
			writeHLSLoadingFile(b, filepath.Join(directory, "360p/segment-00000.m4s"), strings.Repeat("f", 16<<20))
			copiedRecoveryProbe(b, manager, "")
			if err := manager.bindCopiedHLSClock(b.Context(), item, recipe, directory, "360p/index.m3u8", policy, timeline); err != nil {
				b.Fatal(err)
			}
			modified := time.Now().Add(state.modified)
			for _, name := range []string{"init.mp4", "segment-00000.m4s"} {
				if err := os.Chtimes(filepath.Join(directory, "360p", name), modified, modified); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportAllocs()
			for b.Loop() {
				response := httptest.NewRecorder()
				manager.serveRecipe(response, httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/hls/"+item.ID+"/p/"+recipe.token()+"/index.m3u8", nil), item, recipe, "index.m3u8")
				if response.Code != http.StatusOK {
					b.Fatalf("master = %d %q", response.Code, response.Body.String())
				}
			}
		})
	}
}
