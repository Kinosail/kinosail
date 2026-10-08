package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// This isolates rooted file admission from the separate certificate verifier.
// A synthetic geometry tuple is never presented as actual public cache proof.
func TestRemainingNonKeyRootedStartupNeedsFifthAsset(t *testing.T) {
	for _, origin := range []float64{12.5, 13.5, 18.2} {
		t.Run(fmt.Sprintf("%.1f", origin), func(t *testing.T) {
			timeline := remainingNonKeyContractTimeline(t, origin, true)
			cache, directory, root := remainingNonKeyAssetFixture(t)
			physical := remainingNonKeyPhysicalManifest(timeline, 4)
			remainingNonKeyAssetWrite(t, directory, "360p/index.m3u8", physical)
			remainingNonKeyAssetWrite(t, directory, "360p/init.mp4", []byte("owned synthetic init"))
			for number := range 4 {
				remainingNonKeyAssetWrite(t, directory, fmt.Sprintf("360p/segment-%05d.m4s", number), []byte("owned synthetic fragment"))
			}
			projection := func(input []byte) []byte {
				value, accepted := copiedHLSManifest(input, timeline)
				if !accepted {
					return nil
				}
				return value
			}
			if startupRenditionReady(root, directory, "owned", "360p/index.m3u8", 32-origin, projection) {
				t.Fatal("four physical GOPs granted eight presentation seconds")
			}
			remainingNonKeyAssetWrite(t, directory, "360p/segment-00004.m4s", []byte("owned synthetic fifth fragment"))
			if !startupRenditionReady(root, directory, "owned", "360p/index.m3u8", 32-origin, projection) {
				t.Fatal("five complete presentation cuts were not file-ready")
			}
			remainingNonKeyAssetWrite(t, directory, ".source", []byte("owned-source-policy"))
			data, err := json.Marshal(remainingNonKeyContractObject(origin, true))
			if err != nil {
				t.Fatal("nonkey synthetic metadata encoding")
			}
			remainingNonKeyAssetWrite(t, directory, ".copy-timeline", data)
			manager := &hlsManager{ctx: t.Context(), cache: cache}
			if _, err := manager.readCopiedHLSTimelineContext(t.Context(), directory, "owned-source-policy"); err == nil {
				t.Fatal("naked geometry admitted actual cache without asset certificate")
			}
			after, err := os.ReadFile(filepath.Join(directory, ".copy-timeline"))
			if err != nil || !bytes.Equal(after, data) {
				t.Fatal("certificate rejection changed owned metadata")
			}
		})
	}
}

func remainingNonKeyAssetFixture(t *testing.T) (string, string, *os.Root) {
	t.Helper()
	cache := t.TempDir()
	directory := filepath.Join(cache, "owned")
	if os.MkdirAll(filepath.Join(directory, "360p"), 0o700) != nil {
		t.Fatal("nonkey owned rendition directory")
	}
	root, err := os.OpenRoot(cache)
	if err != nil {
		t.Fatal("nonkey owned cache root")
	}
	t.Cleanup(func() { _ = root.Close() })
	return cache, directory, root
}

func remainingNonKeyAssetWrite(t *testing.T, directory, name string, data []byte) {
	t.Helper()
	if os.WriteFile(filepath.Join(directory, name), data, 0o600) != nil {
		t.Fatal("nonkey owned asset write")
	}
}
