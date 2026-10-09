package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"testing"
)

func copiedAACPartialRequest(ctx context.Context, method, name string) *http.Request {
	return httptest.NewRequestWithContext(ctx, method, "/"+name, nil)
}

// Paired otherwise-valid complete and partial controls protect fail-closed
// metadata/root/source rejection and pure reads beside an unrelated live owner.
func TestCopiedAACPartialReadRejectsDamageWithoutPreparation(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("retained Version1 reader is Linux-only")
	}
	for _, damage := range []string{"first-cut", "init", "binding", "timeline", "certificate", "master",
		"source-root", "startup-value", "startup-empty", "startup-overflow", "startup-directory",
		"startup-symlink", "later-directory", "later-symlink"} {
		t.Run(damage, func(t *testing.T) { requireCopiedAACPartialReject(t, damage) })
	}
}

func requireCopiedAACPartialReject(t *testing.T, damage string) {
	t.Helper()
	manager, item, recipe, directory := copiedAACPartialFixture(t, "speculative")
	damageCopiedAACPartial(t, manager, item.Path, directory, damage)
	before, source := copiedAACPartialSnapshot(t, directory), copiedAACPartialSource(t, item.Path)
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		result := copiedAACPartialResponse(t, manager, item, recipe, "index.m3u8", method, "")
		if result.Code != http.StatusNotFound {
			t.Error("damaged partial cache reached delivery")
		}
	}
	requireCopiedAACLegacyInventory(t, before, copiedAACPartialSnapshot(t, directory))
	requireCopiedAACPartialSource(t, source, item.Path)
	if len(manager.jobs) != 0 {
		t.Fatal("partial rejection started preparation")
	}
}

func TestCopiedAACPartialReadDoesNotAdoptOrCancelForeignJob(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("retained Version1 reader is Linux-only")
	}
	manager, item, recipe, directory := copiedAACPartialFixture(t, "speculative")
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(context.Canceled)
	cancellations := 0
	job := &hlsJob{lifecycle: ctx, done: make(chan struct{}), cachePolicy: "unrelated-policy",
		cancel: func(cause error) { cancellations++; cancel(cause) }}
	key := hlsRecipeKey(item.ID, recipe)
	manager.jobs[key] = job
	before := copiedAACPartialSnapshot(t, directory)
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		result := copiedAACPartialResponse(t, manager, item, recipe, "index.m3u8", method, "")
		if result.Code != http.StatusOK {
			t.Errorf("valid read beside a foreign owner returned%d", result.Code)
		}
	}
	if len(manager.jobs) != 1 || manager.jobs[key] != job || cancellations != 0 || ctx.Err() != nil {
		t.Fatal("partial read replaced, adopted or canceled foreign owner")
	}
	requireCopiedAACLegacyInventory(t, before, copiedAACPartialSnapshot(t, directory))
}

func TestCopiedAACPartialCanceledReadReleasesResources(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("retained Version1 reader is Linux-only")
	}
	manager, item, recipe, directory := copiedAACPartialFixture(t, "speculative")
	before := copiedAACPartialSnapshot(t, directory)
	descriptors, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	writer := httptest.NewRecorder()
	if !manager.serveCopiedHLSLegacy(writer, copiedAACPartialRequest(ctx, http.MethodGet, "index.m3u8"), item, recipe, "index.m3u8") ||
		writer.Code != http.StatusNotFound {
		t.Fatal("canceled partial request reached delivery")
	}
	requireCopiedAACLegacyInventory(t, before, copiedAACPartialSnapshot(t, directory))
	after, err := os.ReadDir("/proc/self/fd")
	if err != nil || len(after) != len(descriptors) || len(manager.jobs) != 0 {
		t.Fatal("canceled partial read retained resources or created a worker")
	}
}
