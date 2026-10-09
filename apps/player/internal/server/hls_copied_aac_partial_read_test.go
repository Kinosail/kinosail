package server

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// Failures: complete-only admission rejects later-cut gaps and a valid startup
// marker. A genuine request cannot deterministically hold the metadata boundary.
func TestCopiedAACPartialReadMatchesCompleteResponsesWithoutSideEffects(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("retained Version1 reader is Linux-only")
	}
	for _, shape := range []string{"missing-later", "prefix", "speculative"} {
		t.Run(shape, func(t *testing.T) {
			for _, name := range []string{"index.m3u8", "360p/index.m3u8", "360p/init.mp4", "360p/segment-00000.m4s"} {
				for _, method := range []string{http.MethodGet, http.MethodHead} {
					ranges := []string{""}
					if filepath.Ext(name) != ".m3u8" {
						ranges = append(ranges, "bytes=0-3")
					}
					for _, rangeValue := range ranges {
						label := name + "/" + method
						if rangeValue != "" {
							label += "/Range"
						}
						t.Run(label, func(t *testing.T) {
							manager, item, recipe, directory := copiedAACPartialFixture(t, "complete")
							control := copiedAACPartialBaseline(t, manager, item, recipe, name, method, rangeValue)
							if err := os.Remove(filepath.Join(directory, "360p/segment-00001.m4s")); err != nil {
								t.Fatal(err)
							}
							if shape != "missing-later" {
								_, _, _, prefixDirectory := copiedAACPartialFixture(t, shape)
								data, err := os.ReadFile(filepath.Join(prefixDirectory, "360p/index.m3u8"))
								if err != nil {
									t.Fatal(err)
								}
								writeHLSLoadingFile(t, filepath.Join(directory, "360p/index.m3u8"), string(data))
							}
							if shape == "speculative" {
								writeHLSLoadingFile(t, filepath.Join(directory, ".startup"), "1")
							}
							before, source := copiedAACPartialSnapshot(t, directory), copiedAACPartialSource(t, item.Path)
							descriptors, err := os.ReadDir("/proc/self/fd")
							if err != nil {
								t.Fatal(err)
							}
							result := copiedAACPartialResponse(t, manager, item, recipe, name, method, rangeValue)
							if result.Code != control.Code || result.Body.String() != control.Body.String() ||
								result.Header().Get("Content-Type") != control.Header().Get("Content-Type") ||
								result.Header().Get("Content-Range") != control.Header().Get("Content-Range") {
								t.Errorf("partial read status/body/headers differ from qualified complete control: status=%d wanted=%d", result.Code, control.Code)
							}
							requireCopiedAACLegacyInventory(t, before, copiedAACPartialSnapshot(t, directory))
							requireCopiedAACPartialSource(t, source, item.Path)
							after, err := os.ReadDir("/proc/self/fd")
							if err != nil || len(descriptors) != len(after) {
								t.Error("partial read did not release descriptors")
							}
							if len(manager.jobs) != 0 {
								t.Error("partial metadata or retained asset started a worker")
							}
							options, err := manager.hlsSettings(item, recipe)
							if err != nil || !copiedAACPolicyRequired(options.Cache) {
								t.Error("partial read erased sticky Version2 eligibility")
							}
							t.Logf("partial_read shape=%s method=%s ranged=%t status=%d source_unchanged=true cache_unchanged=true", shape, method, rangeValue != "", result.Code)
						})
					}
				}
			}
		})
	}
}

func TestCopiedAACPartialSlowNetworkReleasesLeaseAndKeepsOpenedBytes(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("retained Version1 reader is Linux-only")
	}
	manager, item, recipe, directory := copiedAACPartialFixture(t, "speculative")
	expected, err := os.ReadFile(filepath.Join(directory, "360p/init.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	owner := newCopiedAACDelivery(t)
	request := copiedAACPartialRequest(owner.ctx, http.MethodGet, "360p/init.mp4")
	go func() {
		owner.done <- manager.serveCopiedHLSLegacy(owner.writer, request, item, recipe, "360p/init.mp4")
	}()
	owner.waitEntered(t, "partial retained delivery did not reach network boundary")
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	_, release, err := manager.copiedHLSMetadataAdmission(ctx)
	if err != nil {
		t.Fatal("partial network delivery retained metadata lease")
	}
	release()
	path := filepath.Join(directory, "360p/init.mp4")
	if err := os.Rename(path, path+"-retired"); err != nil {
		t.Fatal(err)
	}
	writeHLSLoadingFile(t, path, "replacement bytes")
	owner.release()
	owner.join(t, "partial retained delivery did not join with HTTP200")
	if owner.writer.Body.String() != string(expected) {
		t.Fatal("partial delivery reopened replaced canonical init")
	}
}
