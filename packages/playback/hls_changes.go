package playback

import (
	"context"
	"path/filepath"
	"sync"
	"time"
)

const hlsPollInterval = 25 * time.Millisecond

type hlsChanges struct {
	ctx     context.Context
	hub     *hlsWatchHub
	changed chan struct{}
	wake    chan struct{}
	rewatch chan struct{}
	stop    chan struct{}
	joined  chan struct{}
	closed  sync.Once
	targets []string
	parents map[string]bool
}

func watchHLSChanges(ctx context.Context, directories []string) *hlsChanges {
	changes := &hlsChanges{ctx: context.WithoutCancel(ctx), hub: hlsWatches, changed: make(chan struct{}, 1), wake: make(chan struct{}, 1), rewatch: make(chan struct{}, 1), stop: make(chan struct{}), joined: make(chan struct{}), parents: map[string]bool{}}
	pending := changes.watch(directories)
	go changes.run(pending)
	return changes
}

func (changes *hlsChanges) run(pending []string) {
	defer close(changes.joined)
	ticker := time.NewTicker(hlsPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-changes.stop:
			return
		case <-changes.rewatch:
			pending = changes.watch(pending)
			continue
		case <-changes.wake:
		case <-ticker.C:
		}
		pending = changes.watch(pending)
		select {
		case changes.changed <- struct{}{}:
		default:
		}
	}
}

func (changes *hlsChanges) watch(pending []string) []string {
	remaining, needed := pending[:0:0], map[string]bool{}
	for _, directory := range pending {
		if changes.hub.subscribe(changes.ctx, changes, directory, false) {
			changes.targets = append(changes.targets, directory)
			continue
		}
		remaining, needed[filepath.Dir(directory)] = append(remaining, directory), true
	}
	for parent := range needed {
		if !changes.parents[parent] && changes.hub.subscribe(changes.ctx, changes, parent, true) {
			changes.parents[parent] = true
		}
	}
	for parent := range changes.parents {
		if !needed[parent] {
			changes.hub.unsubscribe(changes, parent, true)
			delete(changes.parents, parent)
		}
	}
	return remaining
}

func (changes *hlsChanges) Close() {
	changes.closed.Do(func() {
		close(changes.stop)
		<-changes.joined
		for _, directory := range changes.targets {
			changes.hub.unsubscribe(changes, directory, false)
		}
		for parent := range changes.parents {
			changes.hub.unsubscribe(changes, parent, true)
		}
		changes.targets, changes.parents = nil, nil
	})
}
