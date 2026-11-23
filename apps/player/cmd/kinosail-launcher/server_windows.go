//go:build windows

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const createNoWindow = 0x08000000

type serverFiles struct {
	install, root, data, backups, cache, logs string
}

func currentFiles() (serverFiles, error) {
	executable, err := os.Executable()
	if err != nil {
		return serverFiles{}, err
	}
	local := os.Getenv("LOCALAPPDATA")
	if local == "" || !filepath.IsAbs(local) {
		return serverFiles{}, errors.New("LOCALAPPDATA is not set")
	}
	root := filepath.Join(local, "Kinosail")
	return serverFiles{
		install: filepath.Dir(executable), root: root, data: filepath.Join(root, "data"), backups: filepath.Join(root, "backups"),
		cache: filepath.Join(root, "cache"), logs: filepath.Join(root, "logs"),
	}, nil
}

func (files serverFiles) executable() string {
	return filepath.Join(files.install, "server", "kinosail-server.exe")
}
func (files serverFiles) marker() string { return filepath.Join(files.root, "setup-opened") }

func (files serverFiles) note(text string) {
	if os.MkdirAll(files.logs, 0o700) != nil {
		return
	}
	output, err := os.OpenFile(filepath.Join(files.logs, "launcher.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(output, "%s %s\n", time.Now().UTC().Format(time.RFC3339), text)
	_ = output.Close()
}

func (files serverFiles) prepare() (*os.File, error) {
	for _, folder := range []string{files.data, files.backups, files.cache, files.logs} {
		if err := os.MkdirAll(folder, 0o700); err != nil {
			return nil, err
		}
	}
	log := filepath.Join(files.logs, "kinosail.log")
	if info, err := os.Stat(log); err == nil && info.Size() > 10<<20 {
		_ = os.Rename(log, log+".1")
	}
	return os.OpenFile(log, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
}

func (files serverFiles) environment() []string {
	values := []string{}
	for _, value := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(value), "KINOSAIL_") {
			values = append(values, value)
		}
	}
	drive := os.Getenv("SystemDrive")
	if drive == "" {
		drive = "C:"
	}
	runtime := filepath.Join(files.install, "server")
	return append(values, "KINOSAIL_DATA_DIR="+files.data, "KINOSAIL_BACKUP_DIR="+files.backups, "KINOSAIL_CACHE_DIR="+files.cache,
		"KINOSAIL_MEDIA_DIR="+drive+`\`, "KINOSAIL_SUPERVISED=true", "KINOSAIL_STOP_WITH_LAUNCHER=true",
		"KINOSAIL_FFMPEG="+filepath.Join(runtime, "ffmpeg.exe"), "KINOSAIL_FFPROBE="+filepath.Join(runtime, "ffprobe.exe"),
		"KINOSAIL_FPCALC="+filepath.Join(runtime, "fpcalc.exe"))
}

func (files serverFiles) command(args ...string) *exec.Cmd {
	command := exec.Command(files.executable(), args...) //nolint:gosec,noctx // The server path comes from the installed launcher location.
	command.Dir = files.data
	command.Env = files.environment()
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	return command
}

type supervisor struct {
	files    serverFiles
	status   func(string)
	mu       sync.Mutex
	running  *exec.Cmd
	input    io.WriteCloser
	quitting bool
	crashes  []time.Time
}

func (server *supervisor) start() {
	server.mu.Lock()
	defer server.mu.Unlock()
	if server.quitting || server.running != nil {
		return
	}
	log, err := server.files.prepare()
	if err != nil {
		server.files.note("server folders could not be prepared")
		server.status("Kinosail could not start")
		return
	}
	command := server.files.command()
	command.Stdout, command.Stderr = log, log
	input, err := command.StdinPipe()
	if err == nil {
		err = command.Start()
	}
	if err != nil {
		_ = log.Close()
		server.files.note("server process could not start")
		server.status("Kinosail could not start")
		return
	}
	server.running, server.input = command, input
	server.status("Starting Kinosail…")
	go server.wait(command, log)
	go server.awaitHealth(command)
}

func (server *supervisor) wait(command *exec.Cmd, log *os.File) {
	err := command.Wait()
	code := command.ProcessState.ExitCode()
	_, _ = fmt.Fprintf(log, "launcher: server stopped with exit code %d\n", code)
	_ = log.Close()
	server.mu.Lock()
	server.running, server.input = nil, nil
	quitting := server.quitting
	delay := 500 * time.Millisecond
	if err != nil || code != 0 {
		now := time.Now()
		recent := []time.Time{}
		for _, crash := range server.crashes {
			if now.Sub(crash) < 2*time.Minute {
				recent = append(recent, crash)
			}
		}
		recent = append(recent, now)
		server.crashes = recent
		delay = 3 * time.Second
		if len(server.crashes) >= 5 {
			server.mu.Unlock()
			server.status("Kinosail stopped. Choose Restart to try again.")
			return
		}
	}
	server.mu.Unlock()
	if quitting {
		return
	}
	server.status("Restarting Kinosail…")
	time.AfterFunc(delay, server.start)
}

func (server *supervisor) awaitHealth(command *exec.Cmd) {
	for range 120 {
		check := server.files.command("healthcheck")
		if check.Run() == nil {
			server.mu.Lock()
			current := server.running == command
			server.mu.Unlock()
			if current {
				server.status("Kinosail is running")
				server.openFirstTime()
			}
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func (server *supervisor) openFirstTime() {
	marker := server.files.marker()
	if _, err := os.Stat(marker); err == nil {
		return
	}
	if os.WriteFile(marker, []byte("1"), 0o600) == nil {
		openKinosail()
	}
}

func (server *supervisor) restart() {
	server.mu.Lock()
	server.crashes = nil
	input := server.input
	server.mu.Unlock()
	if input != nil {
		_ = input.Close()
		return
	}
	server.start()
}

func (server *supervisor) stop() {
	server.mu.Lock()
	server.quitting = true
	command, input := server.running, server.input
	server.mu.Unlock()
	if command == nil {
		return
	}
	_ = input.Close()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		server.mu.Lock()
		stopped := server.running == nil
		server.mu.Unlock()
		if stopped {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = command.Process.Kill()
}
