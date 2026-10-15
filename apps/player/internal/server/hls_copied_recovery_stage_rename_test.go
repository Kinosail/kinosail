package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopiedRecoveryStageAcceptsConcurrentTemporaryFileReplacement(t *testing.T) {
	directory := t.TempDir()
	stage, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer stage.Close()
	output := &copiedHLSVariantOutput{stage: stage, observation: t.Context()}
	stop, done, ready := make(chan struct{}), make(chan struct{}), make(chan struct{})
	failures := make(chan error, 1)
	go func() {
		defer close(done)
		if err := copiedRecoveryReplaceTemporaryFile(directory, stop, ready); err != nil {
			failures <- err
		}
	}()
	defer func() { close(stop); <-done }()
	select {
	case <-ready:
	case err := <-failures:
		t.Fatal(err)
	}
	for range 20_000 {
		if !output.boundedStage() {
			t.Fatal("valid atomic temporary-file replacement rejected the refill stage")
		}
	}
	select {
	case err := <-failures:
		t.Fatal(err)
	default:
	}
}

func copiedRecoveryReplaceTemporaryFile(directory string, stop <-chan struct{}, ready chan<- struct{}) error {
	for first := true; ; first = false {
		select {
		case <-stop:
			return nil
		default:
		}
		temporary := filepath.Join(directory, "index.m3u8.tmp")
		if err := os.WriteFile(temporary, []byte("fixture"), 0o600); err != nil {
			return err
		}
		if err := os.Rename(temporary, filepath.Join(directory, "index.m3u8")); err != nil {
			return err
		}
		if first {
			close(ready)
		}
	}
}
