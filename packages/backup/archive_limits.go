package backup

import (
	"bytes"
	"errors"
	"io"
)

const (
	maxEntryDocumentSize = 256 << 20
	entryDocumentBudget  = 2
)

var (
	entryAllowances = entryDocumentBudget * archiveEntrySize(maxEntryDocumentSize)
	maxDecodedTotal = maxDecodedSize + entryAllowances
	maxEncodedSize  = maxArchiveSize + int(entryAllowances)
)

func entryAllowance(contents map[string][]byte, written []string, entry func(string) bool) int {
	allowance := int64(0)
	for _, name := range written {
		if entry(name) {
			allowance += archiveEntrySize(len(contents[name]))
		}
	}
	return int(allowance)
}

type archiveBuffer struct {
	bytes.Buffer
	maximum int
}

func (buffer *archiveBuffer) Write(data []byte) (int, error) {
	if len(data) > buffer.maximum-buffer.Len() {
		return 0, errors.New("backup exceeds maximum size")
	}
	return buffer.Buffer.Write(data)
}

func validateArchiveSize(contents map[string][]byte, written []string, manifest []byte, entry func(string) bool) error {
	size, entries := int64(1024)+archiveEntrySize(len(manifest)), int64(0)
	for _, name := range written {
		if entry(name) {
			entries += archiveEntrySize(len(contents[name]))
		} else {
			size += archiveEntrySize(len(contents[name]))
		}
	}
	if size > maxDecodedSize || entries > entryAllowances {
		return errors.New("backup exceeds maximum decoded size")
	}
	return nil
}

func archiveEntrySize(size int) int64 { return 512 + (int64(size)+511)/512*512 }

func publishArchive(writer io.Writer, archive *archiveBuffer) error {
	_, err := io.Copy(writer, &archive.Buffer)
	return err
}
