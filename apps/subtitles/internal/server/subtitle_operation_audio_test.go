package server_test

import (
	"encoding/json"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

// The executable supplies fictional silence at the process boundary; it is
// not an encoder or an internal audio-manager replacement. Native playback
// and actual lost-write response evidence remain separate journeys.
func TestSubtitleActionLegacyAudioPublicControl(t *testing.T) {
	handler, base, target, calls, _ := subtitleOperationAudioFixture(t, false)
	response := requestJSON(t, handler, http.MethodPost, base+"/audio", `{"language":"en"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("legacy public audio prerequisite = %d", response.Code)
	}
	assertSubtitleOperationAudioResult(t, response.Body.Bytes())
	assertSubtitleOperationProcessCount(t, calls, 1)
	assertSubtitleActionBytes(t, target, []byte(subtitleActionInitial))
	_ = subtitleActionHistory(t, handler, nil, nil)
}

func TestSubtitlePreparedAudioRejectsSecondActivationWithoutStartingAnotherProcess(t *testing.T) {
	handler, base, target, calls, release := subtitleOperationAudioFixture(t, true)
	first := prepareSubtitleOperation(t, handler, base, "audio")
	second := prepareSubtitleOperation(t, handler, base, "audio")
	assertSubtitleOperationAccepted(t, activateSubtitleOperation(t, handler, base+"/audio", `{"language":"en"}`, []string{first.ID}), first.ID)
	waitSubtitleOperationProcess(t, calls)
	busy := activateSubtitleOperation(t, handler, base+"/audio", `{"language":"en"}`, []string{second.ID})
	if busy.Code != http.StatusConflict {
		t.Fatalf("concurrent audio activation = %d, want 409 before work", busy.Code)
	}
	if state := readSubtitleOperation(t, handler, second.ID); state.State != "prepared" || state.Status != 0 {
		t.Fatalf("busy activation consumed second receipt: %+v", state)
	}
	assertSubtitleOperationAccepted(t, activateSubtitleOperation(t, handler, base+"/audio", `{"language":"en"}`, []string{first.ID}), first.ID)
	assertSubtitleOperationProcessCount(t, calls, 1)
	assertSubtitleActionBytes(t, target, []byte(subtitleActionInitial))
	writeTestFile(t, release, "release fictional analysis")
	completed := waitSubtitleOperation(t, handler, first.ID)
	if completed.Status != http.StatusOK || completed.Outcome != "success" {
		t.Fatalf("completed audio outcome = %+v", completed)
	}
	response := requestApp(t, handler, http.MethodGet, "/api/v1/subtitle-operations/"+first.ID+"/result", "")
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("private audio result = %d", response.Code)
	}
	assertSubtitleOperationAudioResult(t, response.Body.Bytes())
	assertSubtitleOperationProcessCount(t, calls, 1)
	assertSubtitleActionBytes(t, target, []byte(subtitleActionInitial))
	_ = subtitleActionHistory(t, handler, nil, nil)
}

func TestSubtitlePreparedAudioCannotBypassActiveLegacyAnalysis(t *testing.T) {
	handler, base, target, calls, release := subtitleOperationAudioFixture(t, true)
	prepared := prepareSubtitleOperation(t, handler, base, "audio")
	finished := make(chan int, 1)
	go func() {
		finished <- requestJSON(t, handler, http.MethodPost, base+"/audio", `{"language":"en"}`).Code
	}()
	waitSubtitleOperationProcess(t, calls)
	busy := activateSubtitleOperation(t, handler, base+"/audio", `{"language":"en"}`, []string{prepared.ID})
	if busy.Code != http.StatusConflict {
		t.Fatalf("activation during legacy analysis = %d, want 409 before work", busy.Code)
	}
	assertSubtitleOperationProcessCount(t, calls, 1)
	writeTestFile(t, release, "release fictional analysis")
	select {
	case status := <-finished:
		if status != http.StatusOK {
			t.Fatalf("legacy analysis after release = %d", status)
		}
	case <-time.After(60 * time.Second):
		t.Fatal("legacy analysis did not settle within its bounded fixture wait")
	}
	assertSubtitleActionBytes(t, target, []byte(subtitleActionInitial))
	_ = subtitleActionHistory(t, handler, nil, nil)
}

func subtitleOperationAudioFixture(t *testing.T, held bool) (http.Handler, string, string, string, string) {
	t.Helper()
	config, target, calls, release := subtitleOperationAudioConfig(t, held)
	handler := server.New(config)
	return handler, "/api/v1/subtitle-library/" + firstSubtitleInventoryID(t, handler), target, calls, release
}

func subtitleOperationAudioConfig(t *testing.T, held bool) (server.Config, string, string, string) {
	t.Helper()
	media, tools := t.TempDir(), t.TempDir()
	writeTestFile(t, filepath.Join(media, "R06 Example.mp4"), "fictional synthetic media")
	target := filepath.Join(media, "R06 Example.en.srt")
	writeTestFile(t, target, subtitleActionInitial)
	probe, process := filepath.Join(tools, "ffprobe"), filepath.Join(tools, "fictional-audio")
	calls, release := filepath.Join(tools, "analysis-starts"), filepath.Join(tools, "release")
	writeExecutable(t, probe, `#!/bin/sh
printf '%s' '{"streams":[{"index":0,"codec_type":"video","codec_name":"h264"},{"index":1,"codec_type":"audio","codec_name":"aac","tags":{"language":"eng"},"disposition":{"default":1}}],"format":{"format_name":"mp4","duration":"60"}}'
`)
	body := "#!/bin/sh\nif [ \"$1\" != '-v' ]; then exit 1; fi\nprintf 'analysis\\n' >> '" + calls + "'\n"
	if held {
		body += "while [ ! -f '" + release + "' ]; do sleep 0.01; done\n"
	}
	body += "dd if=/dev/zero bs=1920000 count=1 2>/dev/null\n"
	writeExecutable(t, process, body)
	t.Cleanup(func() { writeTestFile(t, release, "fixture release") })
	config := server.Config{SubtitleApp: true, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir(), FFprobe: probe, FFmpeg: process, FPCalc: "/usr/bin/false"}
	return config, target, calls, release
}

func waitSubtitleOperationProcess(t *testing.T, calls string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(calls); err == nil && strings.TrimSpace(string(data)) == "analysis" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("fictional analysis process did not start within the fixture bound")
}

func assertSubtitleOperationProcessCount(t *testing.T, calls string, expected int) {
	t.Helper()
	data, err := os.ReadFile(calls)
	if err != nil || strings.Count(string(data), "analysis\n") != expected {
		t.Fatalf("fictional analysis start count = %d, want %d (read error: %v)", strings.Count(string(data), "analysis\n"), expected, err)
	}
}

func assertSubtitleOperationAudioResult(t *testing.T, data []byte) {
	t.Helper()
	var result struct {
		Duration         float64
		Waveform, Speech []float64
	}
	if len(data) > 128*1024 || json.Unmarshal(data, &result) != nil || result.Duration != 60 {
		t.Fatal("fictional one-minute analysis did not produce a bounded public result")
	}
	for _, values := range [][]float64{result.Waveform, result.Speech} {
		if len(values) < 1 || len(values) > 1024 {
			t.Fatalf("public audio result array count = %d, want 1..1024", len(values))
		}
		for _, value := range values {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				t.Fatal("public audio result contains a nonfinite value")
			}
		}
	}
}
