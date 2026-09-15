package server

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

const openSubtitlesHashBlock = 64 << 10

func openSubtitlesHash(path string) (string, error) { //nolint:cyclop // The bounded file hash validates open, size, both reads, and concurrent mutation.
	file, err := os.Open(path) //nolint:gosec // The path is scanned Library Content.
	if err != nil {
		return "", errors.New("media hash is unavailable")
	}
	defer file.Close()
	before, err := file.Stat()
	if err != nil || !before.Mode().IsRegular() || before.Size() < openSubtitlesHashBlock*2 {
		return "", errors.New("media hash is unavailable")
	}
	first := make([]byte, openSubtitlesHashBlock)
	last := make([]byte, openSubtitlesHashBlock)
	if _, err = io.ReadFull(file, first); err != nil {
		return "", errors.New("media hash is unavailable")
	}
	if _, err = file.Seek(before.Size()-openSubtitlesHashBlock, io.SeekStart); err != nil {
		return "", errors.New("media hash is unavailable")
	}
	if _, err = io.ReadFull(file, last); err != nil {
		return "", errors.New("media hash is unavailable")
	}
	hash := uint64(before.Size()) //nolint:gosec // The defined OpenSubtitles algorithm uses unsigned wraparound.
	for offset := 0; offset < openSubtitlesHashBlock; offset += 8 {
		hash += binary.LittleEndian.Uint64(first[offset:])
		hash += binary.LittleEndian.Uint64(last[offset:])
	}
	after, err := file.Stat()
	if err != nil || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
		return "", errors.New("media changed during hashing")
	}
	return fmt.Sprintf("%016x", hash), nil
}
