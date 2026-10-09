package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// Gap: external media HTTP cannot pause the producer between master publication
// and clock commit. This is a metadata scheduling control, not source media proof.
func copiedAACPendingMasterFixture(t *testing.T, certificatePublished bool) (*hlsManager, string, *copiedAACGeneration, *hlsJob, func()) {
	t.Helper()
	manager, directory, held := copiedAACGenerationFixture(t)
	timeline := *held.timeline
	origin := *held.timeline.AudioOrigin
	origin.FirstHash, origin.FirstPTS, origin.Physical, origin.Edit = "", 0, 0, 0
	timeline.Clock, timeline.AudioOrigin = nil, &origin
	pending, err := json.Marshal(timeline)
	if err != nil {
		t.Fatal(err)
	}
	held.close()
	writeHLSLoadingFile(t, filepath.Join(directory, ".copy-timeline"), string(pending))
	if !certificatePublished {
		if err := os.Remove(filepath.Join(directory, ".copy-clock")); err != nil {
			t.Fatal(err)
		}
	}
	lifecycle, cancel := context.WithCancelCause(t.Context())
	manager.ctx = lifecycle
	job := &hlsJob{lifecycle: lifecycle, done: make(chan struct{}), cancel: cancel,
		activity: make(chan struct{}, 1), cachePolicy: held.policy,
		observation: newHLSObservation("", 0)}
	key := hlsRecipeKey(held.item.ID, held.recipe)
	manager.jobs[key] = job
	t.Cleanup(func() {
		cancel(context.Canceled)
		manager.mu.Lock()
		delete(manager.jobs, key)
		manager.mu.Unlock()
		close(job.done)
	})
	commit := func() {
		// Match production's certificate-first, bound-timeline-second commit.
		writeHLSLoadingFile(t, filepath.Join(directory, ".copy-clock"), string(held.certificateData))
		writeHLSLoadingFile(t, filepath.Join(directory, ".copy-timeline"), string(held.timelineData))
	}
	return manager, directory, held, job, commit
}

func TestCopiedAACPendingMasterKeepsGETAndHEADAdmission(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("qualified retained-source producer is Linux-only")
	}
	for _, certificatePublished := range []bool{false, true} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			name := method + "-pending"
			if certificatePublished {
				name += "-certificate-written"
			}
			t.Run(name, func(t *testing.T) {
				manager, directory, held, job, commit := copiedAACPendingMasterFixture(t, certificatePublished)
				before := copiedAACPlaylistSnapshot(t, directory)
				admitted, continueRequest := make(chan error, 1), make(chan struct{})
				var resumeOnce sync.Once
				resume := func() { resumeOnce.Do(func() { close(continueRequest) }) }
				server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					// Pause at the existing preparation boundary with a pending clock.
					err := manager.copiedAACPlaylistBinding(request.Context(), held.item, held.recipe, filepath.Base(directory))
					admitted <- err
					if err != nil {
						writer.WriteHeader(http.StatusNotFound)
						return
					}
					select {
					case <-continueRequest:
					case <-request.Context().Done():
						return
					}
					manager.serveRecipe(writer, request, held.item, held.recipe, "index.m3u8")
				}))
				t.Cleanup(server.Close)
				t.Cleanup(resume)
				type reply struct {
					status int
					body   []byte
					err    error
				}
				done := make(chan reply, 1)
				go func() {
					request, err := http.NewRequestWithContext(t.Context(), method, server.URL+"/index.m3u8", nil)
					if err != nil {
						done <- reply{err: err}
						return
					}
					client := &http.Client{Timeout: 3 * time.Second}
					response, err := client.Do(request)
					if err != nil {
						done <- reply{err: err}
						return
					}
					defer response.Body.Close()
					data, err := io.ReadAll(io.LimitReader(response.Body, (256<<10)+1))
					done <- reply{response.StatusCode, data, err}
				}()
				select {
				case err := <-admitted:
					if err != nil {
						t.Fatal("valid published master rejected its pending clock interval")
					}
				case <-time.After(time.Second):
					t.Fatal("pending master preflight waited for clock certification")
				}
				key := filepath.Base(directory)
				if !reflect.DeepEqual(before, copiedAACPlaylistSnapshot(t, directory)) || manager.jobs[key] != job {
					t.Fatal("pending admission changed the generation or initial owner")
				}
				commit()
				resume()
				select {
				case result := <-done:
					if result.err != nil || result.status != http.StatusOK || len(result.body) > 256<<10 {
						t.Fatal("clock commit did not retain successful HTTP delivery")
					}
					if method == http.MethodHead && len(result.body) != 0 || method == http.MethodGet && !strings.HasPrefix(string(result.body), "#EXTM3U\n") {
						t.Fatal("pending GET or HEAD delivery changed its transport contract")
					}
				case <-time.After(4 * time.Second):
					t.Fatal("committed pending HTTP request did not join")
				}
				if manager.jobs[key] != job {
					t.Fatal("pending request replaced its initial owner")
				}
			})
		}
	}
}

// Gap: a real client cannot freeze that same pending generation while corrupting
// its already-published master. Preflight must reject before any preparation.
func TestCopiedAACPendingMalformedMasterRejectsWithoutPreparation(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("qualified retained-source producer is Linux-only")
	}
	for _, certificatePublished := range []bool{false, true} {
		manager, directory, held, job, _ := copiedAACPendingMasterFixture(t, certificatePublished)
		path := filepath.Join(directory, "index.m3u8")
		original, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		malformed := strings.Replace(string(original), "#EXT-X-STREAM-INF:", "#EXT-X-STSEAM-INF:", 1)
		if malformed == string(original) {
			t.Fatal("pending master fault was not applied")
		}
		writeHLSLoadingFile(t, path, malformed)
		before := copiedAACPlaylistSnapshot(t, directory)
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			request := httptest.NewRequestWithContext(t.Context(), method, "/index.m3u8", nil)
			err := manager.copiedAACPlaylistBinding(request.Context(), held.item, held.recipe, filepath.Base(directory))
			if !errors.Is(err, errCopiedHLSIndex) {
				t.Error("malformed present pending master acquired preparation admission")
			}
			if !reflect.DeepEqual(before, copiedAACPlaylistSnapshot(t, directory)) || len(manager.jobs) != 1 || manager.jobs[filepath.Base(directory)] != job {
				t.Fatal("rejected pending master changed its cache or initial owner")
			}
		}
	}
}
