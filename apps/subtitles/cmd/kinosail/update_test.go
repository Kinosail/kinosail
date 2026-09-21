package main

import (
	"bytes"
	"database/sql"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail-subtitles/internal/database"
	"github.com/MikeO7/kinosail/packages/commandtest"
	"github.com/MikeO7/kinosail/packages/updatecontrol"
)

func updateReleasePolicy() updatecontrol.Policy {
	return updatecontrol.SubtitlesPolicy(database.SchemaVersion, configuration.SchemaVersion)
}

func writeUpdatePlan(output io.Writer, dataDir string) error {
	return updateCommand(updatecontrol.CommandPlan, nil, output, dataDir, "", "")
}

func recordUpdateReport(input io.Reader, dataDir string) error {
	return updateCommand(updatecontrol.CommandReport, input, nil, dataDir, "", "")
}

func updateTestContract() commandtest.UpdateContract {
	return commandtest.UpdateContract{
		Run:         updateCommand,
		StateSchema: database.SchemaVersion,
		Open:        func(directory string) (updatecontrol.CommandDatabase, error) { return database.Open(directory, false) },
		Policy:      updateReleasePolicy(), Product: "subtitles", SignatureIdentity: `^https://github\.com/Kinosail/kinosail/\.github/workflows/subtitles-release\.yml@refs/tags/subtitles-v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`,
	}
}

func TestUpdateCommandsExposeAVersionedAdapterContract(t *testing.T) {
	updateTestContract().Versioned(t)
}

func TestUpdateArtifactCommandSelectsVerifiedManifestEntry(t *testing.T) {
	updateTestContract().Artifact(t)
}

func TestUpdatePlanPinsManifestForPendingTarget(t *testing.T) {
	updateTestContract().PendingTarget(t)
}

func TestUpdateReportCommandRejectsUntrustedInput(t *testing.T) {
	updateTestContract().RejectedReport(t)
}

func TestUpdateCommandsReportUnavailableOrCorruptStorage(t *testing.T) { //nolint:cyclop // The table covers each storage failure boundary.
	var output bytes.Buffer
	if err := writeUpdatePlan(&output, ""); err == nil {
		t.Fatal("empty update storage was accepted")
	}
	if err := recordUpdateReport(strings.NewReader(`{"adapter":"linux","state":"current","currentVersion":"dev","checkedAt":"2026-08-27T12:00:00Z"}`), ""); err == nil {
		t.Fatal("update report without storage was accepted")
	}

	dataDir := t.TempDir()
	db, err := database.Open(dataDir, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.SaveJSON("settings.json", map[string]any{"subtitleLanguages": []string{"en"}}); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	raw, err := sql.Open("sqlite3", (&url.URL{Scheme: "file", Path: filepath.Join(dataDir, database.Filename)}).String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = raw.ExecContext(t.Context(), `UPDATE state SET value = ? WHERE name = 'settings.json'`, []byte(`"invalid settings shape"`)); err != nil {
		t.Fatal(err)
	}
	if err = raw.Close(); err != nil {
		t.Fatal(err)
	}
	if err = writeUpdatePlan(&output, dataDir); err == nil {
		t.Fatal("corrupt update settings were accepted")
	}

	blocked := filepath.Join(t.TempDir(), "data")
	if err = os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if openErr := writeUpdatePlan(&output, blocked); openErr == nil {
		t.Fatal("file update storage was accepted")
	}
}

func TestUpdateCommandsRejectSemanticInvalidSQLiteSettingsWithoutEffects(t *testing.T) {
	for _, action := range []updateAction{
		{"plan", func(dataDir string, output *bytes.Buffer) error { return writeUpdatePlan(output, dataDir) }},
		{"report", func(dataDir string, _ *bytes.Buffer) error {
			return recordUpdateReport(strings.NewReader(`{"adapter":"linux","state":"current","currentVersion":"dev","checkedAt":"2026-08-27T12:00:00Z"}`), dataDir)
		}},
	} {
		t.Run(action.name, func(t *testing.T) {
			assertSemanticInvalidSettingsRejected(t, action.run)
		})
	}
}

type updateAction struct {
	name string
	run  func(string, *bytes.Buffer) error
}

func assertSemanticInvalidSettingsRejected(t *testing.T, run func(string, *bytes.Buffer) error) { //nolint:cyclop // One invalid-settings matrix remains below the repository complexity ceiling.
	t.Helper()
	dataDir := t.TempDir()
	db, err := database.Open(dataDir, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.SaveJSON("settings.json", map[string]any{"subtitleLanguages": []string{"en"}}); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := sql.Open("sqlite3", (&url.URL{Scheme: "file", Path: filepath.Join(dataDir, database.Filename)}).String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = raw.ExecContext(t.Context(), `UPDATE state SET value = ? WHERE name = 'settings.json'`, []byte(`{"subtitleLanguages":[]}`)); err != nil {
		t.Fatal(err)
	}
	var before int
	if err = raw.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM state WHERE name = 'updates.json'`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err = raw.Close(); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err = run(dataDir, &output); err == nil || output.Len() != 0 {
		t.Fatalf("invalid settings action = %q, %v", output.String(), err)
	}
	raw, err = sql.Open("sqlite3", (&url.URL{Scheme: "file", Path: filepath.Join(dataDir, database.Filename)}).String())
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	var after int
	if err = raw.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM state WHERE name = 'updates.json'`).Scan(&after); err != nil || before != after {
		t.Fatalf("rejected action changed update state: before=%d after=%d error=%v", before, after, err)
	}
}
