package updatecontrol

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

const (
	CommandArtifact = "update-artifact"
	CommandPlan     = "update-plan"
	CommandReport   = "update-report"
	reportLimit     = 4 << 10
)

// CommandDatabase is the storage required by update commands.
type CommandDatabase interface {
	DocumentStore
	Close() error
}

// CommandConfig supplies one app's release and storage adapters.
type CommandConfig struct {
	Policy       Policy
	OpenDatabase func(string) (CommandDatabase, error)
	Version      func() string
}

// Command runs one update adapter command.
type Command func(string, io.Reader, io.Writer, string, string, string) error

// BindCommand binds Player's update command contract to one app.
func BindCommand(config CommandConfig) Command {
	if validatePolicy(config.Policy) != nil || config.OpenDatabase == nil || config.Version == nil {
		return func(string, io.Reader, io.Writer, string, string, string) error {
			return errors.New("update command is not configured")
		}
	}
	commands := commandSet{config: config}
	return commands.run
}

type commandSet struct {
	config CommandConfig
}

func (commands commandSet) run(name string, input io.Reader, output io.Writer, dataDir, goos, arch string) error {
	switch name {
	case CommandArtifact:
		return commands.writeArtifact(input, output, goos, arch)
	case CommandPlan:
		return commands.writePlan(output, dataDir)
	case CommandReport:
		return commands.recordReport(input, dataDir)
	default:
		return errors.New("unknown update command")
	}
}

func (commands commandSet) writeArtifact(input io.Reader, output io.Writer, goos, arch string) error {
	if input == nil {
		return errors.New("invalid release manifest")
	}
	if output == nil {
		return errors.New("update output is unavailable")
	}
	manifest, err := ParseManifest(input)
	if err != nil {
		return err
	}
	if err = manifest.Compatible(commands.config.Policy.stateSchema, commands.config.Policy.configurationSchema); err != nil {
		return err
	}
	artifact, err := manifest.Select(goos, arch)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(artifact)
}

func (commands commandSet) writePlan(output io.Writer, dataDir string) error {
	if output == nil {
		return errors.New("update output is unavailable")
	}
	store, database, closeStore, err := commands.open(dataDir)
	if err != nil {
		return err
	}
	defer closeStore()
	var settings struct {
		Automatic bool `json:"updateChecks"`
	}
	if _, err = database.LoadJSON("settings.json", &settings); err != nil {
		return err
	}
	plan, err := store.Plan(commands.config.Version(), settings.Automatic)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(plan)
}

func (commands commandSet) recordReport(input io.Reader, dataDir string) error {
	if input == nil {
		return errors.New("invalid update report")
	}
	data, err := io.ReadAll(io.LimitReader(input, reportLimit+1))
	if err != nil || len(data) > reportLimit {
		return errors.New("invalid update report")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var report Report
	if decoder.Decode(&report) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("invalid update report")
	}
	store, _, closeStore, err := commands.open(dataDir)
	if err != nil {
		return err
	}
	defer closeStore()
	return store.Report(report)
}

func (commands commandSet) open(dataDir string) (*Store, CommandDatabase, func(), error) {
	if dataDir == "" {
		return nil, nil, func() {}, errors.New("update storage is unavailable")
	}
	database, err := commands.config.OpenDatabase(dataDir)
	if err != nil {
		return nil, nil, func() {}, err
	}
	if database == nil {
		return nil, nil, func() {}, errors.New("update storage is unavailable")
	}
	store, err := New(database, commands.config.Policy)
	if err != nil {
		_ = database.Close()
		return nil, nil, func() {}, err
	}
	return store, database, func() { _ = database.Close() }, nil
}
