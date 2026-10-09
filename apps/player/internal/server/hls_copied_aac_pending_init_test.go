package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// Gap: external HTTP cannot freeze master publication before clock certification.
// A grammar-valid pending master must match its opened init before preparation,
// even while the bound certificate is absent or written ahead of the timeline.
func TestCopiedAACPendingInitMismatchRejectsBeforePreparation(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("qualified retained-source producer is Linux-only")
	}
	faults := []struct {
		name, before, after string
	}{
		{"codecs", `CODECS="avc1.`, `CODECS="avc3.`},
		{"dimensions", "RESOLUTION=320x180", "RESOLUTION=321x180"},
		{"range", "VIDEO-RANGE=SDR", "VIDEO-RANGE=HLG"},
	}
	for _, certificatePublished := range []bool{false, true} {
		for _, fault := range faults {
			name := fault.name + "-pending"
			if certificatePublished {
				name += "-certificate-written"
			}
			t.Run(name, func(t *testing.T) {
				copiedAACPendingInitFault(t, certificatePublished, fault.before, fault.after)
			})
		}
	}
}

func copiedAACPendingInitFault(t *testing.T, certificatePublished bool, before, after string) {
	t.Helper()
	manager, directory, held, job, _ := copiedAACPendingMasterFixture(t, certificatePublished)
	path := filepath.Join(directory, "index.m3u8")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	master := strings.Replace(string(original), before, after, 1)
	if master == string(original) {
		t.Fatal("pending init consistency fault was not applied")
	}
	entry, valid := parseCopiedAACMaster([]byte(master), held.policy)
	if !valid || entry.rendition != held.certificate.Rendition {
		t.Fatal("pending init fault changed the independent structural control")
	}
	writeHLSLoadingFile(t, path, master)
	snapshot := copiedAACPlaylistSnapshot(t, directory)
	key := filepath.Base(directory)
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		request := httptest.NewRequestWithContext(t.Context(), method, "/index.m3u8", nil)
		err := manager.copiedAACPlaylistBinding(request.Context(), held.item, held.recipe, key)
		if !errors.Is(err, errCopiedHLSIndex) {
			t.Error("init-mismatched pending master acquired preparation admission")
		}
		assertCopiedAACPendingInitOwner(t, manager, directory, snapshot, job)

	}
}

func assertCopiedAACPendingInitOwner(t *testing.T, manager *hlsManager, directory string, snapshot map[string][32]byte, job *hlsJob) {
	t.Helper()
	if !reflect.DeepEqual(snapshot, copiedAACPlaylistSnapshot(t, directory)) ||
		len(manager.jobs) != 1 || manager.jobs[filepath.Base(directory)] != job || startupJobStopping(job) {
		t.Fatal("pending init rejection changed its cache or initial owner")
	}
}
