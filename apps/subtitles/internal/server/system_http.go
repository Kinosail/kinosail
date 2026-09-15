package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/catalogapi"
	sharedoperations "github.com/MikeO7/kinosail/packages/operations"
)

var systemView = newLocalizedTemplate("system", sharedoperations.SystemViewSource(sharedoperations.SystemBrand{
	Product: "Subtitles", IconVersion: "8", ThemeVersion: "4", StylesheetVersion: "81",
}))

func showSystem(settings *settingsStore, index *libraryIndex, progress *progressStore, profiles *profileStore, hls *hlsManager, audit *auditStore) http.HandlerFunc {
	return sharedoperations.NewSystemHandler(sharedoperations.SystemSource[playbackView]{
		Logs: audit.RecentLogs, Playback: func() []playbackView { return catalogapi.RecentAdminProgress(progress, index, profiles.list()) },
		Diagnostics: func() diagnosticReport { return buildDiagnosticReport(settings, index, profiles, hls, audit) },
	}, systemView)
}
