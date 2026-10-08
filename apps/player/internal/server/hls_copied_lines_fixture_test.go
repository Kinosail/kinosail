package server

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type copiedHLSBuildDiagnostic struct {
	data []byte
	size int
}

func (output *copiedHLSBuildDiagnostic) Write(data []byte) (int, error) {
	output.size += len(data)
	output.data = append(output.data, data[:min(len(data), 4096-len(output.data))]...)
	return len(data), nil
}

// Build the isolated real probe before starting the unchanged operation budget.
func copiedHLSProbeExecutable(t *testing.T) string {
	t.Helper()
	source := copiedHLSProbeFileHash(t, "testdata/copied-probe-fixture/main.go", 128<<10)
	if source != "7db25a5574a719da5f67e80ad7fb8b433017f70abe0319e72d3e051d318af971" {
		t.Fatal("standalone probe fixture source changed")
	}
	name := "copied-probe-fixture"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	executable := filepath.Join(t.TempDir(), name)
	ctx, cancel := context.WithTimeout(t.Context(), 29*time.Second)
	defer cancel()
	var diagnostic copiedHLSBuildDiagnostic
	command := exec.CommandContext(ctx, "go", "build", "-trimpath", "-o", executable, "./testdata/copied-probe-fixture")
	command.WaitDelay = time.Second // Setup drain bound; never a settlement oracle.
	command.Stdout, command.Stderr = &diagnostic, &diagnostic
	if command.Run() != nil {
		classes := []string{}
		for _, class := range []string{"undefined:", "cannot use", "syntax error", "build constraints", "permission denied"} {
			if strings.Contains(string(diagnostic.data), class) {
				classes = append(classes, class)
			}
		}
		t.Logf("nonkey owned probe fixture build bytes=%d retained=%d diagnostic-sha256=%x classes=%v", diagnostic.size, len(diagnostic.data), sha256.Sum256(diagnostic.data), classes)
		t.Fatal("standalone configured probe fixture did not compile")
	}
	binary := copiedHLSProbeFileHash(t, executable, 8<<20)
	t.Logf("nonkey owned probe fixture source-sha256=%s executable-sha256=%s", source, binary)
	return executable
}

func copiedHLSProbeFileHash(t *testing.T, path string, maximum int64) string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal("configured probe fixture is unavailable")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maximum {
		t.Fatal("configured probe fixture exceeds its regular-file bound")
	}
	hash := sha256.New()
	if count, err := io.Copy(hash, io.LimitReader(file, maximum+1)); err != nil || count != info.Size() {
		t.Fatal("configured probe fixture changed while being bound")
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func copiedHLSProbeArguments(mode string) []string {
	return []string{mode}
}
