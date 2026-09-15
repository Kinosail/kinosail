package server

import (
	"os"
	"os/exec"
	"time"
)

type subtitleReadinessCheck struct {
	Name     string `json:"name"`
	Ready    bool   `json:"ready"`
	Blocking bool   `json:"blocking"`
	Detail   string `json:"detail"`
}

type subtitleReadiness struct {
	State      string                   `json:"state"`
	Correction string                   `json:"correction,omitempty"`
	Checks     []subtitleReadinessCheck `json:"checks"`
}

func (manager *subtitleManager) readiness() subtitleReadiness { //nolint:cyclop // One projection must evaluate all public readiness checks together.
	readable, writable := manager.libraryAccess()
	providers := manager.provider.healthViews()
	configured := 0
	for _, provider := range providers {
		if provider.Configured {
			configured++
		}
	}
	embeddedAvailable := false
	if manager.embeddedReady() {
		_, probeErr := exec.LookPath(manager.probe.executable)
		_, ffmpegErr := exec.LookPath(manager.probe.ffmpeg)
		embeddedAvailable = probeErr == nil && ffmpegErr == nil
	}
	providerReady := configured > 0 || embeddedAvailable
	sourceDetail := "Configure one provider or install FFmpeg and FFprobe for embedded text"
	if configured > 0 {
		sourceDetail = "Provider credentials are configured; connections are checked automatically"
	} else if embeddedAvailable {
		sourceDetail = "Local embedded text extraction is available"
	}
	watching, _ := manager.index.Monitoring()
	background := watching || manager.settings.scanFrequency() != "off"
	backupReady := manager.backups != nil && manager.backups.Status().Enabled
	languageReady := validLanguage(manager.settings.subtitleLanguage())
	lastWrite := manager.provider.ledger.lastSuccessfulWrite()
	lastWriteDetail := "No subtitle written yet"
	if lastWrite.Unix() > 0 {
		lastWriteDetail = lastWrite.UTC().Format(time.RFC3339)
	}
	checks := []subtitleReadinessCheck{
		{"Media library readable", readable, true, map[bool]string{true: "All configured folders are readable", false: "Fix the media mount or folder access"}[readable]},
		{"Sidecar folder writable", writable, true, map[bool]string{true: "Sidecars can be written beside media", false: "Grant the Server write access to the media folders"}[writable]},
		{"Subtitle sources configured", providerReady, true, sourceDetail},
		{"Preferred language valid", languageReady, true, manager.settings.subtitleLanguage()},
		{"Background scan active", background, true, map[bool]string{true: "Filesystem monitoring or a safety scan is active", false: "Turn on a safety scan"}[background]},
		{"Encrypted backup configured", backupReady, false, map[bool]string{true: "Automatic encrypted recovery is configured", false: "Optional: configure an encrypted backup; subtitle replacements keep a local recovery copy"}[backupReady]},
		{"Last successful subtitle write", lastWrite.Unix() > 0, false, lastWriteDetail},
	}
	result := subtitleReadiness{State: "Ready", Checks: checks}
	for _, check := range checks {
		if check.Blocking && !check.Ready {
			result.State, result.Correction = "Needs one correction", check.Detail
			break
		}
	}
	return result
}

func (manager *subtitleManager) libraryAccess() (bool, bool) {
	roots := manager.settings.roots()
	if len(roots) == 0 {
		return false, false
	}
	readable, writable := true, true
	for _, root := range roots {
		directory, err := os.Open(root.Path) //nolint:gosec // The path was resolved inside the configured media root.
		if err != nil {
			readable = false
		} else {
			_ = directory.Close()
		}
		if !subtitleDirectoryWritable(root.Path) {
			writable = false
		}
	}
	return readable, writable
}

func subtitleDirectoryWritable(path string) bool {
	probe, err := os.CreateTemp(path, ".kinosail-write-check-*") //nolint:gosec // The directory was resolved inside the configured media root.
	if err != nil {
		return false
	}
	name := probe.Name()
	closeErr := probe.Close()
	removeErr := os.Remove(name) //nolint:gosec // The name comes from os.CreateTemp in the validated directory.
	return closeErr == nil && removeErr == nil
}
