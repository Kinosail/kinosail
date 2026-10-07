package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/servertest"
)

// Controlled adapters force launch/admission boundaries that moving-media E2E
// cannot force repeatably. These are isolated public HTTP checks, not decode proof.
func TestHLSPhasePublicHTTP(t *testing.T) {
	previousCapacity := runtime.GOMAXPROCS(4)
	t.Cleanup(func() { runtime.GOMAXPROCS(previousCapacity) })
	t.Run("failed launch is never called started", phaseLaunchFailure)
	t.Run("started child waits for valid presentation", phasePendingAndReady)
	t.Run("queued admission is distinct from running child", phaseQueuedAdmission)
	t.Run("page departure releases admission for the next movie", phaseSessionDeparture)
}

type phaseHTTPFixture struct {
	host                          *httptest.Server
	client                        *http.Client
	ids                           []string
	starts, release, media, cache string
	output                        *phaseLogBuffer
	cancel                        context.CancelFunc
}

type phaseLogBuffer struct {
	sync.Mutex
	bytes.Buffer
}

func (buffer *phaseLogBuffer) Write(data []byte) (int, error) {
	buffer.Lock()
	defer buffer.Unlock()
	return buffer.Buffer.Write(data)
}

func (buffer *phaseLogBuffer) snapshot() string {
	buffer.Lock()
	defer buffer.Unlock()
	return buffer.String()
}

func phaseFixture(t *testing.T, count int, executable bool) phaseHTTPFixture {
	t.Helper()
	media, cache, tools := t.TempDir(), t.TempDir(), t.TempDir()
	for index := range count {
		if err := os.WriteFile(filepath.Join(media, fmt.Sprintf("Private Phase Title %d.mkv", index)), []byte("synthetic adapter input"), 0o600); err != nil {
			t.Fatal("cannot write synthetic source")
		}
	}
	probe, encoder := filepath.Join(tools, "ffprobe"), filepath.Join(tools, "ffmpeg")
	servertest.WriteExecutable(t, probe, "#!/bin/sh\nprintf '%s' '{\"streams\":[{\"index\":0,\"codec_type\":\"video\",\"codec_name\":\"h264\",\"width\":1920,\"height\":1080},{\"index\":1,\"codec_type\":\"audio\",\"codec_name\":\"aac\"}],\"format\":{\"format_name\":\"matroska\",\"duration\":\"60\"}}'\n")
	starts, release := filepath.Join(tools, "starts"), filepath.Join(tools, "release")
	if executable {
		servertest.WriteExecutable(t, encoder, fmt.Sprintf("#!/bin/sh\nprintf 'started\\n' >> '%s'\nwhile [ ! -f '%s' ]; do sleep 0.02; done\n", starts, release)+servertest.PlayableHLS())
	}
	output := &phaseLogBuffer{}
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	ctx, cancel := context.WithCancel(t.Context())
	host := httptest.NewServer(New(Config{Lifecycle: ctx, MediaDir: media, DataDir: t.TempDir(), CacheDir: cache, FFprobe: probe, FFmpeg: encoder}))
	t.Cleanup(func() { cancel(); host.Close() })
	fixture := phaseHTTPFixture{host: host, client: &http.Client{Timeout: 4 * time.Second}, starts: starts, release: release, media: media, cache: cache, output: output, cancel: cancel}
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, host.URL+"/?view=movies", nil)
	if err != nil {
		t.Fatal("cannot create synthetic Movies request")
	}
	response, err := fixture.client.Do(request)
	if err != nil {
		t.Fatal("synthetic Movies HTTP request failed")
	}
	var body bytes.Buffer
	_, _ = body.ReadFrom(response.Body)
	_ = response.Body.Close()
	for _, match := range regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindAllStringSubmatch(body.String(), -1) {
		if !strings.Contains(strings.Join(fixture.ids, ","), match[1]) {
			fixture.ids = append(fixture.ids, match[1])
		}
	}
	if len(fixture.ids) != count {
		t.Fatal("synthetic library did not expose expected items")
	}
	return fixture
}


func (fixture phaseHTTPFixture) events(requestID string) []map[string]any {
	var result []map[string]any
	for _, line := range strings.Split(fixture.output.snapshot(), "\n") {
		var entry map[string]any
		if json.Unmarshal([]byte(line), &entry) == nil && entry["request_id"] == requestID {
			result = append(result, entry)
		}
	}
	return result
}

func phaseSequence(events []map[string]any) []string {
	var result []string
	for _, event := range events {
		if phase, ok := event["phase"].(string); ok {
			result = append(result, phase)
		}
	}
	return result
}

func phaseCount(events []map[string]any, message string) int {
	count := 0
	for _, event := range events {
		if event["msg"] == message {
			count++
		}
	}
	return count
}

