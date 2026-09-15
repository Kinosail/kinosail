package updatecontrol

import "slices"

const PlanSchemaVersion = 2

type Phase string

const (
	PhaseWaitingForIdle   Phase = "waiting-for-idle"
	PhaseBackingUp        Phase = "backing-up"
	PhaseVerifyingBackup  Phase = "verifying-backup"
	PhaseDownloading      Phase = "downloading"
	PhaseVerifyingRelease Phase = "verifying-release"
	PhaseStopping         Phase = "stopping"
	PhaseReplacing        Phase = "replacing"
	PhaseStarting         Phase = "starting"
	PhaseCheckingHealth   Phase = "checking-health"
	PhaseRestoringBackup  Phase = "restoring-backup"
)

type ErrorCode string

const (
	ErrorDownloadFailed     ErrorCode = "download-failed"
	ErrorSignatureInvalid   ErrorCode = "signature-invalid"
	ErrorDigestMismatch     ErrorCode = "digest-mismatch"
	ErrorBackupFailed       ErrorCode = "backup-failed"
	ErrorServiceStopFailed  ErrorCode = "service-stop-failed"
	ErrorInstallFailed      ErrorCode = "install-failed"
	ErrorServiceStartFailed ErrorCode = "service-start-failed"
	ErrorHealthCheckFailed  ErrorCode = "health-check-failed"
	ErrorRestoreFailed      ErrorCode = "restore-failed"
	ErrorRollbackFailed     ErrorCode = "rollback-failed"
	ErrorInsufficientSpace  ErrorCode = "insufficient-space"
	ErrorPermissionDenied   ErrorCode = "permission-denied"
)

type RecoveryPlan struct {
	BackupCommand     []string `json:"backupCommand"`
	VerifyCommand     []string `json:"verifyCommand"`
	RestoreCommand    []string `json:"restoreCommand"`
	Format            string   `json:"format"`
	IncludesSecrets   bool     `json:"includesSecrets"`
	RequiresKey       bool     `json:"requiresInstallationKey"`
	RestoreOnRollback bool     `json:"restoreOnRollback"`
}

func validPhase(value Phase) bool {
	return slices.Contains([]Phase{PhaseWaitingForIdle, PhaseBackingUp, PhaseVerifyingBackup, PhaseDownloading, PhaseVerifyingRelease, PhaseStopping, PhaseReplacing, PhaseStarting, PhaseCheckingHealth, PhaseRestoringBackup}, value)
}

func validErrorCode(value ErrorCode) bool {
	return slices.Contains([]ErrorCode{ErrorDownloadFailed, ErrorSignatureInvalid, ErrorDigestMismatch, ErrorBackupFailed, ErrorServiceStopFailed, ErrorInstallFailed, ErrorServiceStartFailed, ErrorHealthCheckFailed, ErrorRestoreFailed, ErrorRollbackFailed, ErrorInsufficientSpace, ErrorPermissionDenied}, value)
}
