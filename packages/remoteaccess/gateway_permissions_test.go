package remoteaccess

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateListenerRejectsForeignDirectoryBeforeMutation(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "http.sock")
	if err := os.WriteFile(path, []byte("owned sentinel"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, rejected := range []string{"", "relative.sock", path} {
		listener, err := privateListener(rejected)
		if err == nil || listener != nil {
			t.Fatal("foreign socket path accepted")
		}
	}
	body, err := os.ReadFile(path)
	if err != nil || string(body) != "owned sentinel" {
		t.Fatal("rejected socket path changed foreign data")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatal("rejected socket path changed foreign permissions")
	}
}
