package servertest

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type ObservedAuditHandler func(*testing.T, string, http.Handler) http.Handler

func RequestLoggingRecoversPanicsAndRedactsURLs(t *testing.T, observe ObservedAuditHandler, setViewer func(*http.Request)) {
	t.Helper()
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	handler := observe(t, "GET /media/{id}", http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		setViewer(request)
		panic("boom")
	}))
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/media/item?api_key=secret", nil)
	request.RemoteAddr = "192.0.2.1:1234"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError || response.Header().Get("X-Request-ID") == "" || !strings.Contains(output.String(), `"status":500`) || !strings.Contains(output.String(), `"panic":"boom"`) {
		t.Fatalf("response=%d headers=%v log=%s", response.Code, response.Header(), output.String())
	}
	for _, private := range []string{"secret", "api_key", "/media/item", "private-profile-id", "Private Viewer", "192.0.2.1"} {
		if strings.Contains(output.String(), private) {
			t.Fatalf("request log exposed %q: %s", private, output.String())
		}
	}
	if !strings.Contains(output.String(), `"route":"GET /media/{id}"`) {
		t.Fatalf("request log exposed query: %s", output.String())
	}
}

func UnknownJellyfinRouteLoggingKeepsOnlySafeRouteSegments(t *testing.T, observe ObservedAuditHandler) {
	t.Helper()
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	handler := observe(t, "", http.NotFoundHandler())
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/Playback/BitrateTest/0123456789abcdef?api_key=secret", nil)
	request.Header.Set("Authorization", `MediaBrowser Client="Swiftfin", Device="iPhone"`)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound || !strings.Contains(output.String(), `"compatibility_path":"/Playback/BitrateTest/{value}"`) {
		t.Fatalf("unknown Jellyfin route log = %s", output.String())
	}
	for _, private := range []string{"0123456789abcdef", "api_key", "secret"} {
		if strings.Contains(output.String(), private) {
			t.Fatalf("unknown Jellyfin route log exposed %q: %s", private, output.String())
		}
	}
}

func JellyfinStreamShapeDistinguishesSafeHLSPaths(t *testing.T, shape func(string) string) {
	t.Helper()
	for path, want := range map[string]string{
		"/Videos/item/stream":                                  "direct",
		"/Videos/item/p/t-a0-s0-none-t0-b0/index.m3u8":         "planned-master",
		"/Videos/item/p/t-a0-s0-none-t0-b0/360p/index.m3u8":    "planned-variant",
		"/Videos/item/360p/index.m3u8":                         "session-variant",
		"/Videos/item/360p/init.mp4":                           "session-init",
		"/Videos/item/360p/segment-00000.m4s":                  "session-segment",
		"/Videos/item/source/hls1/main/index.m3u8":             "source-master",
		"/Videos/item/source/hls1/main/360p/index.m3u8":        "source-variant",
		"/Videos/item/source/hls1/main/360p/init.mp4":          "source-init",
		"/Videos/item/source/hls1/main/360p/segment-00000.m4s": "source-segment",
		"/Videos/item/source/Subtitles/3/Stream.vtt":           "subtitle",
	} {
		if got := shape(path); got != want {
			t.Errorf("jellyfinStreamShape(%q) = %q, want %q", path, got, want)
		}
	}
}

func JellyfinSourceHLSFileRejectsUnsafeChildren(t *testing.T, sourceFile func(string) (string, bool)) {
	t.Helper()
	for _, path := range []string{
		"source/hls1/main/9999p/index.m3u8",
		"source/hls1/main/360p/../index.m3u8",
		"source/hls1/main/360p/segment-1.m4s",
		"source/hls1/main/360p/unexpected.bin",
	} {
		if file, valid := sourceFile(path); valid {
			t.Errorf("jellyfinSourceHLSFile(%q) = %q, true", path, file)
		}
	}
}

// RequestLoggingCannotForgeRecords exercises the same JSON handler installed by appcli.Execute.
func RequestLoggingCannotForgeRecords(t *testing.T, observe ObservedAuditHandler) {
	t.Helper()
	const attack = "untrusted\r\n{\"level\":\"ERROR\",\"msg\":\"forged\"}\x1b[2J<script>alert(1)</script>"
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	handler := observe(t, "GET /media/{id}", http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(attack) }))
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/media/item?api_key=secret", nil)
	request.Method = attack
	request.Header.Set("X-Request-ID", attack)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	lines := bytes.Split(bytes.TrimSpace(output.Bytes()), []byte{'\n'})
	if response.Code != http.StatusInternalServerError || len(lines) != 2 {
		t.Fatalf("response=%d records=%d log=%q", response.Code, len(lines), output.String())
	}
	for index, line := range lines {
		assertJSONRequestRecord(t, line, []string{"request panic", "request"}[index], attack)
	}
}

func assertJSONRequestRecord(t *testing.T, line []byte, message, attack string) {
	t.Helper()
	var record map[string]any
	if err := json.Unmarshal(line, &record); err != nil {
		t.Fatal(err)
	}
	if record["msg"] != message || record["method"] != attack {
		t.Fatalf("record changed shape: %#v", record)
	}
	if message == "request panic" && record["panic"] != attack {
		t.Fatalf("panic detail lost: %#v", record)
	}
	if record["request_id"] == attack || strings.Contains(string(line), "api_key") || bytes.ContainsAny(line, "\r\x1b") {
		t.Fatalf("unescaped or private data in log: %q", line)
	}
}
