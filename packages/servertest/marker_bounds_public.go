package servertest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/documentdb"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/markers"
)

func (fixture PlaybackAPIFixture) PlaybackMarkerBoundsPublicRealMedia(t *testing.T, wantDirectAllowed bool) {
	t.Helper()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("real FFmpeg required")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("real FFprobe required")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	media := t.TempDir()
	source := filepath.Join(media, "Fixture.mp4")
	command := exec.CommandContext(ctx, ffmpeg, "-nostdin", "-v", "error", "-f", "lavfi", "-i", "color=c=blue:s=32x32:r=25:d=304.96", "-c:v", "libx264", "-threads", "1", "-preset", "ultrafast", "-an", source)
	if err := command.Run(); err != nil {
		t.Fatal("real media fixture failed:", err)
	}
	sourceBytes, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	items, err := library.NewScanner(media, ".").Scan(ctx)
	if err != nil || len(items) != 1 {
		t.Fatalf("fixture scan: items=%d err=%v", len(items), err)
	}
	for _, test := range []markerBoundsCase{
		{"ordinary", 10, 20, 20, false},
		{"endpoint", 300, 304.96, 304.96, false},
		{"sub-frame-overrun", 300, 304.96799999999996, 304.96, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture.markerBoundsPublicCase(t, ctx, test, media, source, sourceBytes, items[0], ffmpeg, ffprobe, wantDirectAllowed)
		})
	}
}

type markerBoundsCase struct {
	name                string
	start, end, wantEnd float64
	warning             bool
}

func (fixture PlaybackAPIFixture) markerBoundsPublicCase(t *testing.T, ctx context.Context, test markerBoundsCase, media, source string, sourceBytes []byte, item library.Item, ffmpeg, ffprobe string, wantDirectAllowed bool) {
	t.Helper()
	data := t.TempDir()
	db, err := documentdb.OpenContext(ctx, data, true, documentdb.PlayerConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	analyzer := markers.NewAnalyzer(markers.Config{DataDir: data, Database: db}) //nolint:contextcheck // Persisted fixture setup uses the existing synchronous analyzer/store APIs, which accept no context.
	if err := analyzer.SetManual(item, "credits", test.start, test.end, 305); err != nil {
		t.Fatal(err)
	}
	before, found, err := db.Load("playback_markers.json") //nolint:contextcheck // The existing synchronous store Load API accepts no context.
	if err != nil || !found {
		t.Fatal(err)
	}
	var logs markerBoundsLogBuffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	defer slog.SetDefault(previous)
	workers, stop := context.WithCancel(ctx)
	defer stop()
	handler := fixture.New(PlaybackAPIConfig{Lifecycle: workers, MediaDir: media, DataDir: data, CacheDir: CacheDir(t), FFmpeg: ffmpeg, FFprobe: ffprobe, RequireAuth: true})
	cookie := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	endpoint := httptest.NewServer(handler)
	defer endpoint.Close()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.URL+"/api/v1/items/"+item.ID+"/playback", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.AddCookie(cookie)
	const requestID = "0123456789abcdef01234567"
	request.Header.Set("X-Request-ID", requestID)
	response, err := endpoint.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var result markerBoundsResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	markerBoundsAssertResponse(t, response.StatusCode, result, test, wantDirectAllowed)
	markerBoundsAssertLog(t, logs.String(), requestID, test.warning)
	markerBoundsAssertDirect(t, ctx, endpoint, cookie, result.Direct, sourceBytes)
	markerHash := markerBoundsAssertPreserved(t, db, item.ID, before, source, sourceBytes) //nolint:contextcheck // The assertion reads only this disposable fixture through the existing context-free store API.
	t.Logf("HTTP=%d duration=%.2f marker=[%.3f,%.3f] normalized=%t source_sha256=%x persisted_marker_sha256=%x", response.StatusCode, result.Duration, result.Markers[0].Start, result.Markers[0].End, test.warning, sha256.Sum256(sourceBytes), markerHash)
}

type markerBoundsResponse struct {
	Duration      float64
	Markers       []markers.Marker
	DirectAllowed bool
	Direct        string
}

func markerBoundsAssertResponse(t *testing.T, status int, result markerBoundsResponse, test markerBoundsCase, wantDirectAllowed bool) {
	t.Helper()
	if status != http.StatusOK || result.Duration != 304.96 || len(result.Markers) != 1 || result.Markers[0].Start != test.start || result.Markers[0].End != test.wantEnd || result.DirectAllowed != wantDirectAllowed || result.Direct == "" {
		t.Fatalf("playback contract: status=%d result=%+v", status, result)
	}
}

func markerBoundsAssertDirect(t *testing.T, ctx context.Context, endpoint *httptest.Server, cookie *http.Cookie, path string, sourceBytes []byte) {
	t.Helper()
	direct, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	direct.AddCookie(cookie)
	served, err := endpoint.Client().Do(direct)
	if err != nil {
		t.Fatal(err)
	}
	defer served.Body.Close()
	servedBytes, err := io.ReadAll(io.LimitReader(served.Body, int64(len(sourceBytes)+1)))
	if err != nil || served.StatusCode != http.StatusOK || !bytes.Equal(servedBytes, sourceBytes) {
		t.Fatal("direct source changed or unavailable")
	}
}

func markerBoundsAssertPreserved(t *testing.T, db *documentdb.Store, id string, before []byte, source string, sourceBytes []byte) [32]byte {
	t.Helper()
	after, found, err := db.Load("playback_markers.json")
	if err != nil || !found {
		t.Fatal("saved marker fixture was unavailable")
	}
	var beforeRecords, afterRecords map[string]markers.Record
	if json.Unmarshal(before, &beforeRecords) != nil || json.Unmarshal(after, &afterRecords) != nil || !reflect.DeepEqual(beforeRecords[id].Markers, afterRecords[id].Markers) {
		t.Fatal("saved marker values were modified")
	}
	unchanged, err := os.ReadFile(source)
	if err != nil || !bytes.Equal(sourceBytes, unchanged) {
		t.Fatal("original source was modified")
	}
	return sha256.Sum256(after)
}

func markerBoundsAssertLog(t *testing.T, logs, requestID string, want bool) {
	t.Helper()
	found := 0
	for _, line := range strings.Split(logs, "\n") {
		var record map[string]any
		if json.Unmarshal([]byte(line), &record) != nil || record["failure_class"] != "marker_bounds" {
			continue
		}
		found++
		markerBoundsAssertLogRecord(t, record, line, requestID)
	}
	if (want && found != 1) || (!want && found != 0) {
		t.Fatalf("marker warnings=%d wantWarning=%t", found, want)
	}
}

func markerBoundsAssertLogRecord(t *testing.T, record map[string]any, line, requestID string) {
	t.Helper()
	for key, value := range map[string]any{"level": "WARN", "operation": "playback-response", "request_id": requestID, "input_count": float64(1), "emitted_count": float64(1)} {
		if record[key] != value {
			t.Fatalf("marker diagnostic %s=%v, want %v", key, record[key], value)
		}
	}
	for _, forbidden := range []string{"304.", "Credits", "owner-password"} {
		if strings.Contains(line, forbidden) {
			t.Fatal("marker diagnostic contained private data")
		}
	}
}

type markerBoundsLogBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (logs *markerBoundsLogBuffer) Write(value []byte) (int, error) {
	logs.mu.Lock()
	defer logs.mu.Unlock()
	return logs.buffer.Write(value)
}

func (logs *markerBoundsLogBuffer) String() string {
	logs.mu.Lock()
	defer logs.mu.Unlock()
	return logs.buffer.String()
}
