package playback

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

const recordFakeEncoderRun = "#!/bin/sh\nprintf x >> \"$(dirname \"$0\")/runs.log\"\n"

const writeFakeFrame = "for argument; do last=\"$argument\"; done\nprintf jpeg > \"$last\"\n"

var fakeEncoderScripts = map[string]string{
	"frame":      "#!/bin/sh\nfor target do :; done\nprintf frame > \"$target\"\n",
	"slow-frame": recordFakeEncoderRun + "sleep 0.2\n" + writeFakeFrame,
}

var fakeEncoders string

func TestMain(m *testing.M) {
	os.Exit(runWithFakeEncoders(m))
}

func runWithFakeEncoders(m *testing.M) int {
	directory, err := os.MkdirTemp("", "playback-encoders-")
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(directory)
	for name, script := range fakeEncoderScripts {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(script), 0o700); err != nil { //nolint:gosec // Fake encoders must be executable by the test process.
			_, _ = fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	fakeEncoders = directory
	return m.Run()
}

func linkFakeEncoder(t *testing.T, directory, name string) string {
	t.Helper()
	if _, known := fakeEncoderScripts[name]; !known {
		t.Fatalf("unknown fake encoder %q", name)
	}
	link := filepath.Join(directory, name)
	if err := os.Symlink(filepath.Join(fakeEncoders, name), link); err != nil {
		t.Fatal(err)
	}
	return link
}
