package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A rehashed, structurally valid proof is not authority to publish media whose
// measured clocks disagree. Exercise the real encoder and HTTP rejection path.
func transcodeRefillRejectClockMismatch(t *testing.T, ctx context.Context, root, prefix, owned, variant, fragment, source string, beforeOwned int, logs *copiedLogBuffer) int {
	t.Helper()
	proofPath, masterPath := filepath.Join(root, ".transcode-clock"), filepath.Join(root, "index.m3u8")
	proof, master := transcodeRefillRead(t, proofPath), transcodeRefillRead(t, masterPath)
	directory := filepath.Join(root, filepath.Dir(variant))
	changed := transcodeRefillAlterPacketClock(t, proof, filepath.Dir(variant))
	changedMaster := transcodeRefillRehashProof(master, changed)
	defer transcodeRefillRestoreProof(t, map[string][]byte{proofPath: proof, masterPath: master})
	if os.WriteFile(proofPath, changed, 0o600) != nil || os.WriteFile(masterPath, changedMaster, 0o600) != nil {
		t.Fatal("cannot prepare rehashed media clock mismatch")
	}
	retained := map[string][]byte{proofPath: changed, masterPath: changedMaster, source: transcodeRefillRead(t, source)}
	for _, name := range []string{"index.m3u8", "init.mp4", "segment-00006.m4s", "segment-00008.m4s"} {
		path := filepath.Join(directory, name)
		retained[path] = transcodeRefillRead(t, path)
	}
	status, requestID := transcodeRefillRejectedResponse(t, ctx, prefix+strings.TrimSuffix(variant, "index.m3u8")+fragment)
	joined := copiedRecoveryJoinedCodecs(t, ctx, owned)
	_, targetErr := os.Stat(filepath.Join(directory, fragment))
	_, zeroErr := os.Stat(filepath.Join(directory, "segment-00000.m4s"))
	if status != http.StatusNotFound || joined != beforeOwned+1 || !os.IsNotExist(targetErr) || !os.IsNotExist(zeroErr) {
		t.Fatal("mismatched staged media was accepted or its producer did not join")
	}
	transcodeRefillRetained(t, retained)
	transcodeRefillStageRemoved(t, filepath.Dir(root), directory)
	transcodeRefillMismatchLog(t, logs, requestID, root, source, prefix)
	transcodeRefillLog(t, map[string]any{"stagedClockRejected": true, "status": status, "producers": joined, "targetAbsent": true, "zeroAbsent": true, "retainedUnchanged": !t.Failed(), "ownedStageRemoved": true})
	return joined
}

func transcodeRefillStageRemoved(t *testing.T, paths ...string) {
	t.Helper()
	for _, path := range paths {
		entries, err := os.ReadDir(path)
		if err != nil {
			t.Fatal("cannot inspect owned refill cleanup")
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".transcode-refill-") {
				t.Fatal("rejected refill left staged or pending media")
			}
		}
	}
}

func transcodeRefillAlterPacketClock(t *testing.T, proof []byte, rendition string) []byte {
	t.Helper()
	var clock map[string]json.RawMessage
	var renditions map[string]json.RawMessage
	var selected map[string]json.RawMessage
	var cuts []map[string]json.RawMessage
	if json.Unmarshal(proof, &clock) != nil || json.Unmarshal(clock["Renditions"], &renditions) != nil || json.Unmarshal(renditions[rendition], &selected) != nil || json.Unmarshal(selected["Cuts"], &cuts) != nil || len(cuts) <= 7 {
		t.Fatal("completed proof lacks the selected late fragment")
	}
	var digests [2][32]byte
	if json.Unmarshal(cuts[7]["Clocks"], &digests) != nil {
		t.Fatal("completed proof lacks packet clock digests")
	}
	digests[0][0] ^= 1
	cuts[7]["Clocks"], _ = json.Marshal(digests)
	selected["Cuts"], _ = json.Marshal(cuts)
	renditions[rendition], _ = json.Marshal(selected)
	clock["Renditions"], _ = json.Marshal(renditions)
	changed, err := json.Marshal(clock)
	if err != nil {
		t.Fatal("cannot encode changed proof control")
	}
	return changed
}

func transcodeRefillMismatchLog(t *testing.T, logs *copiedLogBuffer, requestID string, private ...string) {
	t.Helper()
	matches := 0
	for _, line := range strings.Split(logs.snapshot(), "\n") {
		var record map[string]any
		if json.Unmarshal([]byte(line), &record) != nil || record["request_id"] != requestID || record["level"] != "ERROR" {
			continue
		}
		matches++
		if !transcodeRefillMismatchFields(record) {
			t.Error("staged rejection lacks its correlated measured clock diagnosis")
		}
		for _, value := range private {
			if strings.Contains(line, value) {
				t.Error("staged rejection logged a private path or URL")
			}
		}
	}
	if matches != 1 {
		t.Error("expected one correlated encoder failure for mismatched staged media")
	}
	transcodeRefillLog(t, map[string]any{"stagedClockDiagnostic": true, "errorRecords": matches, "correlationAndPrivacyVerified": !t.Failed()})
}

func transcodeRefillMismatchFields(record map[string]any) bool {
	clock := transcodeRefillClockFields(record)
	return record["msg"] == "HLS transcode failed" && transcodeRefillFailureOperation(record) == "refill-stage-clock" && clock["measured"] == true && clock["init_equal"] == true && clock["video_clock_equal"] == false && clock["audio_clock_equal"] == true
}
