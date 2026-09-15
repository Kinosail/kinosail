package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/configuration"
	"github.com/MikeO7/kinosail-player/internal/database"
	"github.com/MikeO7/kinosail/packages/commandtest"
	"github.com/MikeO7/kinosail/packages/updatecontrol"
)

func updateReleasePolicy() updatecontrol.Policy {
	return updatecontrol.PlayerPolicy(database.SchemaVersion, configuration.SchemaVersion)
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
		Policy:      updateReleasePolicy(), Product: "player", SignatureIdentity: `^https://github\.com/MikeO7/kinosail/\.github/workflows/player-release\.yml@refs/tags/player-v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`,
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

func TestUpdateCommandsReportUnavailableOrCorruptStorage(t *testing.T) {
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
	if err = db.SaveJSON("settings.json", "invalid settings shape"); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
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