func phaseWait(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("bounded diagnostic fixture wait expired")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func phaseStatus(t *testing.T, response <-chan int) int {
	t.Helper()
	select {
	case status := <-response:
		return status
	case <-time.After(3 * time.Second):
		t.Fatal("bounded public HLS request did not finish")
		return 0
	}
}

func phaseLaunchFailure(t *testing.T) {
	fixture := phaseFixture(t, 1, false)
	status := phaseStatus(t, fixture.request(t, 0, "a-a0-s0-none-t0-b0", "phase-launch"))
	if status != http.StatusServiceUnavailable {
		t.Fatalf("failed launch HTTP status=%d", status)
	}
	events := fixture.events("phase-launch")
	if phaseCount(events, "HLS transcode started") != 0 {
		t.Fatal("failed executable launch was misleadingly described as started")
	}
	if got := strings.Join(phaseSequence(events), ","); got != "queued,admission_wait,admitted,process_start_failed" {
		t.Fatalf("failed launch phases=%s", got)
	}
	phasePrivacy(t, fixture, events)
	phaseReceipt(t, status, events)
}

func phasePendingAndReady(t *testing.T) {
	fixture := phaseFixture(t, 1, true)
	response := fixture.request(t, 0, "a-a0-s0-none-t0-b0-o3000", "phase-ready")
	phaseWait(t, func() bool { data, _ := os.ReadFile(fixture.starts); return len(data) > 0 })
	phaseWait(t, func() bool { return phaseCount(fixture.events("phase-ready"), "HLS transcode started") > 0 })
	events := fixture.events("phase-ready")
	if got := strings.Join(phaseSequence(events), ","); got != "queued,admission_wait,admitted,process_started" {
		t.Fatalf("pending media phases=%s", got)
	}
	select {
	case <-response:
		t.Fatal("presentation published before encoder output")
	default:
	}
	if err := os.WriteFile(fixture.release, nil, 0o600); err != nil {
		t.Fatal("cannot release encoder fixture")
	}
	status := phaseStatus(t, response)
	if status != http.StatusOK {
		t.Fatalf("ready presentation HTTP status=%d", status)
	}
	phaseWait(t, func() bool { return phaseCount(fixture.events("phase-ready"), "HLS transcode completed") > 0 })
	events = fixture.events("phase-ready")
	if got := strings.Join(phaseSequence(events), ","); got != "queued,admission_wait,admitted,process_started,media_ready" {
		t.Fatalf("published media phases=%s", got)
	}
	for _, event := range events {
		if event["phase"] == "process_started" && event["input_seek_ms"] != float64(3000) {
			t.Fatal("process phase did not preserve effective input seek")
		}
	}
	phasePrivacy(t, fixture, events)
	phaseReceipt(t, status, events)
}

func phaseQueuedAdmission(t *testing.T) {
	fixture := phaseFixture(t, 2, true)
	first := fixture.request(t, 0, "t-a0-s0-none-t0-b0", "phase-first")
	phaseWait(t, func() bool { data, _ := os.ReadFile(fixture.starts); return len(data) > 0 })
	second := fixture.request(t, 1, "a-a0-s0-none-t0-b0", "phase-second")
	phaseWait(t, func() bool { return len(fixture.events("phase-second")) > 0 })
	time.Sleep(100 * time.Millisecond)
	events := fixture.events("phase-second")
	if phaseCount(events, "HLS transcode started") != 0 {
		t.Fatal("waiting admission was misleadingly described as a running child")
	}
	if got := strings.Join(phaseSequence(events), ","); got != "queued,admission_wait" {
		t.Fatalf("waiting admission phases=%s", got)
	}
	fixture.cancel()
	status := phaseStatus(t, second)
	_ = phaseStatus(t, first)
	if status != http.StatusServiceUnavailable {
		t.Fatalf("cancelled waiting admission HTTP status=%d", status)
	}
	events = fixture.events("phase-second")
	for _, event := range events {
		if event["msg"] == "HLS playlist preparation failed" && event["encoder_phase"] != "admission_wait" && event["encoder_phase"] != "admission_rejected" {
			t.Fatal("readiness failure did not retain last admission phase")
		}
	}
	phasePrivacy(t, fixture, events)
	phaseReceipt(t, status, events)
}

func phasePrivacy(t *testing.T, fixture phaseHTTPFixture, events []map[string]any) {
	t.Helper()
	allowed := map[string]bool{}
	for _, key := range []string{"time", "level", "msg", "request_id", "phase", "mode", "work_class", "encoder_cost", "capacity", "input_seek_ms", "segment_start", "elapsed_ms", "queue_wait_ms", "outcome"} {
		allowed[key] = true
	}
	for _, event := range events {
		if _, phaseEvent := event["phase"]; !phaseEvent {
			continue
		}
		for key := range event {
			if !allowed[key] {
				t.Fatalf("new phase event has unapproved field=%s", key)
			}
		}
		encoded, _ := json.Marshal(event)
		for _, private := range append(fixture.ids, fixture.media, fixture.cache, "Private Phase Title", "private-phase-session", "a-a0-s0-none-t0-b0") {
			if strings.Contains(string(encoded), private) {
				t.Fatal("new phase event exposed private diagnostic data")
			}
		}
	}
}

func phaseReceipt(t *testing.T, status int, events []map[string]any) {
	t.Helper()
	projection := make([]map[string]any, 0, len(events))
	for _, event := range events {
		if _, exists := event["phase"]; exists {
			copy := map[string]any{}
			for key, value := range event {
				if key != "request_id" && key != "time" {
					copy[key] = value
				}
			}
			projection = append(projection, copy)
		}
	}
	encoded, _ := json.Marshal(map[string]any{"httpStatus": status, "phases": projection, "boundary": "isolated loopback HTTP with controlled encoder/probe adapters"})
	t.Log(string(encoded))
}
