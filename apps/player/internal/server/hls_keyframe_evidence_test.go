package server_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Completed-file browser tests cannot locate a producer/request ordering failure.
// Observe this real-media journey before canceling its producer, without retries.
func copiedKeyframeFailureEvidence(t *testing.T, cache string, logs *copiedLogBuffer) {
	t.Helper()
	files, unavailable := copiedKeyframeCache(cache)
	data, err := json.Marshal(map[string]any{"events": copiedKeyframeEvents(logs), "files": files, "unavailable": unavailable})
	if err == nil {
		t.Logf("COPIED_KEYFRAME_FAILURE %s", data)
	}
}

func copiedKeyframeEvents(logs *copiedLogBuffer) []map[string]any {
	var events []map[string]any
	for _, line := range strings.Split(logs.snapshot(), "\n") {
		if len(events) == 16 {
			break
		}
		var entry map[string]any
		if json.Unmarshal([]byte(line), &entry) != nil {
			continue
		}
		message, _ := entry["msg"].(string)
		if !copiedKeyframeRelevantEvent(entry, message) {
			continue
		}
		events = append(events, copiedKeyframeEvent(entry, message))
	}
	return events
}

func copiedKeyframeRelevantEvent(entry map[string]any, message string) bool {
	if !strings.HasPrefix(message, "HLS ") || len(message) > 120 {
		return false
	}
	switch entry["level"] {
	case "WARN", "ERROR":
		return true
	case "INFO":
		return message == "HLS transcode completed" || message == "HLS missing segment recovery resumed"
	default:
		return false
	}
}

func copiedKeyframeEvent(entry map[string]any, message string) map[string]any {
	failure, _ := entry["error"].(string)
	classes := make([]string, 0)
	for _, value := range []string{"context canceled", "deadline exceeded", "copied HLS", "timeline", "clock", "source", "no such file", "outside the playable duration", "exit status", "signal: killed", "permission denied"} {
		if strings.Contains(failure, value) {
			classes = append(classes, value)
		}
	}
	failureClass := "other"
	for _, known := range []string{"audio-grid", "generation-or-assets", "invalid-timeline"} {
		if entry["failure_class"] == known {
			failureClass = known
		}
	}
	return map[string]any{"level": entry["level"], "message": message, "failureClass": failureClass,
		"errorClasses": classes, "errorSHA256": fmt.Sprintf("%x", sha256.Sum256([]byte(failure))), "requestCorrelated": entry["request_id"] != nil && entry["request_id"] != ""}
}

func copiedKeyframeCache(cache string) ([]map[string]any, map[string]int) {
	var files []map[string]any
	generation, unavailable := map[string]int{}, map[string]int{}
	walkErr := filepath.WalkDir(cache, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			unavailable["walk"]++
			return fs.SkipDir
		}
		relative, err := filepath.Rel(cache, path)
		if err != nil || relative == "." {
			return nil
		}
		parts := strings.Split(relative, string(filepath.Separator))
		if len(parts) > 3 || len(files) == 128 {
			unavailable["truncated"]++
			return fs.SkipDir
		}
		if entry.IsDir() {
			return nil
		}
		if _, exists := generation[parts[0]]; !exists {
			generation[parts[0]] = len(generation)
		}
		if row := copiedKeyframeCacheFile(path, entry, unavailable); row != nil {
			row["generation"], row["depth"] = generation[parts[0]], len(parts)
			row["resumed"] = strings.Contains(parts[0], "-o12700")
			files = append(files, row)
		}
		return nil
	})
	if walkErr != nil {
		unavailable["walk"]++
	}
	return files, unavailable
}

func copiedKeyframeCacheFile(path string, entry fs.DirEntry, unavailable map[string]int) map[string]any {
	info, err := entry.Info()
	if err != nil || !info.Mode().IsRegular() {
		unavailable["file"]++
		return nil
	}
	kind := copiedKeyframeFileKind(entry.Name())
	row := map[string]any{"file": kind, "bytes": info.Size()}
	if kind == "index.m3u8" {
		if info.Size() > 1<<20 {
			unavailable["playlistBound"]++
		} else if data, err := os.ReadFile(path); err == nil {
			row["playlist"] = json.RawMessage(copiedKeyframePlaylistEvidence(data))
		} else {
			unavailable["playlistRead"]++
		}
	}
	return row
}

func copiedKeyframeFileKind(name string) string {
	for _, known := range []string{"index.m3u8", "init.mp4", ".source", ".copy-timeline", ".copy-clock"} {
		if name == known {
			return known
		}
	}
	if strings.HasPrefix(name, "segment-") && strings.HasSuffix(name, ".m4s") && len(name) == 17 {
		if _, err := strconv.ParseUint(name[8:13], 10, 32); err == nil {
			return name
		}
	}
	return "other"
}

func copiedKeyframePlaylistEvidence(data []byte) []byte {
	var durations []float64
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "#EXTINF:") && len(durations) < 128 {
			value, _, _ := strings.Cut(strings.TrimPrefix(line, "#EXTINF:"), ",")
			if duration, err := strconv.ParseFloat(value, 64); err == nil && duration > 0 && duration <= 60 {
				durations = append(durations, duration)
			}
		}
	}
	result, _ := json.Marshal(map[string]any{"sha256": fmt.Sprintf("%x", sha256.Sum256(data)),
		"durations": durations, "endlist": strings.Contains(string(data), "#EXT-X-ENDLIST"), "bytes": len(data)})
	return result
}
