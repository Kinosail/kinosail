package updatecontrol

import "slices"

const PlanSchemaVersion = 2

type Phase string

const (
	phaseWaitingForIdle   Phase = "waiting-for-idle"
	phaseBackingUp        Phase = "backing-up"
	phaseVerifyingBackup  Phase = "verifying-backup"
	phaseDownloading      Phase = "downloading"
	phaseVerifyingRelease Phase = "verifying-release"
	phaseStopping         Phase = "stopping"
	phaseReplacing        Phase = "replacing"
	phaseStarting         Phase = "starting"
	phaseCheckingHealth   Phase = "checking-health"
	phaseRestoringBackup  Phase = "restoring-backup"
)

type ErrorCode string

const (
	errorDownloadFailed     ErrorCode = "download-failed"
	errorSignatureInvalid   ErrorCode = "signature-invalid"
	errorDigestMismatch     ErrorCode = "digest-mismatch"
	errorBackupFailed       ErrorCode = "backup-failed"
	errorServiceStopFailed  ErrorCode = "service-stop-failed"
	errorInstallFailed      ErrorCode = "install-failed"
	errorServiceStartFailed ErrorCode = "service-start-failed"
	errorHealthCheckFailed  ErrorCode = "health-check-failed"
	errorRestoreFailed      ErrorCode = "restore-failed"
	errorRollbackFailed     ErrorCode = "rollback-failed"
	errorInsufficientSpace  ErrorCode = "insufficient-space"
	errorPermissionDenied   ErrorCode = "permission-denied"
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
	return slices.Contains([]Phase{phaseWaitingForIdle, phaseBackingUp, phaseVerifyingBackup, phaseDownloading, phaseVerifyingRelease, phaseStopping, phaseReplacing, phaseStarting, phaseCheckingHealth, phaseRestoringBackup}, value)
}

func validErrorCode(value ErrorCode) bool {
	return slices.Contains([]ErrorCode{errorDownloadFailed, errorSignatureInvalid, errorDigestMismatch, errorBackupFailed, errorServiceStopFailed, errorInstallFailed, errorServiceStartFailed, errorHealthCheckFailed, errorRestoreFailed, errorRollbackFailed, errorInsufficientSpace, errorPermissionDenied}, value)
}
