// Package updatecontrol owns the platform-neutral update contract.
package updatecontrol

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"slices"
	"sync"
	"time"
)

const document = "updates.json"

type Report struct {
	Adapter          string    `json:"adapter"`
	State            string    `json:"state"`
	CurrentVersion   string    `json:"currentVersion,omitempty"`
	AvailableVersion string    `json:"availableVersion,omitempty"`
	Message          string    `json:"message,omitempty"`
	Phase            Phase     `json:"phase,omitempty"`
	ErrorCode        ErrorCode `json:"errorCode,omitempty"`
	RequestID        string    `json:"requestId,omitempty"`
	CheckedAt        time.Time `json:"checkedAt"`
}

type State struct {
	SchemaVersion    int       `json:"schemaVersion"`
	RequestID        string    `json:"requestId,omitempty"`
	RequestedAt      string    `json:"requestedAt,omitempty"`
	TargetVersion    string    `json:"targetVersion,omitempty"`
	Adapter          string    `json:"adapter,omitempty"`
	Status           string    `json:"status,omitempty"`
	CurrentVersion   string    `json:"currentVersion,omitempty"`
	AvailableVersion string    `json:"availableVersion,omitempty"`
	Message          string    `json:"message,omitempty"`
	Phase            Phase     `json:"phase,omitempty"`
	ErrorCode        ErrorCode `json:"errorCode,omitempty"`
	CheckedAt        string    `json:"checkedAt,omitempty"`
}

type View struct {
	CurrentVersion   string    `json:"currentVersion"`
	AvailableVersion string    `json:"availableVersion,omitempty"`
	Status           string    `json:"status"`
	Adapter          string    `json:"adapter,omitempty"`
	Message          string    `json:"message,omitempty"`
	Phase            Phase     `json:"phase,omitempty"`
	ErrorCode        ErrorCode `json:"errorCode,omitempty"`
	RequestID        string    `json:"requestId,omitempty"`
	RequestedAt      string    `json:"requestedAt,omitempty"`
	TargetVersion    string    `json:"targetVersion,omitempty"`
	CheckedAt        string    `json:"checkedAt,omitempty"`
}

type Plan struct {
	SchemaVersion        int          `json:"schemaVersion"`
	StateSchema          int          `json:"stateSchema"`
	ConfigurationSchema  int          `json:"configurationSchema"`
	CurrentVersion       string       `json:"currentVersion"`
	Automatic            bool         `json:"automatic"`
	RequestID            string       `json:"requestId,omitempty"`
	RequestedAt          string       `json:"requestedAt,omitempty"`
	TargetVersion        string       `json:"targetVersion,omitempty"`
	ManifestURL          string       `json:"manifestUrl,omitempty"`
	ManifestSignatureURL string       `json:"manifestSignatureUrl,omitempty"`
	SignatureIdentity    string       `json:"signatureIdentity"`
	RequireSignature     bool         `json:"requireSignature"`
	BackupBeforeInstall  bool         `json:"backupBeforeInstall"`
	DeferWhilePlaying    bool         `json:"deferWhilePlaying"`
	RollbackOnUnhealthy  bool         `json:"rollbackOnUnhealthy"`
	Recovery             RecoveryPlan `json:"recovery"`
}

type Store struct {
	mu     sync.Mutex
	db     DocumentStore
	policy Policy
	state  State
	now    func() time.Time
	random io.Reader
}

func New(db DocumentStore, policy Policy) (*Store, error) {
	if err := validatePolicy(policy); err != nil {
		return nil, err
	}
	store := &Store{db: db, policy: policy, now: time.Now, random: rand.Reader}
	return store, store.reload()
}

func (store *Store) View(currentVersion string) (View, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := store.reload(); err != nil {
		return View{}, err
	}
	state, status := store.state, store.state.Status
	if state.RequestID != "" && status == "" {
		status = "requested"
	} else if status == "" {
		status = "unavailable"
	}
	return View{CurrentVersion: currentVersion, AvailableVersion: state.AvailableVersion, Status: status, Adapter: state.Adapter, Message: state.Message, Phase: state.Phase, ErrorCode: state.ErrorCode, RequestID: state.RequestID, RequestedAt: state.RequestedAt, TargetVersion: state.TargetVersion, CheckedAt: state.CheckedAt}, nil
}

