package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func BenchmarkCopiedHLSCachedSegmentRequest(b *testing.B) {
	manager, item, recipe, directory, policy, timeline := copiedRecoveryFixture(b)
	copiedRecoveryProbe(b, manager, "")
	if err := manager.bindCopiedHLSClock(b.Context(), item, recipe, directory, "360p/index.m3u8", policy, timeline); err != nil {
		b.Fatal(err)
	}
	settled := time.Now().Add(-time.Hour)
	for _, name := range []string{"init.mp4", "segment-00000.m4s"} {
		if err := os.Chtimes(filepath.Join(directory, "360p", name), settled, settled); err != nil {
			b.Fatal(err)
		}
	}
	name := "360p/segment-00001.m4s"
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			response := httptest.NewRecorder()
			manager.serveRecipe(response, httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/hls/"+item.ID+"/p/"+recipe.token()+"/"+name, nil), item, recipe, name)
			if response.Code != http.StatusOK || response.Body.String() != "last fragment" {
				b.Errorf("segment = %d %q", response.Code, response.Body.String())
				return
			}
		}
	})
}
