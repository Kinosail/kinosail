package server

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestCopiedVariantPlaylistSurvivesLiveEncoderRewrites(t *testing.T) {
	_, directory, playlist := boundCopiedPlaylistFixture(t)
	variant := filepath.Join(directory, "360p", "index.m3u8")
	manifest, err := os.ReadFile(variant)
	if err != nil {
		t.Fatal(err)
	}
	ctx, stop := context.WithCancel(t.Context())
	rewritten := make(chan struct{})
	go func() {
		defer close(rewritten)
		for ctx.Err() == nil {
			_ = writeAtomicFile(variant, manifest)
		}
	}()
	codes := map[int]int{}
	for range 300 {
		codes[playlist("360p/index.m3u8").Code]++
	}
	stop()
	<-rewritten
	if codes[http.StatusOK] != 300 {
		t.Fatalf("variant playlist responses while the encoder replaced it = %v", codes)
	}
}
