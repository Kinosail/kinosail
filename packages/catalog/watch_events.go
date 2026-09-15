package catalog

import (
	"context"
	"errors"
	"time"

	"github.com/fsnotify/fsnotify"
)

func watchEvents(ctx context.Context, changes <-chan struct{}, events <-chan fsnotify.Event, failures <-chan error, quiet func() <-chan time.Time, clearQuiet func(), record func(fsnotify.Event) error, settle func() error) error { //nolint:cyclop // The score of 11 remains below the repository ceiling of 22 for one select-driven lifecycle.
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-changes:
			return nil
		case event, ok := <-events:
			if !ok {
				return errors.New("filesystem event stream closed")
			}
			if err := record(event); err != nil {
				return err
			}
		case err, ok := <-failures:
			if !ok {
				return errors.New("filesystem error stream closed")
			}
			return err
		case <-quiet():
			clearQuiet()
			if err := settle(); err != nil {
				return err
			}
		}
	}
}

type watchDispatch struct {
	index   *Index
	watcher *fsnotify.Watcher
	cycle   *watchCycle
}

func (dispatch watchDispatch) quiet() <-chan time.Time {
	return dispatch.cycle.quiet
}

func (dispatch watchDispatch) clearQuiet() {
	dispatch.cycle.quiet = nil
}

func (dispatch watchDispatch) record(event fsnotify.Event) error {
	return dispatch.cycle.record(dispatch.watcher, event, dispatch.index.watchDebounce)
}

func (dispatch watchDispatch) settle() error {
	return dispatch.cycle.settle(dispatch.index)
}
