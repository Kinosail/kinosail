package commandtest

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/updatecontrol"
)

// UpdateContract runs the same release-command scenarios against each application.
type UpdateContract struct {
	Run                        updatecontrol.Command
	Open                       func(string) (updatecontrol.CommandDatabase, error)
	Policy                     updatecontrol.Policy
	Product, SignatureIdentity string
	StateSchema                int
}

func (contract UpdateContract) writePlan(output io.Writer, directory string) error {
	return contract.Run(updatecontrol.CommandPlan, nil, output, directory, "", "")
}

func (contract UpdateContract) writeArtifact(input io.Reader, output io.Writer, goos, arch string) error {
	return contract.Run(updatecontrol.CommandArtifact, input, output, "", goos, arch)
}

func (contract UpdateContract) recordReport(input io.Reader, directory string) error {
	return contract.Run(updatecontrol.CommandReport, input, nil, directory, "", "")
}

func (contract UpdateContract) Versioned(t *testing.T) { //nolint:cyclop // One adapter contract remains below the repository complexity ceiling.
	dataDir := t.TempDir()
	var output bytes.Buffer
	if err := contract.writePlan(&output, dataDir); err != nil {
		t.Fatal(err)
	}
	var plan updatecontrol.Plan
	if err := json.Unmarshal(output.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.SchemaVersion != updatecontrol.PlanSchemaVersion || plan.StateSchema != contract.StateSchema || plan.ConfigurationSchema != 1 || plan.SignatureIdentity != contract.SignatureIdentity || plan.Automatic || !plan.RequireSignature || !plan.BackupBeforeInstall || !plan.DeferWhilePlaying || !plan.RollbackOnUnhealthy || !plan.Recovery.IncludesSecrets || !plan.Recovery.RequiresKey || !plan.Recovery.RestoreOnRollback {
		t.Fatalf("update plan = %#v", plan)
	}
	db, err := contract.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveJSON("settings.json", map[string]any{"updateChecks": true}); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	output.Reset()
	if err := contract.writePlan(&output, dataDir); err != nil || json.Unmarshal(output.Bytes(), &plan) != nil || !plan.Automatic {
		t.Fatalf("automatic update plan = %#v, %v", plan, err)
	}
	contract.reportContract(t, dataDir)
}

func (contract UpdateContract) reportContract(t *testing.T, dataDir string) {
	t.Helper()
	report := `{"adapter":"linux","state":"current","currentVersion":"dev","checkedAt":"2026-08-27T12:00:00Z"}`
	if err := contract.recordReport(strings.NewReader(report), dataDir); err != nil {
		t.Fatal(err)
	}
	db, err := contract.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := updatecontrol.New(db, contract.Policy)
	if err != nil {
		t.Fatal(err)
	}
	view, err := store.View("dev")
	if err != nil || view.Adapter != "linux" || view.Status != "current" {
		t.Fatalf("update view = %#v, %v", view, err)
	}
}

func (contract UpdateContract) Artifact(t *testing.T) {
	manifest := updatecontrol.Manifest{SchemaVersion: updatecontrol.ManifestSchemaVersion, Version: "v1.2.3", UpdateSchema: updatecontrol.PlanSchemaVersion, StateSchema: contract.StateSchema, MinimumStateSchema: contract.StateSchema, ConfigurationSchema: 1, MinimumConfigurationSchema: 1, RuntimeCommands: []string{"ffmpeg", "ffprobe", "fpcalc"}, Installation: updatecontrol.InstallationContract{SchemaVersion: updatecontrol.InstallationContractSchemaVersion, File: "kinosail-native-installation.json", SHA256: strings.Repeat("a", 64), Size: 1}}
	for _, platform := range []struct{ os, arch, format string }{{"linux", "amd64", "tar.gz"}, {"linux", "arm64", "tar.gz"}, {"macos", "amd64", "tar.gz"}, {"macos", "arm64", "tar.gz"}, {"windows", "amd64", "zip"}, {"windows", "arm64", "zip"}} {
		extension := "." + platform.format
		manifest.Artifacts = append(manifest.Artifacts, updatecontrol.Artifact{OS: platform.os, Arch: platform.arch, File: "kinosail-core-v1.2.3-" + platform.os + "-" + platform.arch + extension, Format: platform.format, SHA256: strings.Repeat("a", 64), Size: 1})
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := contract.writeArtifact(bytes.NewReader(data), &output, "darwin", "arm64"); err != nil || !strings.Contains(output.String(), `"file":"kinosail-core-v1.2.3-macos-arm64.tar.gz"`) {
		t.Fatalf("artifact = %q, %v", output.String(), err)
	}
	manifest.StateSchema, manifest.MinimumStateSchema = contract.StateSchema+1, contract.StateSchema+1
	incompatible, _ := json.Marshal(manifest)
	output.Reset()
	if err := contract.writeArtifact(bytes.NewReader(incompatible), &output, "darwin", "arm64"); err == nil || output.Len() != 0 {
		t.Fatalf("incompatible artifact = %q, %v", output.String(), err)
	}
	for name, input := range map[string][]byte{"malformed manifest": []byte(`{`), "unsupported platform": data} {
		t.Run(name, func(t *testing.T) {
			output.Reset()
			if err := contract.writeArtifact(bytes.NewReader(input), &output, "plan9", "amd64"); err == nil || output.Len() != 0 {
				t.Fatalf("rejected artifact = %q, %v", output.String(), err)
			}
		})
	}
}

func (contract UpdateContract) PendingTarget(t *testing.T) {
	dataDir := t.TempDir()
	db, err := contract.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	store, err := updatecontrol.New(db, contract.Policy)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Request("v4.5.6"); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	var output bytes.Buffer
	if err := contract.writePlan(&output, dataDir); err != nil {
		t.Fatal(err)
	}
	var plan updatecontrol.Plan
	if json.Unmarshal(output.Bytes(), &plan) != nil || plan.TargetVersion != "v4.5.6" || !strings.HasSuffix(plan.ManifestURL, "/"+contract.Product+"-v4.5.6/kinosail-release.json") || plan.ManifestSignatureURL != plan.ManifestURL+".sigstore.json" || plan.SignatureIdentity != contract.SignatureIdentity {
		t.Fatalf("plan = %#v", plan)
	}
}

func (contract UpdateContract) RejectedReport(t *testing.T) {
	dataDir := t.TempDir()
	for name, input := range map[string]string{
		"unknown field":   `{"adapter":"linux","state":"current","checkedAt":"2026-08-27T12:00:00Z","extra":true}`,
		"trailing object": `{"adapter":"linux","state":"current","checkedAt":"2026-08-27T12:00:00Z"}{}`,
		"oversized":       strings.Repeat("x", 4<<10+1),
	} {
		t.Run(name, func(t *testing.T) {
			if err := contract.recordReport(strings.NewReader(input), dataDir); err == nil {
				t.Fatal("invalid update report was accepted")
			}
		})
	}
	var output bytes.Buffer
	if err := contract.writePlan(&output, dataDir); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"automatic":false`) || strings.Contains(output.String(), "requestId") {
		t.Fatalf("rejected reports changed update plan: %s", output.String())
	}
}
