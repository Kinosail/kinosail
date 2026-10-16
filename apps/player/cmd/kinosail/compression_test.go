package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/commandtest"
	"github.com/MikeO7/kinosail/packages/servertest"
)

type subtitleClient struct {
	server *httptest.Server
	host   string
	token  string
}

func (client subtitleClient) call(t *testing.T, method, path string, body any, header http.Header) (int, http.Header, []byte) {
	t.Helper()
	var payload bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&payload).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	request, err := http.NewRequestWithContext(t.Context(), method, client.server.URL+path, &payload)
	if err != nil {
		t.Fatal(err)
	}
	request.Host = client.host
	request.Header.Set("Content-Type", "application/json")
	if client.token != "" {
		request.Header.Set("Authorization", "Bearer "+client.token)
	}
	for name, values := range header {
		request.Header[name] = values
	}
	transport := &http.Transport{DisableCompression: true}
	defer transport.CloseIdleConnections()
	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, response.Header, data
}

func TestConfiguredServerCompressesSubtitleTracks(t *testing.T) {
	media := t.TempDir()
	var track strings.Builder
	for cue := range 200 {
		fmt.Fprintf(&track, "%d\n00:%02d:%02d,000 --> 00:%02d:%02d,500\nThe tide turns at line %d.\n\n", cue+1, cue/60, cue%60, cue/60, cue%60, cue)
	}
	if err := os.WriteFile(filepath.Join(media, "Arrival.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(media, "Arrival.en.srt"), []byte(track.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	webVTT := "WEBVTT\n\nNOTE stored WebVTT file\n\n" + strings.ReplaceAll(track.String(), ",", ".")
	if err := os.WriteFile(filepath.Join(media, "Arrival.fr.vtt"), []byte(webVTT), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KINOSAIL_DATA_DIR", t.TempDir())
	t.Setenv("KINOSAIL_CACHE_DIR", t.TempDir())
	t.Setenv("KINOSAIL_MEDIA_DIR", media)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	httpServer := commandtest.HTTPFactory(loadConfiguration, newConfiguredHTTPServerWithAccess)(ctx)
	client := subtitleClient{server: httptest.NewServer(httpServer.Handler), host: httpServer.Addr}
	defer client.server.Close()
	var owner struct {
		Token string
		TOTP  struct{ Secret string }
	}
	status, _, body := client.call(t, http.MethodPost, "/api/v1/setup", map[string]any{"name": "Owner", "password": "owner-password", "totp": true}, nil)
	if err := json.Unmarshal(body, &owner); err != nil || owner.Token == "" {
		t.Fatalf("setup = %d %q", status, body)
	}
	client.token = owner.Token
	if status, _, body = client.call(t, http.MethodPut, "/api/v1/me/mfa", map[string]any{"code": servertest.TestTOTP(t, owner.TOTP.Secret, time.Now())}, nil); status != http.StatusOK {
		t.Fatalf("MFA = %d %q", status, body)
	}
	if status, _, body = client.call(t, http.MethodPost, "/api/v1/tasks/scan", nil, nil); status != http.StatusNoContent {
		t.Fatalf("scan = %d %q", status, body)
	}
	var library struct{ Items []struct{ ID string } }
	if _, _, body = client.call(t, http.MethodGet, "/api/v1/library", nil, nil); json.Unmarshal(body, &library) != nil || len(library.Items) != 1 {
		t.Fatalf("library = %q", body)
	}
	path := "/subtitle/" + library.Items[0].ID
	status, header, plain := client.call(t, http.MethodGet, path, nil, http.Header{"Accept-Encoding": {"identity"}})
	if status != http.StatusOK || header.Get("Content-Encoding") != "" || !bytes.HasPrefix(plain, []byte("WEBVTT")) || len(plain) < 1024 {
		t.Fatalf("identity subtitle = %d %v %.80q", status, header, plain)
	}
	status, header, compressed := client.call(t, http.MethodGet, path, nil, http.Header{"Accept-Encoding": {"gzip"}})
	if status != http.StatusOK || header.Get("Content-Encoding") != "gzip" || !strings.Contains(strings.Join(header.Values("Vary"), ","), "Accept-Encoding") || len(compressed) >= len(plain)/2 {
		t.Fatalf("compressed subtitle = %d %v, %d of %d bytes", status, header, len(compressed), len(plain))
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := io.ReadAll(reader)
	if err != nil || !bytes.Equal(decoded, plain) {
		t.Fatalf("decoded subtitle differs: %v", err)
	}
	t.Logf("subtitle track: identity %d bytes, gzip %d bytes", len(plain), len(compressed))
	if _, header, _ = client.call(t, http.MethodGet, path, nil, http.Header{"Accept-Encoding": {"gzip"}, "Range": {"bytes=0-99"}}); header.Get("Content-Encoding") != "" {
		t.Fatalf("range subtitle = %v", header)
	}
	assertStoredWebVTTDelivery(t, client, "/subtitle/"+library.Items[0].ID, []byte(webVTT))
}

func assertStoredWebVTTDelivery(t *testing.T, client subtitleClient, base string, webVTT []byte) {
	t.Helper()
	for track := range 2 {
		path := fmt.Sprintf("%s/%d", base, track)
		if _, _, plain := client.call(t, http.MethodGet, path, nil, http.Header{"Accept-Encoding": {"identity"}}); !bytes.Equal(plain, webVTT) {
			continue
		}
		status, header, partial := client.call(t, http.MethodGet, path, nil, http.Header{"Accept-Encoding": {"gzip"}, "Range": {"bytes=0-99"}})
		if status != http.StatusPartialContent || header.Get("Content-Encoding") != "" || !bytes.Equal(partial, webVTT[:100]) {
			t.Fatalf("stored WebVTT range = %d %v %q", status, header, partial)
		}
		status, header, compressed := client.call(t, http.MethodGet, path, nil, http.Header{"Accept-Encoding": {"gzip"}})
		reader, err := gzip.NewReader(bytes.NewReader(compressed))
		if status != http.StatusOK || header.Get("Content-Encoding") != "gzip" || err != nil {
			t.Fatalf("stored WebVTT gzip = %d %v %v", status, header, err)
		}
		if decoded, err := io.ReadAll(reader); err != nil || !bytes.Equal(decoded, webVTT) {
			t.Fatalf("stored WebVTT decoded differently: %v", err)
		}
		return
	}
	t.Fatal("no subtitle track served the stored WebVTT file")
}
