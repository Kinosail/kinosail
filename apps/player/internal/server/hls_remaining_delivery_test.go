package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// Native immutable-media proofs do not schedule pathname replacement between
// validation and delivery. The Header hook deterministically schedules that
// HTTP-boundary race using mocked fragment bytes, without a live encoder.
func TestRemainingCachedDeliveryRetainsValidatedFragment(t *testing.T) {
	for _, substitution := range []string{"replacement", "symlink"} {
		for _, ranged := range []bool{false, true} {
			t.Run(substitution+"/"+map[bool]string{false: "full", true: "range"}[ranged], func(t *testing.T) {
				manager, item, recipe, directory := remainingAACAdmissionFixture(t)
				path := filepath.Join(directory, "audio/segment-00000.m4s")
				original, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				target := filepath.Join(t.TempDir(), "replacement-fragment")
				writeHLSLoadingFile(t, target, "unvalidated-replacement-payload")
				response := httptest.NewRecorder()
				writer := &remainingAACReplacementWriter{ResponseWriter: response, replace: func() {
					remainingAACSubstituteAsset(t, path, target, substitution)
				}}
				request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/"+item.ID+"/p/fixture/audio/segment-00000.m4s", nil)
				expected, status := original, http.StatusOK
				if ranged {
					request.Header.Set("Range", "bytes=3-8")
					expected, status = original[3:9], http.StatusPartialContent
				}
				manager.serveRecipe(writer, request, item, recipe, "audio/segment-00000.m4s")
				if !writer.changed || response.Code != status || !bytes.Equal(response.Body.Bytes(), expected) {
					t.Fatalf("validated fragment replaced during delivery: changed=%t status=%d wanted=%d original=%t", writer.changed, response.Code, status, bytes.Equal(response.Body.Bytes(), expected))
				}
				manager.mu.Lock()
				jobs := len(manager.jobs)
				manager.mu.Unlock()
				if jobs != 0 {
					t.Fatal("cached replacement control scheduled encoding")
				}
			})
		}
	}
}

type remainingAACReplacementWriter struct {
	http.ResponseWriter
	replace func()
	changed bool
}

func (writer *remainingAACReplacementWriter) Header() http.Header {
	if !writer.changed {
		writer.changed = true
		writer.replace()
	}
	return writer.ResponseWriter.Header()
}

func remainingAACSubstituteAsset(t *testing.T, path, target, substitution string) {
	t.Helper()
	if substitution == "replacement" {
		if err := os.Rename(target, path); err != nil {
			t.Fatal(err)
		}
		return
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
}
