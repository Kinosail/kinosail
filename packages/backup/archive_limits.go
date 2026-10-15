package backup

import (
	"bytes"
	"errors"
	"io"
)

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
