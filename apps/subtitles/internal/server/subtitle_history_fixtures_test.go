package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func writeSubtitleHistoryUIFixtures(t *testing.T, dir string) { //nolint:funlen // One visual fixture covers each recorded operation using the production template.
	t.Helper()
	manager, item := subtitleFactsFixture(t, "#!/bin/sh\nexit 1\n")
	item.Title = "Signal Garden"
	now := time.Now().Unix()
	evidence := []string{
		`"action":"recorded","source":"subdl","reason":"legacy","score":70,"releaseMatch":0.75`,
		`"action":"added","source":"subdl"`,
		`"action":"updated","source":"external","reason":"manual"`,
		`"action":"restored","source":"external","reason":"restore"`,
		`"action":"added","source":"embedded","reason":"embedded","score":100,"releaseMatch":1`,
		`"action":"updated","source":"opensubtitles","reason":"exact-hash","score":100,"releaseMatch":1`,
		`"action":"updated","source":"subsource","reason":"higher-score","score":92,"releaseMatch":1,"previousSource":"subdl","previousScore":72,"previousReleaseMatch":0.75`,
		`"action":"added","source":"opensubtitles","reason":"missing","score":88,"releaseMatch":0.95`,
	}
	for i, fields := range evidence {
		var event subtitleHistoryEvent
		if err := json.Unmarshal([]byte(fmt.Sprintf(`{"key":"%s:en","installedAt":%d,%s}`, item.ID, now-int64(len(evidence)-i)*3600, fields)), &event); err != nil {
			t.Fatal(err)
		}
		manager.provider.ledger.state.History = append(manager.provider.ledger.state.History, event)
	}
	data, err := manager.subtitleHistoryProjection(subtitleDashboardOptions{View: "history", Page: 1}, []library.Item{item})
	if err != nil {
		t.Fatal(err)
	}
	data.History[0].Title = "The Remarkably Long Journey Through the Quiet Gardens of the Northern Hemisphere"
	data.History[0].Context = "S06E10 · The last light before winter"
	for name, value := range map[string]subtitleDashboardData{"populated": data, "empty": {subtitleDashboardOptions: subtitleDashboardOptions{View: "history", Page: 1}}, "failed": {subtitleDashboardOptions: subtitleDashboardOptions{View: "history", Page: 1}, LoadFailed: true}} {
		response := httptest.NewRecorder()
		if err := subtitleDashboardView.Execute(response, ownerRequest("/?view=history"), value); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "subtitle-history-"+name+".html"), response.Body.Bytes(), 0o600); err != nil { //nolint:gosec // Fixed fixture names under the caller-selected test artifact directory.
			t.Fatal(err)
		}
	}
}
