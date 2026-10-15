package server_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func transcodeRefillRejectDamagedProof(t *testing.T, ctx context.Context, root, prefix, owned, variant, fragment, source string, beforeOwned int) {
	t.Helper()
	proofPath := filepath.Join(root, ".transcode-clock")
	proof := transcodeRefillRead(t, proofPath)
	master := transcodeRefillRead(t, filepath.Join(root, "index.m3u8"))
	sourceBefore := transcodeRefillRead(t, source)
	if !bytes.Contains(master, []byte("#KINOSAIL-TRANSCODE-REFILL:")) {
		t.Fatal("completed public presentation did not bind its refill proof")
	}
	directory := filepath.Join(root, filepath.Dir(variant))
	initialization := transcodeRefillRead(t, filepath.Join(directory, "init.mp4"))
	neighbor := transcodeRefillRead(t, filepath.Join(directory, "segment-00008.m4s"))
	if err := os.Remove(filepath.Join(directory, fragment)); err != nil {
		t.Fatal("cannot evict owned fragment for rejected recovery")
	}
	for _, damage := range []string{"missing-proof", "malformed-proof", "changed-init", "oversized-samples", "oversized-video-base"} {
		t.Run(damage, func(t *testing.T) {
			expectedMaster := transcodeRefillDamageProof(t, root, variant, damage, proof, initialization, master)
			defer transcodeRefillRestoreProof(t, map[string][]byte{proofPath: proof, filepath.Join(directory, "init.mp4"): initialization, filepath.Join(root, "index.m3u8"): master})
			status := transcodeRefillRejectedStatus(t, ctx, prefix+strings.TrimSuffix(variant, "index.m3u8")+fragment)
			joined := copiedRecoveryJoinedCodecs(t, ctx, owned)
			_, targetErr := os.Stat(filepath.Join(directory, fragment))
			_, zeroErr := os.Stat(filepath.Join(directory, "segment-00000.m4s"))
			transcodeRefillLog(t, map[string]any{"damagedProof": damage, "status": status, "producers": joined, "targetAbsent": os.IsNotExist(targetErr), "zeroAbsent": os.IsNotExist(zeroErr)})
			if status != http.StatusNotFound || joined != beforeOwned || !os.IsNotExist(targetErr) || !os.IsNotExist(zeroErr) {
				t.Error("damaged proof admitted a writer or exposed regenerated media")
			}
			transcodeRefillRetained(t, map[string][]byte{filepath.Join(root, "index.m3u8"): expectedMaster, filepath.Join(directory, "segment-00008.m4s"): neighbor, source: sourceBefore})
		})
	}
}

func transcodeRefillRestoreProof(t *testing.T, files map[string][]byte) {
	t.Helper()
	for path, data := range files {
		if os.WriteFile(path, data, 0o600) != nil {
			t.Error("cannot restore owned proof control")
		}
	}
}

func transcodeRefillRejectedStatus(t *testing.T, ctx context.Context, url string) int {
	t.Helper()
	status, _ := transcodeRefillRejectedResponse(t, ctx, url)
	return status
}

func transcodeRefillRejectedResponse(t *testing.T, ctx context.Context, url string) (int, string) {
	t.Helper()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal("damaged-proof HTTP request did not complete")
	}
	_ = response.Body.Close()
	requestID := response.Header.Get("X-Request-ID")
	if requestID == "" || len(requestID) > 128 {
		t.Fatal("rejected HTTP request lacks a bounded correlation ID")
	}
	return response.StatusCode, requestID
}

func transcodeRefillRetained(t *testing.T, files map[string][]byte) {
	t.Helper()
	for path, expected := range files {
		if !bytes.Equal(expected, transcodeRefillRead(t, path)) {
			t.Error("rejected recovery changed retained presentation bytes")
		}
	}
}

func transcodeRefillDamageProof(t *testing.T, root, variant, damage string, proof, initialization, master []byte) []byte {
	t.Helper()
	proofPath := filepath.Join(root, ".transcode-clock")
	switch damage {
	case "missing-proof":
		if err := os.Remove(proofPath); err != nil {
			t.Fatal(err)
		}
	case "malformed-proof":
		if err := os.WriteFile(proofPath, []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
	case "changed-init":
		changed := bytes.Clone(initialization)
		changed[len(changed)-1] ^= 1
		if err := os.WriteFile(filepath.Join(root, filepath.Dir(variant), "init.mp4"), changed, 0o600); err != nil {
			t.Fatal(err)
		}
	default:
		changed := transcodeRefillOversizedProof(t, proof, damage, filepath.Dir(variant))
		master = transcodeRefillRehashProof(master, changed)
		if os.WriteFile(proofPath, changed, 0o600) != nil || os.WriteFile(filepath.Join(root, "index.m3u8"), master, 0o600) != nil {
			t.Fatal("cannot prepare rehashed arithmetic rejection control")
		}
	}
	return master
}

func transcodeRefillRehashProof(master, changed []byte) []byte {
	lines := strings.Split(string(master), "\n")
	for index, line := range lines {
		if strings.HasPrefix(line, "#KINOSAIL-TRANSCODE-REFILL:") {
			lines[index] = fmt.Sprintf("#KINOSAIL-TRANSCODE-REFILL:%x", sha256.Sum256(changed))
		}
	}
	return []byte(strings.Join(lines, "\n"))
}

func transcodeRefillOversizedProof(t *testing.T, proof []byte, damage, rendition string) []byte {
	t.Helper()
	var clock map[string]json.RawMessage
	if json.Unmarshal(proof, &clock) != nil {
		t.Fatal("completed proof control is not JSON")
	}
	var fields map[string]json.RawMessage
	if damage == "oversized-samples" {
		if json.Unmarshal(clock["Source"], &fields) != nil {
			t.Fatal("source proof fields unavailable")
		}
		fields["Samples"] = json.RawMessage("9223372036854775807")
		clock["Source"], _ = json.Marshal(fields)
	} else {
		var renditions map[string]json.RawMessage
		if json.Unmarshal(clock["Renditions"], &renditions) != nil || json.Unmarshal(renditions[rendition], &fields) != nil {
			t.Fatal("rendition proof fields unavailable")
		}
		fields["VideoBase"] = json.RawMessage("9223372036854775807")
		renditions[rendition], _ = json.Marshal(fields)
		clock["Renditions"], _ = json.Marshal(renditions)
	}
	changed, err := json.Marshal(clock)
	if err != nil {
		t.Fatal(err)
	}
	return changed
}
