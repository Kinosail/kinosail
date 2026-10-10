package server

import (
	"context"
	"net/http"
	"os"
	"runtime"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

// Gap: public HTTP cannot deterministically replace one exact missing file or
// marker after metadata admission and before the retained current fence.
type copiedAACPartialOpening interface {
	openCopiedHLSLegacyPartialGeneration(context.Context, library.Item, hlsRecipe, string) (*copiedAACGeneration, error)
}

func openCopiedAACPartialControl(t *testing.T, manager *hlsManager, item library.Item, recipe hlsRecipe, directory string) *copiedAACGeneration {
	t.Helper()
	opener, ok := any(manager).(copiedAACPartialOpening)
	if !ok {
		t.Fatal("partial generation opener unavailable")
	}
	held, err := opener.openCopiedHLSLegacyPartialGeneration(t.Context(), item, recipe, directory)
	if err != nil {
		t.Fatal("valid partial generation did not qualify")
	}
	t.Cleanup(held.close)
	if !held.current() {
		t.Fatal("valid retained partial control was not current")
	}
	return held
}

func TestCopiedAACPartialCompleteOpenerStaysCompleteOnly(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("retained Version1 reader is Linux-only")
	}
	for _, shape := range []string{"missing-later", "prefix", "speculative"} {
		t.Run(shape, func(t *testing.T) {
			manager, item, recipe, directory := copiedAACPartialFixture(t, shape)
			before := copiedAACPartialSnapshot(t, directory)
			held, err := manager.openCopiedHLSLegacyGeneration(t.Context(), item, recipe, directory)
			if err == nil {
				held.close()
				t.Fatal("complete-only opener admitted partial generation")
			}
			requireCopiedAACLegacyInventory(t, before, copiedAACPartialSnapshot(t, directory))
		})
	}
}

func TestCopiedAACPartialCurrentRejectsAdmittedStateChanges(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("retained Version1 reader is Linux-only")
	}
	rows := []struct{ shape, damage string }{
		{"missing-later", "later-arrived"},
		{"missing-later", "later-directory"},
		{"missing-later", "later-symlink"},
		{"missing-later", "startup-added"},
		{"speculative", "startup-removed"},
		{"speculative", "startup-replaced"},
		{"speculative", "startup-same-stat-value"},
		{"speculative", "source"},
		{"speculative", "source-root"},
		{"speculative", "generation"},
		{"prefix", "rendition"},
		{"complete", "later-remove"},
		{"complete", "later-replace"},
	}
	for _, row := range rows {
		t.Run(row.damage, func(t *testing.T) { requireCopiedAACPartialFence(t, row.shape, row.damage) })
	}
}

func requireCopiedAACPartialFence(t *testing.T, shape, damage string) {
	t.Helper()
	manager, item, recipe, directory := copiedAACPartialFixture(t, shape)
	held := openCopiedAACPartialControl(t, manager, item, recipe, directory)
	damageCopiedAACPartialFence(t, manager, item, directory, damage)
	before, source := copiedAACPartialSnapshot(t, directory), copiedAACPartialSource(t, item.Path)
	if held.current() {
		t.Error("changed admitted partial state remained current")
	}
	request := copiedAACPartialRequest(t.Context(), http.MethodGet, "360p/index.m3u8")
	if _, err := held.playlist("360p/index.m3u8", 0, 4, request); err == nil {
		t.Error("changed admitted partial state reached playlist")
	}
	file, _, _, err := held.openAsset("360p/init.mp4")
	if file != nil {
		_ = file.Close()
	}
	if err == nil {
		t.Error("changed admitted partial state reached asset delivery")
	}
	requireCopiedAACLegacyInventory(t, before, copiedAACPartialSnapshot(t, directory))
	requireCopiedAACPartialSource(t, source, item.Path)
	if len(manager.jobs) != 0 {
		t.Error("partial current rejection created a worker")
	}
	if err := held.ctx.Err(); err != nil {
		t.Error("partial fence fixture expired before qualification")
	}
	if _, err := os.Stat(item.Path); err != nil {
		t.Fatal("partial fence source fixture disappeared")
	}
}
