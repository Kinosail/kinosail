package updatecontrol

import (
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"time"
)

func validateState(state State) error { //nolint:cyclop // Persisted-state validation remains below the repository complexity ceiling.
	if state.SchemaVersion != 1 {
		return errors.New("invalid persisted update schema")
	}
	if invalidStateRequest(state) {
		return errors.New("invalid persisted update request")
	}
	if state.RequestID == "" && (state.RequestedAt != "" || state.TargetVersion != "") {
		return errors.New("invalid persisted update request")
	}
	if invalidStateStatus(state) {
		return errors.New("invalid persisted update status")
	}
	if invalidStateAdapter(state) {
		return errors.New("invalid persisted update adapter")
	}
	if state.CheckedAt != "" && parseTime(state.CheckedAt) != nil {
		return errors.New("invalid persisted update check time")
	}
	if invalidStateDetails(state) {
		return errors.New("invalid persisted update details")
	}
	if state.Phase != "" && !validPhase(state.Phase) || state.ErrorCode != "" && !validErrorCode(state.ErrorCode) {
		return errors.New("invalid persisted update details")
	}
	return nil
}

func invalidStateRequest(state State) bool {
	return state.RequestID != "" && (len(state.RequestID) != 32 || !hexadecimal(state.RequestID) || parseTime(state.RequestedAt) != nil || !ValidReleaseVersion(state.TargetVersion))
}

func invalidStateStatus(state State) bool {
	return state.Status != "" && !slices.Contains([]string{"checking", "current", "available", "installing", "failed", "rolled-back"}, state.Status)
}

func invalidStateAdapter(state State) bool {
	return state.Adapter != "" && !slices.Contains([]string{"docker", "linux", "macos", "windows"}, state.Adapter)
}

func invalidStateDetails(state State) bool {
	return tooLong(state.CurrentVersion, 64) || tooLong(state.AvailableVersion, 64) || tooLong(state.Message, 240)
}

func validateReport(report Report) error {
	state := State{SchemaVersion: 1, Adapter: report.Adapter, Status: report.State, CurrentVersion: report.CurrentVersion, AvailableVersion: report.AvailableVersion, Message: report.Message, Phase: report.Phase, ErrorCode: report.ErrorCode}
	if invalidReportIdentity(report, state) {
		return errors.New("invalid update report")
	}
	if invalidReportResult(report) {
		return errors.New("invalid update report")
	}
	if invalidReportProgress(report) {
		return errors.New("invalid update report")
	}
	if invalidReportPhase(report) {
		return errors.New("invalid update report")
	}
	return nil
}

func invalidReportIdentity(report Report, state State) bool {
	return report.Adapter == "" || report.State == "" || report.CheckedAt.IsZero() || validateState(state) != nil || report.RequestID != "" && (len(report.RequestID) != 32 || !hexadecimal(report.RequestID))
}

func invalidReportResult(report Report) bool {
	return report.State == "available" && report.AvailableVersion == "" || report.State == "current" && report.CurrentVersion == "" || slices.Contains([]string{"failed", "rolled-back"}, report.State) && report.Message == ""
}

func invalidReportProgress(report Report) bool {
	return report.State == "installing" && report.Phase == "" || slices.Contains([]string{"failed", "rolled-back"}, report.State) && report.ErrorCode == ""
}

func invalidReportPhase(report Report) bool {
	return !slices.Contains([]string{"installing", "failed", "rolled-back"}, report.State) && (report.Phase != "" || report.ErrorCode != "") || report.State == "installing" && report.ErrorCode != ""
}

func parseTime(value string) error {
	_, err := time.Parse(time.RFC3339, value)
	return err
}

func hexadecimal(value string) bool {
	_, err := hex.DecodeString(value)
	return err == nil
}

func tooLong(value string, limit int) bool {
	return len(value) > limit || strings.ContainsAny(value, "\r\n")
}
