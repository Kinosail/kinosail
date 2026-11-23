//go:build windows

package main

import (
	"errors"
	"os"
	"runtime"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	kinosailAddress = "https://localhost:38127/"
	runKey          = `Software\Microsoft\Windows\CurrentVersion\Run`
)

func init() { runtime.LockOSThread() }

func main() {
	os.Exit(run())
}

func run() int {
	name, _ := windows.UTF16PtrFromString(`Local\Kinosail.Launcher`)
	mutex, err := windows.CreateMutex(nil, false, name)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		openKinosail()
		return 0
	}
	if err != nil {
		return 1
	}
	defer func() { _ = windows.CloseHandle(mutex) }()
	files, err := currentFiles()
	if err != nil {
		return 1
	}
	tray := &trayIcon{}
	server := &supervisor{files: files, status: tray.setStatus}
	tray.server = server
	if err := tray.create(files.install); err != nil {
		return 1
	}
	server.start()
	tray.loop()
	return 0
}

func openKinosail() {
	verb, _ := windows.UTF16PtrFromString("open")
	target, _ := windows.UTF16PtrFromString(kinosailAddress)
	_ = windows.ShellExecute(0, verb, target, nil, nil, windows.SW_SHOWNORMAL)
}

func startsAtLogin() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer func() { _ = key.Close() }()
	_, _, err = key.GetStringValue("Kinosail")
	return err == nil
}

func setStartsAtLogin(enabled bool) {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return
	}
	defer func() { _ = key.Close() }()
	if !enabled {
		_ = key.DeleteValue("Kinosail")
		return
	}
	if executable, err := os.Executable(); err == nil {
		_ = key.SetStringValue("Kinosail", `"`+executable+`"`)
	}
}