func (store *Store) Plan(currentVersion string, automatic bool) (Plan, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := store.reload(); err != nil {
		return Plan{}, err
	}
	manifest, signature := releaseManifestURLs(store.state.TargetVersion, store.policy)
	return Plan{SchemaVersion: PlanSchemaVersion, StateSchema: store.policy.stateSchema, ConfigurationSchema: store.policy.configurationSchema, CurrentVersion: currentVersion, Automatic: automatic, RequestID: store.state.RequestID, RequestedAt: store.state.RequestedAt, TargetVersion: store.state.TargetVersion, ManifestURL: manifest, ManifestSignatureURL: signature, SignatureIdentity: store.policy.signatureIdentity, RequireSignature: true, BackupBeforeInstall: true, DeferWhilePlaying: true, RollbackOnUnhealthy: true, Recovery: RecoveryPlan{BackupCommand: []string{"backup"}, VerifyCommand: []string{"backup", "verify"}, RestoreCommand: []string{"restore"}, Format: "kinosail-backup-v1-encrypted", IncludesSecrets: true, RequiresKey: true, RestoreOnRollback: true}}, nil
}

func (store *Store) Request(targetVersion string) (string, error) {
	if !ValidReleaseVersion(targetVersion) {
		return "", errors.New("invalid update target")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := store.reload(); err != nil {
		return "", err
	}
	if store.state.RequestID != "" {
		if store.state.TargetVersion != targetVersion {
			return "", errors.New("another update target is pending")
		}
		return store.state.RequestID, nil
	}
	value := make([]byte, 16)
	if _, err := io.ReadFull(store.random, value); err != nil {
		return "", err
	}
	state := store.state
	state.RequestID = hex.EncodeToString(value)
	state.RequestedAt = store.now().UTC().Format(time.RFC3339)
	state.TargetVersion = targetVersion
	state.Status, state.AvailableVersion, state.Message, state.CheckedAt = "", "", "", ""
	if err := store.save(state); err != nil {
		return "", err
	}
	return state.RequestID, nil
}

func (store *Store) Report(report Report) error { //nolint:cyclop,gocognit // Report validation and persistence remain one atomic operation.
	if err := validateReport(report); err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := store.reload(); err != nil {
		return err
	}
	if report.RequestID != "" && report.RequestID != store.state.RequestID {
		return errors.New("update report does not match the pending request")
	}
	if store.state.RequestID != "" && report.RequestID == "" {
		return errors.New("update report omits the pending request")
	}
	if store.state.RequestID != "" && (!validTransition(store.state.Status, report.State) || report.AvailableVersion != "" && report.AvailableVersion != store.state.TargetVersion || report.State == "current" && report.CurrentVersion != store.state.TargetVersion) {
		return errors.New("invalid update transition")
	}
	state := store.state
	state.Adapter, state.Status, state.CurrentVersion = report.Adapter, report.State, report.CurrentVersion
	state.AvailableVersion, state.Message = report.AvailableVersion, report.Message
	state.Phase, state.ErrorCode = report.Phase, report.ErrorCode
	state.CheckedAt = report.CheckedAt.UTC().Format(time.RFC3339)
	if report.RequestID != "" && slices.Contains([]string{"current", "rolled-back"}, report.State) {
		state.RequestID, state.RequestedAt, state.TargetVersion = "", "", ""
	}
	return store.save(state)
}

func (store *Store) reload() error {
	if store.db == nil {
		migrateState(&store.state)
		return validateState(store.state)
	}
	state := State{}
	found, err := store.db.LoadJSON(document, &state)
	if err != nil {
		return err
	}
	if !found {
		state = State{}
	}
	migrateState(&state)
	if err := validateState(state); err != nil {
		return err
	}
	store.state = state
	return nil
}

func (store *Store) save(state State) error {
	state.SchemaVersion = 1
	if err := validateState(state); err != nil {
		return err
	}
	if store.db != nil {
		if err := store.db.SaveJSON(document, state); err != nil {
			return err
		}
	}
	store.state = state
	return nil
}

func migrateState(state *State) {
	if state.SchemaVersion != 0 {
		return
	}
	state.SchemaVersion = 1
	if state.RequestID != "" && state.TargetVersion == "" {
		state.RequestID, state.RequestedAt = "", ""
		state.Status = "failed"
		state.Message = "Run a new update check before installation."
	}
}
