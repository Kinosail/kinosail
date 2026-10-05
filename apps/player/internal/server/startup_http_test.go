package server_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

// Registered HTTP routes own admission and delivery; the encoder is synthetic.
func TestPlaybackPreparationRejectsInvalidSourcesWithoutConsumingAdmission(t *testing.T) {
	h, ids, starts := preparationFixture(t, 4, true)
	page := apiCall(t, h, "", http.MethodGet, "/watch/"+ids[0], nil)
	session := regexp.MustCompile("data-playback-session=\"([a-zA-Z0-9_-]+)\"").FindStringSubmatch(page.Body.String())
	if len(session) != 2 {
		t.Fatal("missing playback session")
	}
	assertAPIBody(t, apiCall(t, h, "", http.MethodPost, "/api/v1/items/"+ids[0]+"/playback-events", map[string]any{"session": session[1], "event": "playing", "sequence": 1}), http.StatusNoContent)
	base, valid := "/api/v1/items/"+ids[0]+"/playback-prepare", preparationSource(ids[0])
	body := preparationJSON(valid)
	bad := []string{"{}", "null", "{\"source\":null}", "{\"source\":1}", "{\"source\":", body + " {}", strings.TrimSuffix(body, "}") + ",\"unknown\":true}", strings.TrimSuffix(body, "}") + ",\"source\":" + strconv.Quote(valid) + "}"}
	for _, source := range []string{"", "/media/other", "https://outside.test" + valid, valid + "?x=1", "/hls/" + ids[0] + "/p/t-a0-s0-none-t0-b0/segment-00000.m4s", "/hls/" + ids[0] + "/p/t-a99-s0-none-t0-b0/index.m3u8", "/hls/" + ids[0] + "/p/t-a0-s0-none-t9000-b0/index.m3u8", strings.Repeat("x", 2049)} {
		bad = append(bad, preparationJSON(source))
	}
	for _, input := range bad {
		r := preparationRaw(t, h, http.MethodPost, base, input)
		if r.Code != http.StatusBadRequest {
			t.Fatalf("invalid preparation: %d %s", r.Code, r.Body.String())
		}
	}
	assertAPIBody(t, preparationRaw(t, h, http.MethodPost, base+"?unknown=1", body), http.StatusBadRequest)
	assertAPIBody(t, preparationRaw(t, h, http.MethodPost, "/api/v1/items/missing/playback-prepare", preparationJSON("/media/missing")), http.StatusNotFound)
	assertPreparationNoEncoder(t, starts)
	// Rejection must leave all public admission slots available.
	for _, id := range ids[:3] {
		assertPreparationState(t, prepareSource(t, h, id, "/media/"+id), http.StatusAccepted, "queued")
	}
	assertPreparationState(t, prepareSource(t, h, ids[3], "/media/"+ids[3]), http.StatusTooManyRequests, "busy")
	assertPreparationState(t, prepareSource(t, h, ids[0], "/media/"+ids[0]), http.StatusAccepted, "queued")
	assertAPIBody(t, preparationRaw(t, h, http.MethodDelete, base+"?unknown=1", ""), http.StatusBadRequest)
	assertPreparationState(t, prepareSource(t, h, ids[3], "/media/"+ids[3]), http.StatusTooManyRequests, "busy")
	assertAPIBody(t, preparationRaw(t, h, http.MethodDelete, base, ""), http.StatusNoContent)
	assertPreparationState(t, prepareSource(t, h, ids[3], "/media/"+ids[3]), http.StatusAccepted, "queued")
	assertPreparationNoEncoder(t, starts)
}

