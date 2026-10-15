package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/catalog"
	sharedoperations "github.com/MikeO7/kinosail/packages/operations"
)

var systemView = newLocalizedTemplate("system", sharedoperations.SystemViewSource(sharedoperations.SystemBrand{
	Product: "Subtitles", IconVersion: "8", ThemeVersion: "4", StylesheetVersion: "81",
}))

func showSystem(settings *settingsStore, index *libraryIndex, profiles *profileStore, audit *auditStore) http.HandlerFunc {
	return sharedoperations.NewSystemHandler(sharedoperations.SystemSource[catalog.PlaybackView]{
		Logs: audit.RecentLogs, Playback: func() []catalog.PlaybackView { return nil },
		Diagnostics: func() diagnosticReport { return buildDiagnosticReport(settings, index, profiles, audit) },
	}, systemView)
}
