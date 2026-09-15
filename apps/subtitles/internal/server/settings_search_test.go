package server_test

import (
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestSettingsExposeSmartSearch(t *testing.T) {
	servertest.SettingsExposeSmartSearch(t, settingsSearchHandler)
}

func TestSettingsBookmarksFollowTheRenderedSectionOrder(t *testing.T) {
	servertest.SettingsBookmarksFollowTheRenderedSectionOrder(t, settingsSearchHandler)
}

func settingsSearchHandler(dataDir string) http.Handler {
	return server.New(server.Config{DataDir: dataDir})
}