func TestPlaybackPreparationProducesAReusableStartupWindow(t *testing.T) {
	h, ids, starts := preparationFixture(t, 1, true)
	id, source := ids[0], preparationSource(ids[0])
	assertPreparationState(t, prepareSource(t, h, id, source), http.StatusAccepted, "queued")
	deadline := time.Now().Add(6 * time.Second)
	for {
		r := prepareSource(t, h, id, source)
		if strings.Contains(r.Body.String(), "\"ready\"") {
			assertPreparationState(t, r, http.StatusAccepted, "ready")
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("window never ready: %d %s", r.Code, r.Body.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	if data, err := os.ReadFile(starts); err != nil || len(data) == 0 {
		t.Fatal("encoder never started")
	}
	master := apiCall(t, h, "", http.MethodGet, source, nil)
	assertAPIBody(t, master, http.StatusOK, "#EXTM3U")
	count := 0
	for _, line := range strings.Split(master.Body.String(), "\n") {
		if !strings.HasSuffix(line, "/index.m3u8") {
			continue
		}
		count++
		rendition := strings.TrimSuffix(source, "index.m3u8") + line
		assertAPIBody(t, apiCall(t, h, "", http.MethodGet, rendition, nil), http.StatusOK, "segment-00000.m4s", "segment-00001.m4s")
		for _, asset := range []string{"init.mp4", "segment-00000.m4s", "segment-00001.m4s"} {
			r := apiCall(t, h, "", http.MethodGet, strings.TrimSuffix(rendition, "index.m3u8")+asset, nil)
			if r.Code != http.StatusOK || r.Body.Len() == 0 {
				t.Fatalf("asset %s: %d", asset, r.Code)
			}
		}
	}
	if count == 0 {
		t.Fatal("no delivered renditions")
	}
}

func TestPlaybackPreparationRequiresCompatibleCacheButAllowsDirect(t *testing.T) {
	h, ids, starts := preparationFixture(t, 1, false)
	assertAPIBody(t, prepareSource(t, h, ids[0], preparationSource(ids[0])), http.StatusServiceUnavailable)
	assertPreparationState(t, prepareSource(t, h, ids[0], "/media/"+ids[0]), http.StatusAccepted, "queued")
	assertPreparationNoEncoder(t, starts)
}

func preparationFixture(t *testing.T, count int, caching bool) (http.Handler, []string, string) {
	t.Helper()
	media, tools, cache := t.TempDir(), t.TempDir(), ""
	if caching {
		cache = t.TempDir()
	}
	for index := range count {
		if err := os.WriteFile(filepath.Join(media, fmt.Sprintf("Preparation %d.mp4", index)), []byte("synthetic media"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	probe, encoder, starts := filepath.Join(tools, "ffprobe"), filepath.Join(tools, "ffmpeg"), filepath.Join(tools, "starts")
	servertest.WriteExecutable(t, probe, "#!/bin/sh\nprintf '%s' '{\"streams\":[{\"index\":0,\"codec_type\":\"video\",\"codec_name\":\"h264\",\"width\":640,\"height\":360},{\"index\":1,\"codec_type\":\"audio\",\"codec_name\":\"aac\"}],\"format\":{\"format_name\":\"mp4\",\"duration\":\"8\"}}'\n")
	servertest.WriteExecutable(t, encoder, "#!/bin/sh\nprintf 'started\\n' >> '"+starts+"'\n"+servertest.PlayableHLS())
	h := server.New(server.Config{Lifecycle: t.Context(), MediaDir: media, DataDir: t.TempDir(), CacheDir: cache, FFprobe: probe, FFmpeg: encoder})
	listing := apiCall(t, h, "", http.MethodGet, "/api/v1/library", nil)
	var library struct{ Items []struct{ ID string } }
	if listing.Code != http.StatusOK || json.Unmarshal(listing.Body.Bytes(), &library) != nil || len(library.Items) != count {
		t.Fatalf("preparation library = %d %s", listing.Code, listing.Body.String())
	}
	var ids []string
	for _, item := range library.Items {
		ids = append(ids, item.ID)
	}
	return h, ids, starts
}
func preparationSource(id string) string { return "/hls/" + id + "/p/t-a0-s0-none-t0-b0/index.m3u8" }
func preparationJSON(source string) string {
	body, _ := json.Marshal(map[string]string{"source": source})
	return string(body)
}
func prepareSource(t *testing.T, h http.Handler, id, source string) *httptest.ResponseRecorder {
	t.Helper()
	return apiCall(t, h, "", http.MethodPost, "/api/v1/items/"+id+"/playback-prepare", map[string]string{"source": source})
}
func preparationRaw(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	r := httptest.NewRecorder()
	h.ServeHTTP(r, request)
	return r
}
func assertPreparationState(t *testing.T, r *httptest.ResponseRecorder, status int, state string) {
	t.Helper()
	assertAPIBody(t, r, status, "\"state\":"+strconv.Quote(state))
	if r.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("preparation response was cacheable")
	}
}
func assertPreparationNoEncoder(t *testing.T, starts string) {
	t.Helper()
	if data, err := os.ReadFile(starts); !os.IsNotExist(err) {
		t.Fatalf("unexpected encoder: %q %v", data, err)
	}
}
