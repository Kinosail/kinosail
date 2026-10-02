package backup

import (
	"bytes"
	"errors"
	"io"
)

// archiveBuffer bounds encoded output while keeping invalid candidates private.
// A caller sees bytes only after all size checks and compressor closes succeed.
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

func validateArchiveSize(contents map[string][]byte, written []string, manifest []byte) error {
	// USTAR records have one 512-byte header, padded contents, and two end blocks.
	size := int64(1024) + archiveEntrySize(len(manifest))
	for _, name := range written {
		size += archiveEntrySize(len(contents[name]))
	}
	if size > maxDecodedSize {
		return errors.New("backup exceeds maximum decoded size")
	}
	return nil
}

func archiveEntrySize(size int) int64 { return 512 + (int64(size)+511)/512*512 }

func publishArchive(writer io.Writer, archive *archiveBuffer) error {
	_, err := io.Copy(writer, &archive.Buffer)
	return err
}
