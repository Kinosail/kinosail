package server_test

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A completed certified generation cannot silently downgrade to a legacy writer
// after its proof disappears. Ordinary browser journeys cannot evict this state.
func transcodeRefillRejectDamagedProof(t *testing.T, ctx context.Context, root, prefix, owned, variant, fragment string, beforeOwned int) {
	t.Helper()
	proofPath := filepath.Join(root, ".transcode-clock")
	proof := transcodeRefillRead(t, proofPath)
	master := transcodeRefillRead(t, filepath.Join(root, "index.m3u8"))
	if !bytes.Contains(master, []byte("#KINOSAIL-TRANSCODE-REFILL:")) {
		t.Fatal("completed public presentation did not bind its refill proof")
	}
	directory := filepath.Join(root, filepath.Dir(variant))
	initialization := transcodeRefillRead(t, filepath.Join(directory, "init.mp4"))
	neighbor := transcodeRefillRead(t, filepath.Join(directory, "segment-00008.m4s"))
	if err := os.Remove(filepath.Join(directory, fragment)); err != nil {
		t.Fatal("cannot evict owned fragment for rejected recovery")
	}
	for _, damage := range []string{"missing-proof", "malformed-proof", "changed-init"} {
		t.Run(damage, func(t *testing.T) {
			if damage == "missing-proof" {
				if err := os.Remove(proofPath); err != nil {
					t.Fatal(err)
				}
			} else if damage == "malformed-proof" {
				if err := os.WriteFile(proofPath, []byte("{"), 0o600); err != nil {
					t.Fatal(err)
				}
			} else {
				changed := bytes.Clone(initialization)
				changed[len(changed)-1] ^= 1
				if err := os.WriteFile(filepath.Join(directory, "init.mp4"), changed, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			defer func() {
				if os.WriteFile(proofPath, proof, 0o600) != nil || os.WriteFile(filepath.Join(directory, "init.mp4"), initialization, 0o600) != nil {
					t.Error("cannot restore owned proof control")
				}
			}()
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, prefix+strings.TrimSuffix(variant, "index.m3u8")+fragment, nil)
			if err != nil {
				t.Fatal(err)
			}
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatal("damaged-proof HTTP request did not complete")
			}
			_ = response.Body.Close()
			joined := copiedRecoveryJoinedCodecs(t, ctx, owned)
			_, targetErr := os.Stat(filepath.Join(directory, fragment))
			_, zeroErr := os.Stat(filepath.Join(directory, "segment-00000.m4s"))
			transcodeRefillLog(t, map[string]any{"damagedProof": damage, "status": response.StatusCode, "producers": joined, "targetAbsent": os.IsNotExist(targetErr), "zeroAbsent": os.IsNotExist(zeroErr)})
			if response.StatusCode != http.StatusNotFound || joined != beforeOwned || !os.IsNotExist(targetErr) || !os.IsNotExist(zeroErr) {
				t.Error("damaged proof admitted a writer or exposed regenerated media")
			}
			if !bytes.Equal(master, transcodeRefillRead(t, filepath.Join(root, "index.m3u8"))) || !bytes.Equal(neighbor, transcodeRefillRead(t, filepath.Join(directory, "segment-00008.m4s"))) {
				t.Error("rejected recovery changed retained presentation bytes")
			}
		})
	}
}
