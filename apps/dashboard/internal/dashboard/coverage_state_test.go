package dashboard

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCatalogKinosailApplications(t *testing.T) {
	var ids []string
	for _, entry := range Catalog() {
		if entry.Category == "Kinosail" {
			ids = append(ids, entry.ID)
		}
	}
	want := []string{"kinosail-player", "kinosail-subtitles", "kinosail-dashboard"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("Kinosail catalog = %v, want %v", ids, want)
	}
	for id := range CatalogAliases() {
		if _, ok := catalogByID(id); !ok {
			t.Fatalf("alias has no catalog entry: %s", id)
		}
	}
}

func TestCoverageCatalogCopiesAndDefaults(t *testing.T) {
	entries := Catalog()
	original := entries[0]
	entries[0].Name = "changed"
	if Catalog()[0] != original {
		t.Fatal("catalog leaked mutable storage")
	}
	aliases := CatalogAliases()
	aliases["jellyfin"][0] = "changed"
	delete(aliases, "plex")
	if got := CatalogAliases(); got["jellyfin"][0] == "changed" || len(got["plex"]) == 0 {
		t.Fatal("aliases leaked mutable storage")
	}
	app, err := normalizeCreate(CreateInput{CatalogID: original.ID, URL: "https://media.test"})
	if err != nil {
		t.Fatal(err)
	}
	if app.Name != original.Name || app.Description != original.Description || app.Category != original.Category || app.Icon != original.Icon || app.Accent != original.Accent {
		t.Fatalf("catalog defaults = %+v, want %+v", app, original)
	}
}

func TestCoverageFieldValidation(t *testing.T) {
	for name, input := range map[string]CreateInput{
		"health":   {Name: "Media", URL: "https://media.test", HealthURL: "ftp://health.test"},
		"category": {Name: "Media", URL: "https://media.test", Category: strings.Repeat("x", 41)},
		"control":  {Name: "M\x00edia", URL: "https://media.test"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := normalizeCreate(input); err == nil {
				t.Fatal("invalid field accepted")
			}
		})
	}
	for _, address := range []string{"https://media.test:0", "https://media.test:65536"} {
		if _, err := normalizeURL("url", address, true); err == nil {
			t.Fatalf("invalid port accepted: %s", address)
		}
	}
	if address, err := normalizeURL("url", "", false); err != nil || address != "" {
		t.Fatalf("optional URL = %q, %v", address, err)
	}
	if address, err := normalizeURL("url", "http://[::1]:80", true); err != nil || address != "http://[::1]" {
		t.Fatalf("IPv6 URL = %q, %v", address, err)
	}
}

func TestCoverageCompleteUpdateFields(t *testing.T) {
	service, _ := serviceForTest(t, testBoard(testApp(1, "https://media.test", false)))
	name, address, health := "New", "https://new.test", "https://health.test"
	description, category, icon, accent := "Description", "Media", "video", "lime"
	enabled, favorite := true, true
	input := UpdateInput{Name: &name, URL: &address, HealthURL: &health, Description: &description, Category: &category, Icon: &icon, Accent: &accent, CheckEnabled: &enabled, Favorite: &favorite, ExpectedVersion: 1}
	app, _, err := service.Update(t.Context(), service.board.Apps[0].ID, input, "Owner")
	if err != nil {
		t.Fatal(err)
	}
	want := CreateInput{Name: name, URL: address, HealthURL: health, Description: description, Category: category, Icon: icon, Accent: accent, CheckEnabled: enabled, Favorite: favorite}
	got := CreateInput{}
	applyUpdate(&got, input)
	if !reflect.DeepEqual(got, want) || app.Name != name || app.HealthURL != health || !app.Favorite || !app.CheckEnabled {
		t.Fatalf("patch = %+v, app %+v", got, app)
	}
}

func TestCoverageAuditValidation(t *testing.T) {
	valid := AuditEvent{Action: "app.added", Subject: "Media", Actor: "Owner", At: time.Now()}
	for name, change := range map[string]func(*AuditEvent){
		"action":  func(event *AuditEvent) { event.Action = "invalid" },
		"subject": func(event *AuditEvent) { event.Subject = "" },
		"actor":   func(event *AuditEvent) { event.Actor = "" },
	} {
		t.Run(name, func(t *testing.T) {
			event := valid
			change(&event)
			if err := validateAudit([]AuditEvent{event}); err == nil {
				t.Fatal("invalid audit accepted")
			}
		})
	}
	if err := validateAudit([]AuditEvent{valid}); err != nil {
		t.Fatal(err)
	}
	service, _ := serviceForTest(t, testBoard())
	for range 101 {
		service.record(valid.Action, valid.Subject, " ", valid.At)
	}
	if len(service.board.Audit) != 100 || service.board.Audit[0].Actor != "Owner" {
		t.Fatalf("bounded audit = %+v", service.board.Audit)
	}
}

func TestCoveragePersistedBoardValidation(t *testing.T) {
	board := testBoard()
	board.Title = ""
	if err := validatePersistedBoard(board, time.Now()); err == nil {
		t.Fatal("empty persisted title accepted")
	}
	app := testApp(1, "https://media.test", false)
	board = testBoard()
	board.Removed = []RemovedApp{{App: app, Position: -1, RemovedAt: board.UpdatedAt}}
	if err := validatePersistedBoard(board, time.Now()); err == nil {
		t.Fatal("invalid recovery position accepted")
	}
}

func TestCoverageSnapshotHealthSummary(t *testing.T) {
	states := []string{"reachable", "slow", "degraded", "unavailable", "disabled", "unchecked"}
	var summary Summary
	for _, state := range states {
		incrementSummary(&summary, state)
	}
	if summary != (Summary{Total: 6, Reachable: 1, Slow: 1, Degraded: 1, Unavailable: 1, Disabled: 1, Unchecked: 1}) {
		t.Fatalf("summary = %+v", summary)
	}
	app := testApp(1, "https://media.test", true)
	service, _ := serviceForTest(t, testBoard(app))
	service.health[app.ID] = Health{State: "reachable", CheckedAt: time.Now().Add(-3 * time.Minute)}
	if got := service.Snapshot().Apps[0].Health; got.State != "unchecked" || got.Explanation != "Last observation is stale" {
		t.Fatalf("stale health = %+v", got)
	}
}
