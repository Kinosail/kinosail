package configurationcore

import (
	"errors"
	"os"
	"testing"
)

type stagedFileStub struct {
	name                                  string
	chmodErr, writeErr, syncErr, closeErr error
	closed                                bool
}

func (file *stagedFileStub) Name() string            { return file.name }
func (file *stagedFileStub) Chmod(os.FileMode) error { return file.chmodErr }
func (file *stagedFileStub) Write(data []byte) (int, error) {
	return len(data), file.writeErr
}
func (file *stagedFileStub) Sync() error  { return file.syncErr }
func (file *stagedFileStub) Close() error { file.closed = true; return file.closeErr }

func TestWriteStagedFilePropagatesEveryDurabilityFailure(t *testing.T) {
	want := errors.New("blocked")
	for _, file := range []*stagedFileStub{{chmodErr: want}, {writeErr: want}, {syncErr: want}, {closeErr: want}, {}} {
		err := WriteStagedFile(file, []byte("value"), 0o600)
		wantErr := file.chmodErr != nil || file.writeErr != nil || file.syncErr != nil || file.closeErr != nil
		if (err != nil) != wantErr || !file.closed {
			t.Fatalf("staged write error=%v closed=%t file=%#v", err, file.closed, file)
		}
	}
}
