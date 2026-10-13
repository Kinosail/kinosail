package server_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

type refillFrameClock struct{ DTS, PTS, Duration int64 }

type refillFrameTimeline struct {
	Numerator, Denominator int64
	Frames                 []refillFrameClock
}

func transcodeRefillDecode(t *testing.T, ctx context.Context, ffmpeg, label string, parts [][]byte) refillFrameTimeline { //nolint:funlen,cyclop,gocognit // Parse one bounded real decoder receipt without rescaling the timestamp oracle.
	t.Helper()
	arguments := []string{"-nostdin", "-v", "error", "-copyts", "-i", "pipe:0", "-map", "0:v:0", "-enc_time_base", "demux", "-fps_mode", "passthrough", "-avoid_negative_ts", "disabled", "-f", "framehash", "-hash", "sha256", "-"}
	command := exec.CommandContext(ctx, ffmpeg, arguments...) //nolint:gosec // Discovered hosted executable and fixed arguments decode owned synthetic HTTP media.
	command.Stdin = bytes.NewReader(bytes.Join(parts, nil))
	var diagnostics bytes.Buffer
	command.Stderr = &diagnostics
	output, err := command.Output()
	status := -1
	if command.ProcessState != nil {
		status = command.ProcessState.ExitCode()
	}
	var hashes []string
	for _, part := range parts {
		hashes = append(hashes, fmt.Sprintf("%x", sha256.Sum256(part)))
	}
	var classes []string
	for _, class := range []string{"non monotonically increasing dts", "invalid", "corrupt", "error"} {
		if strings.Contains(strings.ToLower(diagnostics.String()), class) {
			classes = append(classes, class)
		}
	}
	transcodeRefillLog(t, map[string]any{"decode": label, "arguments": arguments, "exit": status, "fragmentSHA256": hashes, "framehashSHA256": fmt.Sprintf("%x", sha256.Sum256(output)), "diagnosticSHA256": fmt.Sprintf("%x", sha256.Sum256(diagnostics.Bytes())), "diagnosticBytes": diagnostics.Len(), "diagnosticClasses": classes})
	if err != nil {
		t.Fatal("real fragment sequence decode failed; bounded receipt retained")
	}
	var result refillFrameTimeline
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "#tb 0:") {
			if count, err := fmt.Sscanf(line, "#tb 0: %d/%d", &result.Numerator, &result.Denominator); err != nil || count != 2 {
				t.Fatal("invalid decoded timebase")
			}
		} else if line != "" && !strings.HasPrefix(line, "#") {
			fields := strings.Split(line, ",")
			if len(fields) != 6 || strings.TrimSpace(fields[0]) != "0" {
				t.Fatal("invalid decoded framehash record")
			}
			var clock refillFrameClock
			for index, target := range []*int64{&clock.DTS, &clock.PTS, &clock.Duration} {
				value, err := strconv.ParseInt(strings.TrimSpace(fields[index+1]), 10, 64)
				if err != nil {
					t.Fatal("invalid decoded frame timestamp")
				}
				*target = value
			}
			result.Frames = append(result.Frames, clock)
		}
	}
	clockJSON, err := json.Marshal(result)
	if err != nil || len(result.Frames) == 0 || result.Numerator <= 0 || result.Denominator <= 0 {
		t.Fatal("decoder returned no usable frame clock")
	}
	transcodeRefillLog(t, map[string]any{"decode": label, "frames": len(result.Frames), "timebaseNumerator": result.Numerator, "timebaseDenominator": result.Denominator, "first": result.Frames[0], "last": result.Frames[len(result.Frames)-1], "clockSHA256": fmt.Sprintf("%x", sha256.Sum256(clockJSON))})
	return result
}

