package playback

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type stubHLSRoot struct {
	info     fs.FileInfo
	lstatErr error
	file     hlsPlaylistFile
	openErr  error
}

func (stubHLSRoot) Close() error { return nil }
func (root stubHLSRoot) Lstat(string) (fs.FileInfo, error) {
	return root.info, root.lstatErr
}
func (root stubHLSRoot) Open(string) (hlsPlaylistFile, error) { return root.file, root.openErr }

type stubHLSFile struct {
	reader  io.Reader
	info    fs.FileInfo
	statErr error
}

func (stubHLSFile) Close() error { return nil }
func (file stubHLSFile) Stat() (fs.FileInfo, error) {
	return file.info, file.statErr
}
func (file stubHLSFile) Read(data []byte) (int, error) { return file.reader.Read(data) }

func TestReadHLSPlaylistCoversEveryFilesystemFailure(t *testing.T) {
	root := t.TempDir()
	regular := filepath.Join(root, "regular")
	writeTestFile(t, regular, "playlist")
	regularInfo, err := os.Stat(regular)
	if err != nil {
		t.Fatal(err)
	}
	directoryInfo, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	large := filepath.Join(root, "large")
	writeTestFile(t, large, "x")
	if err := os.Truncate(large, maximumHLSPlaylistBytes+1); err != nil {
		t.Fatal(err)
	}
	largeInfo, err := os.Stat(large)
	if err != nil {
		t.Fatal(err)
	}
	readErr := errors.New("read")
	tests := []struct {
		name string
		open func(string) (hlsPlaylistRoot, error)
	}{
		{"open root", func(string) (hlsPlaylistRoot, error) { return nil, errors.New("open root") }},
		{"lstat", stubRoot(stubHLSRoot{lstatErr: errors.New("lstat")})},
		{"lstat directory", stubRoot(stubHLSRoot{info: directoryInfo})},
		{"lstat oversized", stubRoot(stubHLSRoot{info: largeInfo})},
		{"open file", stubRoot(stubHLSRoot{info: regularInfo, openErr: errors.New("open file")})},
		{"stat", stubRoot(stubHLSRoot{info: regularInfo, file: stubHLSFile{reader: strings.NewReader("x"), statErr: errors.New("stat")}})},
		{"stat directory", stubRoot(stubHLSRoot{info: regularInfo, file: stubHLSFile{reader: strings.NewReader("x"), info: directoryInfo}})},
		{"stat oversized", stubRoot(stubHLSRoot{info: regularInfo, file: stubHLSFile{reader: strings.NewReader("x"), info: largeInfo}})},
		{"read", stubRoot(stubHLSRoot{info: regularInfo, file: stubHLSFile{reader: errorReader{readErr}, info: regularInfo}})},
		{"read oversized", stubRoot(stubHLSRoot{info: regularInfo, file: stubHLSFile{reader: strings.NewReader(strings.Repeat("x", maximumHLSPlaylistBytes+1)), info: regularInfo}})},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := readHLSPlaylist("cache/index.m3u8", test.open); err == nil {
				t.Fatal("filesystem failure was ignored")
			}
		})
	}
	valid := stubRoot(stubHLSRoot{info: regularInfo, file: stubHLSFile{reader: strings.NewReader("playlist"), info: regularInfo}})
	if data, err := readHLSPlaylist("cache/index.m3u8", valid); err != nil || string(data) != "playlist" {
		t.Fatalf("playlist = %q, %v", data, err)
	}
}

func stubRoot(root stubHLSRoot) func(string) (hlsPlaylistRoot, error) {
	return func(string) (hlsPlaylistRoot, error) { return root, nil }
}

type errorReader struct{ err error }

func (reader errorReader) Read([]byte) (int, error) { return 0, reader.err }

func TestVariantReadyRejectsFreshSymlink(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	writeTestFile(t, source, "source")
	actual := filepath.Join(root, "actual.m3u8")
	writeTestFile(t, actual, "#EXTM3U\n")
	directory := filepath.Join(root, "360p")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	playlist := filepath.Join(directory, "index.m3u8")
	if err := os.Symlink(actual, playlist); err != nil {
		t.Fatal(err)
	}
	if VariantReady(source, directory) || MasterFresh(playlist, source, "ffmpeg") {
		t.Fatal("fresh symlink variant was accepted")
	}
}
