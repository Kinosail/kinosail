package viewing

import (
	"context"
	"net/http"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
)

func MergeProgress(values map[string]catalog.PlaybackState, profileID string, owner bool, changes []ProgressChange, now time.Time) (map[string]catalog.PlaybackState, int, int) {
	updates, applied, conflicts := progressUpdates(values, profileID, owner, changes, now)
	return mergedProgress(values, updates), applied, conflicts
}

func fetchPlexSection(ctx context.Context, client *http.Client, input Input, key, kind string, result []Activity) ([]Activity, error) { //nolint:unparam // Direct source validation retains the section kind boundary.
	return fetchPlexSectionReserved(ctx, client, input, key, kind, result, nil)
}
