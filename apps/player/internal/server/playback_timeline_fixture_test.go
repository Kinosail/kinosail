package server

import (
	"context"
	"net/http"

	"github.com/MikeO7/kinosail/packages/servertest"
)

var webPlaybackTimeline servertest.WebPlaybackFixture = func(enabled []string, owner bool) func(*http.Request, MediaFacts, *playerData) {
	settings := newSettingsStore("", "", "", nil)
	settings.value.AutoSkip = enabled
	return func(request *http.Request, facts MediaFacts, data *playerData) {
		if owner {
			request = request.WithContext(context.WithValue(request.Context(), viewerContextKey{}, viewerProfile{Owner: true}))
		}
		applyPlayback(request, settings, facts, data)
	}
}
