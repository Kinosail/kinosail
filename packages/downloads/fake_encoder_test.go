package downloads

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

const writeFakeMedia = "for output; do :; done\nprintf media > \"$output\"\n"

var fakeEncoderScripts = map[string]string{
	"media": "#!/bin/sh\n" + writeFakeMedia,
	"media-on-retry": "#!/bin/sh\ncount=\"$(dirname \"$0\")/count\"\nruns=0\n[ ! -f \"$count\" ] || runs=$(cat \"$count\")\n" +
		"runs=$((runs+1))\necho $runs > \"$count\"\n[ $runs -gt 1 ] || exit 1\n" + writeFakeMedia,
}

var fakeEncoders string

func TestMain(m *testing.M) {
	os.Exit(runWithFakeEncoders(m))
}

func runWithFakeEncoders(m *testing.M) int {
	directory, err := os.MkdirTemp("", "downloads-encoders-")
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
