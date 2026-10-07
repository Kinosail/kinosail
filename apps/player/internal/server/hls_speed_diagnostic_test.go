package server_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

// These controls own the failure-witness contract, independently of the real
// codec speed oracle. Diagnostics must not expose policy/media contents.
type speedFileFact struct {
	State            string
	Bytes            int64
	SHA256, Identity string
}

type speedCacheFact struct {
	Recipe       string
	Files        map[string]speedFileFact
	TimelineKeys int
	Clock        string
}

func speedFailureCacheFacts(cache, target string) []speedCacheFact {
	if !speedFailureTarget(target) {
		return nil
	}
	root, err := os.OpenRoot(cache)
	if err != nil {
		return nil
	}
	defer root.Close()
	directory, err := root.Open(".")
	if err != nil {
		return nil
	}
	defer directory.Close()
	entries, _ := directory.ReadDir(64)
	var facts []speedCacheFact
	for _, entry := range entries {
		if !entry.IsDir() || len(facts) == 4 {
			continue
		}
		name := entry.Name()
		source, _ := speedFailureFile(root, name+"/.source", 16<<10)
		if source.State == "missing" {
			continue
		}
		fact := speedFailureRecipe(root, name, target, source)
		facts = append(facts, fact)
	}
	return facts
}

func speedFailureFile(root *os.Root, name string, limit int64) (speedFileFact, []byte) {
	before, err := root.Lstat(name)
	if os.IsNotExist(err) {
		return speedFileFact{State: "missing"}, nil
	}
	if err != nil {
		return speedFileFact{State: "stat-error"}, nil
	}
	fact := speedFileFact{State: "nonregular", Bytes: before.Size()}
	if !before.Mode().IsRegular() {
		return fact, nil
	}
	fact.State = "oversized"
	if before.Size() > limit {
		return fact, nil
	}
	return speedFailureRead(root, name, limit, before, fact)
}

type speedDiagnosticHandler struct {
	http.Handler
	cache    string
	logs     *copiedLogBuffer
	requests int
}

func speedFailureOperations(t *testing.T, output *copiedLogBuffer, requestID string) {
	t.Helper()
	lines := strings.Split(output.snapshot(), "\n")
	if len(lines) > 40 {
		lines = lines[len(lines)-40:]
	}
	for _, line := range lines {
		var entry map[string]any
		if json.Unmarshal([]byte(line), &entry) != nil {
			continue
		}
		message, _ := entry["msg"].(string)
		switch message {
		case "HLS startup preparation", "HLS transcode completed", "HLS transcode failed", "HLS transcode paused after playback became inactive", "HLS stream identity changed", "HLS segment preparation failed", "HLS segment unavailable", "HLS copied clock rejected", "HLS master rejected":
		default:
			continue
		}
		facts := speedFailureOperationFields(entry, requestID)
		data, _ := json.Marshal(facts)
		t.Logf("HLS speed observed phase (worker ownership unknown): %s", data)
	}
}

func speedFailureTarget(target string) bool {
	rendition, segment, ok := strings.Cut(target, "/")
	if !ok || len(rendition) < 2 || len(rendition) > 6 || !strings.HasSuffix(rendition, "p") {
		return false
	}
	height, err := strconv.Atoi(strings.TrimSuffix(rendition, "p"))
	if err != nil || height < 1 || height > 4320 || rendition != strconv.Itoa(height)+"p" {
		return false
	}
	if _, valid := hlsSegmentNameForEvidence(segment); !valid {
		return false
	}
	return true
}

func speedFailureRecipe(root *os.Root, name, target string, source speedFileFact) speedCacheFact {
	rendition, _, _ := strings.Cut(target, "/")
	fact := speedCacheFact{Recipe: fmt.Sprintf("%x", sha256.Sum256([]byte(name))), Files: map[string]speedFileFact{"source": source}, TimelineKeys: -1, Clock: "unavailable"}
	var timeline []byte
	for role, file := range map[string]string{"master": "index.m3u8", "manifest": rendition + "/index.m3u8", "initialization": rendition + "/init.mp4", "segment": target, "timeline": ".copy-timeline", "certificate": ".copy-clock"} {
		limit := int64(1 << 20)
		if role == "timeline" {
			limit = 256 << 10
		}
		if role == "certificate" {
			limit = 4096
		}
		value, data := speedFailureFile(root, name+"/"+file, limit)
		fact.Files[role] = value
		if role == "timeline" {
			timeline = data
		}
	}
	fact.TimelineKeys, fact.Clock = speedFailureTimeline(timeline)
	return fact
}

func speedFailureTimeline(timeline []byte) (int, string) {
	keysCount, clockState := -1, "unavailable"
	var value map[string]json.RawMessage
	if httpguard.DecodeUniqueJSON(bytes.NewReader(timeline), 256<<10, &value) == nil && value != nil {
		var keys []json.RawMessage
		if value != nil && json.Unmarshal(value["Keys"], &keys) == nil && keys != nil && len(keys) <= 4096 {
			keysCount = len(keys)
		}
		clockState = speedFailureClock(value)
	}
	return keysCount, clockState
}

func speedFailureRead(root *os.Root, name string, limit int64, before os.FileInfo, fact speedFileFact) (speedFileFact, []byte) {
	file, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		fact.State = "open-error"
		return fact, nil
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !speedFailureSameFile(before, opened) {
		fact.State = "changed"
		return fact, nil
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	after, statErr := root.Lstat(name)
	if err != nil || statErr != nil || !speedFailureSameFile(before, after) || int64(len(data)) != before.Size() {
		fact.State = "changed"
		return fact, nil
	}
	fact.State = "regular"
	fact.SHA256 = fmt.Sprintf("%x", sha256.Sum256(data))
	if stat, ok := before.Sys().(*syscall.Stat_t); ok {
		fact.Identity = fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%d:%d", stat.Dev, stat.Ino))))
	}
	return fact, data
}

func speedFailureSameFile(before, after os.FileInfo) bool {
	return before != nil && after != nil && after.Mode().IsRegular() && os.SameFile(before, after) && before.Size() == after.Size() && before.ModTime().Equal(after.ModTime())
}

func speedFailureOperationFields(entry map[string]any, requestID string) map[string]any {
	facts := map[string]any{"msg": entry["msg"], "request_matches": entry["request_id"] == requestID, "session_matches": entry["playback_session"] == "hls-speed-fixture"}
	switch entry["mode"] {
	case "remux", "transcode", "audio-transcode":
		facts["mode"] = entry["mode"]
	}
	switch entry["failure_class"] {
	case "generation-or-assets", "invalid-master":
		facts["failure_class"] = entry["failure_class"]
	}
	if duration, ok := entry["duration_ms"].(float64); ok && duration >= 0 && duration <= 3600000 {
		facts["duration_ms"] = duration
	}
	return facts
}

func speedFailureClock(value map[string]json.RawMessage) string {
	clockState := "absent"

	if clock, found := value["Clock"]; found && string(clock) != "null" {
		var number float64
		clockState = "malformed"
		if json.Unmarshal(clock, &number) == nil && number >= 0 && number <= 1 {
			clockState = "present"
		}
	}
	return clockState
}
