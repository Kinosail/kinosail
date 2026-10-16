package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCopiedHLSMasterJoinsConcurrentClockBinding(t *testing.T) {
	for name, modified := range map[string]time.Time{"normal": time.Now(), "past": time.Unix(946684800, 0), "future": time.Unix(4039365600, 0)} {
		t.Run(name, func(t *testing.T) {
			manager, item, recipe, directory, policy, _ := copiedRecoveryFixture(t, modified)
			copiedRecoveryProbe(t, manager, "sleep 0.3")
			binding := make(chan error, 1)
			go func() { binding <- manager.ensureCopiedHLSClock(t.Context(), item, recipe, directory, policy) }()
			time.Sleep(50 * time.Millisecond)
			response := httptest.NewRecorder()
			manager.serveRecipe(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/"+item.ID+"/p/"+recipe.token()+"/index.m3u8", nil), item, recipe, "index.m3u8")
			if err := <-binding; err != nil {
				t.Fatalf("startup clock binding = %v", err)
			}
			if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "360p/index.m3u8") {
				t.Fatalf("master during clock binding = %d %q", response.Code, response.Body.String())
			}
		})
	}
}
