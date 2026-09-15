package viewing

import (
	"context"
	"crypto/rand"
	"errors"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

// Engine owns the source-neutral preview lifecycle while an app supplies policy adapters.
type Engine[P any] struct {
	Store         PreviewStore
	TTL           time.Duration
	Validate      func(Input) (Input, P, error)
	Fetch         func(context.Context, Input) ([]Activity, error)
	Snapshot      func() ([]library.Item, error)
	Profile       func(string) (P, bool)
	ProfileID     func(P) string
	ProfileName   func(P) string
	Progress      func(P, string) catalog.PlaybackState
	ListAdditions func(P, string, bool, map[string]int) int
	Commit        func(P, []ProgressChange, []ListChange) (int, int, int, error)
}

// Preview validates the source before fetching or reading destination state.
func (engine Engine[P]) Preview(ctx context.Context, input Input) (Preview, error) {
	input, profile, err := engine.Validate(input)
	if err != nil {
		return Preview{}, err
	}
	return Prepare(ctx, engine.Store, func(ctx context.Context) ([]Activity, error) { return engine.Fetch(ctx, input) }, engine.Snapshot, func(activities []Activity, items []library.Item) Preview {
		return engine.Build(input, profile, activities, items)
	})
}

// Build creates one expiring optimistic preview using destination policy callbacks.
func (engine Engine[P]) Build(input Input, profile P, activities []Activity, items []library.Item) Preview {
	return NewPreview(rand.Text(), time.Now().Add(engine.TTL), input, engine.ProfileID(profile), engine.ProfileName(profile), activities, items, func(id string) catalog.PlaybackState {
		return engine.Progress(profile, id)
	}, func(id string, favorite bool, playlists map[string]int) int {
		return engine.ListAdditions(profile, id, favorite, playlists)
	})
}

// Apply durably commits and consumes a current preview.
func (engine Engine[P]) Apply(id string) (Summary, error) {
	return ApplyPreview(engine.Store, id, func(preview Preview) (int, int, int, error) {
		profile, found := engine.Profile(preview.ProfileID)
		if !found {
			return 0, 0, 0, errors.New("destination Viewer Profile was not found")
		}
		return engine.Commit(profile, preview.Changes, preview.ListChanges)
	})
}
