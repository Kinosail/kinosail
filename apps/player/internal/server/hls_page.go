package server

import (
	"context"
	"net/http"
	"strings"
)

func hlsPageOwner(ctx context.Context) string {
	viewer, _ := ctx.Value(viewerContextKey{}).(viewerProfile)
	session := requestPlaybackSession(ctx)
	if viewer.ID == "" || !validPlaybackSession(session) {
		return ""
	}
	return viewer.ID + ":" + session
}

// The HLS mutex protects attachments; a new job is still private to its creator.
func retainHLSPage(job *hlsJob, ctx context.Context) {
	if !startupActualPlayback(ctx) {
		return
	}
	if job.pages == nil {
		job.pages = make(map[string]bool)
	}
	owner := hlsPageOwner(ctx)
	if retained, exists := job.pages[owner]; exists && !retained {
		return // A late segment request cannot reopen a departed page.
	}
	if len(job.pages) >= 64 && !job.pages[owner] {
		owner = "" // Preserve untracked consumers until the existing idle timeout.
	}
	job.pages[owner] = true
}

func (manager *hlsManager) stopHLSPage(request *http.Request) {
	owner, id := hlsPageOwner(request.Context()), request.PathValue("id")
	if owner == "" || id == "" {
		return
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	for key, job := range manager.jobs {
		if (key == id || strings.HasPrefix(key, id+"-")) && job.pages[owner] {
			job.pages[owner] = false
			active := false
			for _, retained := range job.pages {
				active = active || retained
			}
			if !active {
				job.cancel(errHLSInactive)
			}
		}
	}
}
