package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPublicPlaybackCannotReuseLANSessionCapability(t *testing.T) {
	api := &jellyfinAPI{index: &libraryIndex{}, settings: &settingsStore{}, probe: &mediaProbe{}, hls: &hlsManager{}, downloads: &downloadManager{}}
	api.plays.Store("local-play", jellyfinPlaySession{itemID: "item", profileID: "viewer", expires: time.Now().Add(time.Hour)})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/Videos/item/stream?playSessionId=local-play", nil)
	request.SetPathValue("id", "item")
	allowed := true
	Remote(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		session, valid := api.deliveryModule().PlaySession(request, "item")
		_, concrete := session.(jellyfinPlaySession)
		allowed = valid && concrete
	})).ServeHTTP(httptest.NewRecorder(), request)
	if allowed {
		t.Fatal("LAN playback capability was accepted on the public listener")
	}
}
