package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Real-media E2E covers healthy refill completion. These public-handler controls
// cover canceled ownership at a precisely held join; an external encoder
// cannot deterministically arrange this cancellation and completion ordering.
func TestMissingHLSSegmentDoesNotReplaceCanceledProducer(t *testing.T) {
	for _, cause := range []error{errHLSInactive, errHLSSeekRestart, errHLSIdentityChanged, context.Canceled} {
		t.Run(cause.Error(), func(t *testing.T) {
			manager, item, recipe, directory, initialization := copiedRecoveryEnclosingFixture(t)
			retained := copiedStartupRetained(t, directory, item.Path)
			marker := filepath.Join(t.TempDir(), "refills")
			copiedRecoveryEncoderOutput(t, manager, "printf started > "+copiedRecoveryQuote(marker), initialization, "first fragment", copiedRecoveryManifest)
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			_, job := manager.newHLSJob(ctx, 0)
			defer job.cancel(nil)
			options, err := manager.hlsSettings(item, recipe)
			if err != nil {
				t.Fatal(err)
			}
			job.cachePolicy = options.Cache
			key := hlsRecipeKey(item.ID, recipe)
			manager.jobs[key] = job
			response := httptest.NewRecorder()
			done := make(chan struct{})
			go func() {
				defer close(done)
				request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/hls/segment", nil)
				manager.serveRecipe(response, request, item, recipe, "1080p/segment-00000.m4s")
			}()
			select {
			case <-done:
				t.Fatal("request completed before producer settled")
			case <-time.After(100 * time.Millisecond):
			}
			job.cancel(cause)
			manager.finishHLSEncode(job, key, directory, 0, true)
			<-done
			if response.Code != http.StatusNotFound {
				t.Fatalf("canceled segment status = %d", response.Code)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("canceled producer started another encoder")
			}
			retained()
		})
	}
}
