package server_test

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestSubtitleOperationDeadlineKeepsAdmissionUntilChildPipeAndWaitSettle(t *testing.T) {
	config, target, calls, release, child, settled := subtitleOperationDescendantConfig(t)
	deadlines := make(chan context.CancelFunc, 2)
	config.SubtitleOperationTime.WithTimeout = func(parent context.Context, limit time.Duration) (context.Context, context.CancelFunc) {
		if limit != 21*time.Minute {
			t.Errorf("audio operation deadline = %s, want existing20m plus completion allowance", limit)
		}
		ctx, cancel := context.WithCancel(parent)
		deadlines <- cancel
		t.Cleanup(cancel)
		return ctx, cancel
	}
	handler := server.New(config)
	base := "/api/v1/subtitle-library/" + firstSubtitleInventoryID(t, handler)
	first := prepareSubtitleOperation(t, handler, base, "audio")
	second := prepareSubtitleOperation(t, handler, base, "audio")
	assertSubtitleOperationAccepted(t, activateSubtitleOperation(t, handler, base+"/audio", `{"language":"en"}`, []string{first.ID}), first.ID)
	pid := waitSubtitleOperationChild(t, child)
	restoreRegistry := blockSubtitleOperationDurablePath(t, config.DataDir)
	select {
	case cancel := <-deadlines:
		cancel()
	case <-time.After(time.Second):
		t.Fatal("operation deadline context was not created")
	}
	assertSubtitleOperationHeldThroughCancellation(t, handler, first.ID, pid)
	busy := activateSubtitleOperation(t, handler, base+"/audio", `{"language":"en"}`, []string{second.ID})
	if busy.Code != http.StatusConflict {
		t.Fatalf("activation before actual child settlement = %d", busy.Code)
	}
	assertSubtitleOperationProcessCount(t, calls, 1)
	writeTestFile(t, release, "release controlled descendant")
	waitSubtitleOperationSettled(t, settled)
	unknown := waitSubtitleOperationState(t, handler, first.ID, "unknown")
	if unknown.Outcome == "success" {
		t.Fatal("failed completion persistence reported confirmed success")
	}
	restoreRegistry()
	assertSubtitleOperationUnknownCannotReplay(t, handler, base, first.ID)
	assertSubtitleOperationAccepted(t, activateSubtitleOperation(t, handler, base+"/audio", `{"language":"en"}`, []string{second.ID}), second.ID)
	_ = waitSubtitleOperation(t, handler, second.ID)
	assertSubtitleOperationProcessCount(t, calls, 2)
	assertSubtitleActionBytes(t, target, []byte(subtitleActionInitial))
	_ = subtitleActionHistory(t, handler, nil, nil)
}

func TestSubtitleRunningReceiptRestartCannotResumeOrReplayAfterRealChildSettlement(t *testing.T) {
	config, target, calls, release, child, settled := subtitleOperationDescendantConfig(t)
	lifecycle, stopFirst := context.WithCancel(t.Context())
	t.Cleanup(stopFirst)
	config.Lifecycle = lifecycle
	first := server.New(config)
	base := "/api/v1/subtitle-library/" + firstSubtitleInventoryID(t, first)
	prepared := prepareSubtitleOperation(t, first, base, "audio")
	assertSubtitleOperationAccepted(t, activateSubtitleOperationAfterStartup(t, first, base, prepared.ID, target, calls), prepared.ID)
	pid := waitSubtitleOperationChild(t, child)
	restoreRegistry := blockSubtitleOperationDurablePath(t, config.DataDir)
	stopFirst()
	assertSubtitleOperationHeldThroughCancellation(t, first, prepared.ID, pid)
	writeTestFile(t, release, "release lifecycle-canceled child")
	waitSubtitleOperationSettled(t, settled)
	_ = waitSubtitleOperationState(t, first, prepared.ID, "unknown")
	restoreRegistry()
	config.Lifecycle = t.Context()
	restarted := server.New(config)
	unknown := readSubtitleOperation(t, restarted, prepared.ID)
	if unknown.State != "unknown" || unknown.Outcome == "success" {
		t.Fatalf("interrupted running receipt after restart = %+v", unknown)
	}
	assertSubtitleOperationUnknownCannotReplay(t, restarted, base, prepared.ID)
	assertSubtitleOperationProcessCount(t, calls, 1)
	assertSubtitleActionBytes(t, target, []byte(subtitleActionInitial))
	_ = subtitleActionHistory(t, restarted, nil, nil)
}

func subtitleOperationDescendantConfig(t *testing.T) (server.Config, string, string, string, string, string) {
	t.Helper()
	config, target, calls, release := subtitleOperationAudioConfig(t, false)
	child, settled := filepath.Join(filepath.Dir(calls), "child-pid"), filepath.Join(filepath.Dir(calls), "child-settled")
	body := "printf '%s\\n' \"$$\" > " + subtitleOperationShellQuote(child) + "\nwhile [ ! -f " + subtitleOperationShellQuote(release) + " ]; do sleep 0.01; done\ndd if=/dev/zero bs=1920000 count=1 2>/dev/null\nprintf 'settled\\n' > " + subtitleOperationShellQuote(settled) + "\n"
	script := "#!/bin/sh\nif [ \"$1\" != '-v' ]; then exit 1; fi\nprintf 'analysis\\n' >> " + subtitleOperationShellQuote(calls) + "\n/bin/sh -c " + subtitleOperationShellQuote(body) + " &\nwait \"$!\"\n"
	writeExecutable(t, config.FFmpeg, script)
	t.Cleanup(func() {
		writeTestFile(t, release, "release owned descendant on fixture exit")
		if _, err := os.Stat(child); err == nil {
			waitSubtitleOperationSettled(t, settled)
		}
	})
	return config, target, calls, release, child, settled
}

func subtitleOperationShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func waitSubtitleOperationChild(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			pid, parseErr := strconv.Atoi(strings.TrimSpace(string(data)))
			if parseErr == nil && pid > 0 && syscall.Kill(pid, 0) == nil {
				return pid
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("controlled descendant process prerequisite did not become ready")
	return 0
}

func assertSubtitleOperationHeldThroughCancellation(t *testing.T, handler http.Handler, id string, child int) {
	t.Helper()
	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		if syscall.Kill(child, 0) != nil {
			t.Fatal("controlled child exited before its fixture release")
		}
		if receipt := readSubtitleOperation(t, handler, id); receipt.State != "running" {
			t.Fatalf("cancellation released admission before real child/pipe settlement: %+v", receipt)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitSubtitleOperationSettled(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(path); err == nil && string(data) == "settled\n" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("controlled child did not finish producing output and settle")
}

func waitSubtitleOperationState(t *testing.T, handler http.Handler, id, expected string) subtitleOperationReceipt {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if receipt := readSubtitleOperation(t, handler, id); receipt.State == expected {
			return receipt
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("operation did not settle to the expected %s state", expected)
	return subtitleOperationReceipt{}
}

func assertSubtitleOperationUnknownCannotReplay(t *testing.T, handler http.Handler, base, id string) {
	t.Helper()
	response := activateSubtitleOperation(t, handler, base+"/audio", `{"language":"en"}`, []string{id})
	if response.Code != http.StatusConflict {
		t.Fatalf("unknown receipt replay = %d, want 409 before work", response.Code)
	}
}
