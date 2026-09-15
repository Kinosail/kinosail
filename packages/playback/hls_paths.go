package playback

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type hlsPlaylistRoot interface {
	Close() error
	Lstat(string) (fs.FileInfo, error)
	Open(string) (hlsPlaylistFile, error)
}

type hlsPlaylistFile interface {
	io.Reader
	Close() error
	Stat() (fs.FileInfo, error)
}

type osHLSPlaylistRoot struct{ *os.Root }

func (root osHLSPlaylistRoot) Open(name string) (hlsPlaylistFile, error) { return root.Root.Open(name) }

func HLSFile(name string) bool {
	if name == "index.m3u8" {
		return true
	}
	parts := strings.Split(name, "/")
	return len(parts) == 2 && QualityDirectory(parts[0]) && VariantFile(parts[1])
}

func ReadHLSPlaylist(path string) ([]byte, error) { //nolint:cyclop,gosec // Callers construct cache playlist paths from validated identifiers.
	return readHLSPlaylist(path, func(path string) (hlsPlaylistRoot, error) {
		root, err := os.OpenRoot(path)
		return osHLSPlaylistRoot{root}, err
	})
}

func readHLSPlaylist(path string, openRoot func(string) (hlsPlaylistRoot, error)) ([]byte, error) { //nolint:cyclop // The injected root keeps every filesystem failure deterministic under test.
	directory := filepath.Dir(path)
	rootPath := filepath.Dir(directory)
	relative := filepath.Join(filepath.Base(directory), filepath.Base(path))
	if QualityDirectory(filepath.Base(directory)) {
		keyDirectory := filepath.Dir(directory)
		rootPath = filepath.Dir(keyDirectory)
		relative = filepath.Join(filepath.Base(keyDirectory), filepath.Base(directory), filepath.Base(path))
	}
	root, err := openRoot(rootPath)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	info, err := root.Lstat(relative)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maximumHLSPlaylistBytes {
		return nil, errors.New("HLS playlist is not a bounded regular file")
	}
	file, err := root.Open(relative)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err = file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maximumHLSPlaylistBytes {
		return nil, errors.New("HLS playlist is not a bounded regular file")
	}
	data, err := io.ReadAll(io.LimitReader(file, maximumHLSPlaylistBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maximumHLSPlaylistBytes {
		return nil, errors.New("HLS playlist exceeds the size limit")
	}
	return data, nil
}

func QualityDirectory(name string) bool {
	if name == "audio" {
		return true
	}
	height, err := strconv.Atoi(strings.TrimSuffix(name, "p"))
	return err == nil && strings.HasSuffix(name, "p") && height >= 144 && height <= 2160
}

func VariantFile(name string) bool {
	if name == "index.m3u8" || name == "init.mp4" {
		return true
	}
	if len(name) != len("segment-00000.m4s") || !strings.HasPrefix(name, "segment-") || filepath.Ext(name) != ".m4s" {
		return false
	}
	for _, character := range strings.TrimSuffix(strings.TrimPrefix(name, "segment-"), ".m4s") {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func AudioTrackIndex(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	track, err := strconv.Atoi(value)
	if err != nil || track < 0 || track > 31 {
		return 0, errors.New("audio track is invalid")
	}
	return track, nil
}

func HLSCacheKey(id string, track int) string {
	if track == 0 {
		return id
	}
	return id + "-audio-" + strconv.Itoa(track)
}