func transcodeRefillProducers(t *testing.T, directory string) []map[string][]string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(directory, "args-*"))
	if err != nil {
		t.Fatal(err)
	}
	var records []map[string][]string
	safe := regexp.MustCompile(`^[A-Za-z0-9+._-]{1,64}$`)
	for _, path := range paths {
		arguments := strings.Split(string(transcodeRefillRead(t, path)), "\x00")
		record := make(map[string][]string)
		for index, argument := range arguments {
			switch argument {
			case "-ss", "-output_ts_offset", "-avoid_negative_ts", "-start_number", "-c:v", "-c:a":
				if index+1 >= len(arguments) || !safe.MatchString(arguments[index+1]) {
					t.Fatal("producer clock argument is missing or unbounded")
				}
				record[argument] = append(record[argument], arguments[index+1])
			}
		}
		records = append(records, record)
	}
	return records
}

func transcodeRefillSave(t *testing.T, directory string, value any) {
	t.Helper()
	data, err := json.Marshal(map[string]any{"test": t.Name(), "receipt": value})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(directory, 0o700); err != nil { //nolint:gosec // G703: this test-only evidence root comes from the trusted runner, never an HTTP or media input.
		t.Fatal("cannot create owned media evidence directory")
	}
	path := filepath.Join(directory, fmt.Sprintf("%x.jsonl", sha256.Sum256([]byte(t.Name()))))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600) //nolint:gosec // Trusted test-runner directory and hash-only generated filename.
	if err != nil {
		t.Fatal("cannot retain media evidence")
	}
	_, writeErr := file.Write(append(data, '\n'))
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		t.Fatal("cannot complete media evidence receipt")
	}
}

func transcodeRefillLegacyCache(t *testing.T, ctx context.Context, ffmpeg, master, prefix, root, owned string, variants []string, controls []refillFrameTimeline, beforeOwned int) {
	t.Helper()
	// Simulate a completed pre-policy cache entirely inside the disposable root.
	// Every public identity must agree on the old policy before the HTTP reopen.
	for _, name := range []string{"index.m3u8", ".source", ".seekable"} {
		path := filepath.Join(root, name)
		data := transcodeRefillRead(t, path)
		marker := []byte(":transcode-clock=1")
		if bytes.Count(data, marker) != 1 {
			t.Fatal("transcoded cache must identify its frame-clock policy")
		}
		if err := os.WriteFile(path, bytes.Replace(data, marker, nil, 1), 0o600); err != nil {
			t.Fatal("cannot prepare owned legacy cache")
		}
	}
	rebuilt := bandwidthHTTPGET(t, ctx, master)
	afterOwned := copiedRecoveryJoinedCodecs(t, ctx, owned)
	transcodeRefillLog(t, map[string]any{"legacyCacheReopen": true, "producersBefore": beforeOwned, "producersAfter": afterOwned, "masterSHA256": fmt.Sprintf("%x", sha256.Sum256(rebuilt))})
	if afterOwned != beforeOwned+1 || !bytes.Contains(rebuilt, []byte(":transcode-clock=1")) {
		t.Fatal("public reopen reused the legacy frame clock instead of rebuilding")
	}
	transcodeRefillCurrentBindings(t, root)
	for index, variant := range variants {
		base := prefix + strings.TrimSuffix(variant, "index.m3u8")
		var parts [][]byte
		for _, name := range []string{"init.mp4", "segment-00000.m4s", "segment-00001.m4s", "segment-00002.m4s"} {
			parts = append(parts, bandwidthHTTPGET(t, ctx, base+name))
		}
		actual := transcodeRefillDecode(t, ctx, ffmpeg, "legacy-rebuilt-"+filepath.Dir(variant), parts)
		control := controls[index]
		if actual.Numerator != control.Numerator || actual.Denominator != control.Denominator || !slices.Equal(actual.Frames, control.Frames) {
			t.Fatal("legacy cache rebuild changed the accepted frame clock")
		}
	}
}

func transcodeRefillCurrentBindings(t *testing.T, root string) {
	t.Helper()
	for _, name := range []string{".source", ".seekable"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		// A complete rebuild may remove the seek-only marker. If retained, it
		// must identify the new policy rather than certify the legacy cache.
		if name == ".seekable" && os.IsNotExist(err) {
			continue
		}
		if err != nil || bytes.Count(data, []byte(":transcode-clock=1")) != 1 {
			t.Fatal("rebuilt cache did not persist its frame-clock policy")
		}
	}
}
