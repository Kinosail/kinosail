package server_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Failure-only evidence contains generated-source metadata and hashes, never
// media payloads, absolute paths, request bodies, headers or operator settings.
func retainCopiedFailureFacts(t *testing.T, ffmpeg, ffprobe, source, cache string, logs *copiedLogBuffer) {
	t.Helper()
	t.Cleanup(func() {
		if !t.Failed() {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		for _, tool := range []string{ffmpeg, ffprobe} {
			version, _ := exec.CommandContext(ctx, tool, "-version").Output() //nolint:gosec // Discovered local tool and fixed arguments.
			line := strings.Split(string(version), "\n")[0]
			if len(line) > 256 {
				line = line[:256]
			}
			t.Logf("copied failure tool version: %s", line)
		}
		if data, err := os.ReadFile(source); err == nil {
			t.Logf("copied generated source SHA256=%x bytes=%d", sha256.Sum256(data), len(data))
		}
		copiedSourcePacketFacts(t, ctx, ffprobe, source)
		copiedSourceStreamFacts(t, ctx, ffprobe, source)
		copiedCertificateFacts(t, cache)
		copiedOperationFacts(t, logs)
	})
}

func copiedSourcePacketFacts(t *testing.T, ctx context.Context, ffprobe, source string) {
	t.Helper()
	command := exec.CommandContext(ctx, ffprobe, "-v", "error", "-select_streams", "a:0", "-read_intervals", "%+0.12", "-show_packets", "-show_entries", "packet=pts_time,dts_time,duration_time:packet_side_data=skip_samples,discard_padding", "-of", "json", source) //nolint:gosec // Fixed generated source and tool.
	data, err := command.Output()
	if err != nil {
		t.Log("copied source first AAC packet facts unavailable")
		return
	}
	var value struct {
		Packets []struct {
			PTS      string `json:"pts_time"`
			DTS      string `json:"dts_time"`
			Duration string `json:"duration_time"`
			Side     []struct {
				Skip    int `json:"skip_samples"`
				Discard int `json:"discard_padding"`
			} `json:"side_data_list"`
		} `json:"packets"`
	}
	if json.Unmarshal(data, &value) == nil && len(value.Packets) <= 8 {
		encoded, _ := json.Marshal(value)
		t.Logf("copied first AAC packet facts: %s", encoded)
	}
}

func copiedSourceStreamFacts(t *testing.T, ctx context.Context, ffprobe, source string) {
	t.Helper()
	command := exec.CommandContext(ctx, ffprobe, "-v", "error", "-count_packets", "-show_entries", "stream=index,codec_name,codec_type,time_base,start_time,nb_read_packets,sample_rate,initial_padding:format=start_time,duration", "-of", "json", source) //nolint:gosec // Fixed generated source and tool.
	data, err := command.Output()
	if err == nil && len(data) <= 2048 && json.Valid(data) {
		t.Logf("copied generated stream counts and clocks: %s", data)
	}
}

func copiedCertificateFacts(t *testing.T, cache string) {
	t.Helper()
	root, err := os.OpenRoot(cache)
	if err != nil {
		return
	}
	defer root.Close()
	_ = fs.WalkDir(root.FS(), ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Name() != ".copy-timeline" {
			return nil
		}
		data, readErr := root.ReadFile(path)
		var value struct {
			Keys     []struct{ PTS, DTS int64 }
			TimeBase float64
			End      float64
			Clock    *float64
		}
		if readErr == nil && json.Unmarshal(data, &value) == nil && len(value.Keys) <= 8 {
			encoded, _ := json.Marshal(value)
			t.Logf("copied certified cuts and common clock: %s", encoded)
		}
		return nil
	})
}

func copiedOperationFacts(t *testing.T, output *copiedLogBuffer) {
	t.Helper()
	lines := strings.Split(output.snapshot(), "\n")
	if len(lines) > 40 {
		lines = lines[len(lines)-40:]
	}
	for _, line := range lines {
		var entry map[string]any
		if json.Unmarshal([]byte(line), &entry) != nil || !strings.HasPrefix(fmt.Sprint(entry["msg"]), "HLS ") {
			continue
		}
		projected := map[string]any{}
		for _, key := range []string{"msg", "state", "phase", "mode", "duration_ms", "cache_state", "prepare_ms", "software_fallback", "encoder_phase"} {
			if value, found := entry[key]; found && len(fmt.Sprint(value)) <= 100 {
				projected[key] = value
			}
		}
		encoded, _ := json.Marshal(projected)
		t.Logf("copied operational phase: %s", encoded)
	}
}
