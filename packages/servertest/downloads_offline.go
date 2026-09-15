package servertest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TranscodedDownloadBecomesIntegrityCheckedReadyOfflineFile verifies conversion, integrity, range resume, UI state, and restart persistence.
func (fixture DownloadFixture) TranscodedDownloadBecomesIntegrityCheckedReadyOfflineFile(t *testing.T) { //nolint:cyclop,funlen // One end-to-end lifecycle proves the complete offline contract.
	media, data, cache, tools := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mkv"), []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffmpeg, release := filepath.Join(tools, "ffmpeg"), filepath.Join(tools, "release")
	writeTestExecutable(t, ffmpeg, fmt.Sprintf("#!/bin/sh\nwhile [ ! -e %q ]; do sleep .01; done\nfor output; do :; done\ncase $output in *.mp4|*.m4a) ;; *) exit 234;; esac\nprintf 'optimized-media' > \"$output\"\n", release))
	WriteExecutable(t, filepath.Join(tools, "ffprobe"), `#!/bin/sh
printf '%s' '{"streams":[{"codec_type":"video","codec_name":"h264","width":1920,"height":1080},{"codec_type":"audio","codec_name":"aac","index":1}],"format":{"duration":"120"}}'
`)
	handler := fixture.NewTranscodeHandler(media, data, cache, ffmpeg)
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	token := owner.Value
	var catalog struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	MustJSON(t, fixture.APICall(t, handler, token, http.MethodGet, "/api/v1/library", nil), &catalog)
	started := fixture.APICall(t, handler, token, http.MethodPost, "/api/v1/items/"+catalog.Items[0].ID+"/downloads", map[string]any{"quality": "720p"})
	if started.Code != http.StatusAccepted {
		t.Fatalf("start = %d %q", started.Code, started.Body.String())
	}
	pending := fixture.CookieRequest(t, handler, http.MethodGet, "/offline-downloads", "", owner)
	AssertAPIBody(t, pending, http.StatusOK, `data-downloads-pending="true"`, fixture.DownloadsScript, `"includeIndicatorStyles":false`, `role="status"`, `<progress`, `Preparing Film for offline use`)
	if strings.Contains(pending.Body.String(), `http-equiv="refresh"`) {
		t.Fatalf("downloads page uses a timed full-page refresh: %q", pending.Body.String())
	}
	if err := os.WriteFile(release, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	var job struct {
		ID        string `json:"id"`
		ProfileID string `json:"profileId"`
	}
	MustJSON(t, started, &job)
	var status *httptest.ResponseRecorder
	for range 500 {
		status = fixture.APICall(t, handler, token, http.MethodGet, "/api/v1/downloads/"+job.ID, nil)
		if strings.Contains(status.Body.String(), `"state":"ready"`) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	AssertAPIBody(t, status, http.StatusOK, `"state":"ready"`, `"sha256":`, `"size":15`, `"readyOffline":true`)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/downloads/"+job.ID+"/file", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Range", "bytes=2-5")
	file := httptest.NewRecorder()
	handler.ServeHTTP(file, request)
	if file.Code != http.StatusPartialContent || file.Body.String() != "timi" || file.Header().Get("ETag") == "" || file.Header().Get("Content-Digest") != "" || file.Header().Get("Repr-Digest") == "" {
		t.Fatalf("range = %d %q %q", file.Code, file.Body.String(), file.Header())
	}
	page := fixture.CookieRequest(t, handler, http.MethodGet, "/offline-downloads", "", owner)
	AssertAPIBody(t, page, http.StatusOK, "Film", "Ready to download", "Downloads on this device", "Play offline", `data-download-play`, "File verified on the Server", "15 bytes", "720p", `data-viewer-profile="`+job.ProfileID+`"`)
	player := fixture.CookieRequest(t, handler, http.MethodGet, "/watch/"+catalog.Items[0].ID, "", owner)
	AssertAPIBody(t, player, http.StatusOK, `data-viewer-profile="`+job.ProfileID+`"`, fixture.DownloadsScript)
	if strings.Contains(page.Body.String(), `data-downloads-pending="true"`) {
		t.Fatalf("completed downloads page remains pending: %q", page.Body.String())
	}
	handler = fixture.NewTranscodeHandler(media, data, cache, ffmpeg)
	restored := fixture.APICall(t, handler, token, http.MethodGet, "/api/v1/downloads/"+job.ID, nil)
	var decoded map[string]any
	if json.Unmarshal(restored.Body.Bytes(), &decoded) != nil || decoded["state"] != "ready" {
		t.Fatalf("restored job = %d %q", restored.Code, restored.Body.String())
	}
}

// RestartDoesNotTrustMissingOrChangedReadyDownload verifies persisted integrity before serving bytes.
func (fixture DownloadFixture) RestartDoesNotTrustMissingOrChangedReadyDownload(t *testing.T) {
	media, data, cache := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mp4"), []byte("original-media"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := fixture.NewHandler(media, data, cache)
	cookie := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	jobID := fixture.startAndWaitForDownload(t, handler, cookie.Value, fixture.FirstItemID(t, handler, cookie.Value))
	files, err := filepath.Glob(filepath.Join(cache, "downloads", "*.mp4"))
	if err != nil || len(files) != 1 {
		t.Fatalf("download files = %v, %v", files, err)
	}
	if err := os.WriteFile(files[0], []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	restarted := fixture.NewHandler(media, data, cache)
	status := fixture.APICall(t, restarted, cookie.Value, http.MethodGet, "/api/v1/downloads/"+jobID, nil)
	for deadline := time.Now().Add(3 * time.Second); strings.Contains(status.Body.String(), `"state":"preparing"`) && time.Now().Before(deadline); {
		AssertAPIBody(t, status, http.StatusOK, `"readyOffline":false`)
		if file := fixture.APICall(t, restarted, cookie.Value, http.MethodGet, "/api/v1/downloads/"+jobID+"/file", nil); file.Code != http.StatusServiceUnavailable && file.Code != http.StatusNotFound {
			t.Fatalf("unverified file served = %d", file.Code)
		}
		time.Sleep(10 * time.Millisecond)
		status = fixture.APICall(t, restarted, cookie.Value, http.MethodGet, "/api/v1/downloads/"+jobID, nil)
	}
	AssertAPIBody(t, status, http.StatusOK, `"state":"failed"`, `"readyOffline":false`, "integrity")
	if file := fixture.APICall(t, restarted, cookie.Value, http.MethodGet, "/api/v1/downloads/"+jobID+"/file", nil); file.Code != http.StatusNotFound {
		t.Fatalf("changed file served = %d %q", file.Code, file.Body.String())
	}
}

func (fixture DownloadFixture) startAndWaitForDownload(t *testing.T, handler http.Handler, token, itemID string) string {
	t.Helper()
	started := fixture.APICall(t, handler, token, http.MethodPost, "/api/v1/items/"+itemID+"/downloads", map[string]any{"quality": "original"})
	var job struct{ ID string }
	MustJSON(t, started, &job)
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		status := fixture.APICall(t, handler, token, http.MethodGet, "/api/v1/downloads/"+job.ID, nil)
		if strings.Contains(status.Body.String(), `"state":"ready"`) {
			return job.ID
		}
	}
	t.Fatalf("download %s did not become ready", job.ID)
	return ""
}

func writeTestExecutable(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o700); err != nil { //nolint:gosec // This fixture must be executable and uses a private temporary path.
		t.Fatal(err)
	}
}
