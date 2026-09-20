package downloads

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/identitycore"
	"github.com/MikeO7/kinosail/packages/library"
)

type requestAccess struct {
	profile func(*http.Request) identitycore.Profile
	item    func(*http.Request, string) (library.Item, bool)
}

// NewAccess applies Player's offline-download policy to app-owned request state.
func NewAccess(profile func(*http.Request) identitycore.Profile, item func(*http.Request, string) (library.Item, bool)) Access {
	return requestAccess{profile: profile, item: item}
}

func (access requestAccess) Profile(request *http.Request) (string, bool) {
	if access.profile == nil {
		return "", false
	}
	profile := access.profile(request)
	return profile.ID, profile.Permits("download", profile.Owner || profile.Downloads)
}

func (access requestAccess) Item(request *http.Request, id string) (string, library.Item, bool) {
	if access.profile == nil || access.item == nil {
		return "", library.Item{}, false
	}
	profile := access.profile(request)
	item, found := access.item(request, id)
	return profile.ID, item, found && profile.Permits("download", profile.Owner || profile.Downloads)
}

func visibleJob(access Access, request *http.Request, profile string, job Job) bool {
	current, _, allowed := access.Item(request, job.ItemID)
	return allowed && current == profile && job.Profile == profile
}

func visibleJobs(access Access, request *http.Request, profile string, jobs []Job) []Job {
	visible := make([]Job, 0, len(jobs))
	for _, job := range jobs {
		if visibleJob(access, request, profile, job) {
			visible = append(visible, job)
		}
	}
	return visible
}
