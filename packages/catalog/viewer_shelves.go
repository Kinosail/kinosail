package catalog

import "github.com/MikeO7/kinosail/packages/library"

const recentPlaybackLimit = 20

type ViewerShelves struct {
	Active []ResumeItem
	Listed []library.Item
	Recent []library.Item
}

func (result Result) ViewerShelves(access BrowseAccess) ViewerShelves {
	access.ProgressMutex.Lock()
	defer access.ProgressMutex.Unlock()
	access.ListMutex.RLock()
	defer access.ListMutex.RUnlock()
	progress, listed := *access.Progress, *access.Listed
	isListed := func(id string) bool {
		var buffer [128]byte
		return listed[string(append(append(append(buffer[:0], access.ProfileID...), ':'), id...))]
	}
	relevant := make([]library.Item, 0)
	for _, item := range result.references {
		state := ViewerProgress(progress, access.ProfileID, access.Owner, item.ID)
		if state.Seconds > 0 || !state.Updated.IsZero() || isListed(item.ID) {
			relevant = append(relevant, *item)
		}
	}
	recent := ProgressHistory(progress, access.ProfileID, access.Owner, relevant)
	return ViewerShelves{
		Active: ActiveProgress(progress, access.ProfileID, access.Owner, relevant),
		Listed: ListedItems(relevant, isListed),
		Recent: recent[:min(recentPlaybackLimit, len(recent))],
	}
}
