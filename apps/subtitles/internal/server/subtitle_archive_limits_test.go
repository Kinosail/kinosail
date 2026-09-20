package server

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

func archiveFixture(t *testing.T, count, size int, validLast bool) *zip.Reader {
	t.Helper()
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for i := range count {
		file, err := archive.Create(fmt.Sprintf("%d.srt", i))
		if err != nil {
			t.Fatal(err)
		}
		data := strings.Repeat("x", size)
		if validLast && i == count-1 {
			data = "1\n00:00:01,000 --> 00:00:02,000\nDialogue\n"
		}
		if _, err := io.WriteString(file, data); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
	if err != nil {
		t.Fatal(err)
	}
	return reader
}

func TestSubtitleArchiveBoundsAggregateInvalidCandidates(t *testing.T) {
	for _, input := range []struct{ count, size int }{{129, 1}, {6, 4 << 20}} {
		data, err := readArchivedSubtitle(t.Context(), archiveFixture(t, input.count, input.size, true))
		if err == nil || len(data) != 0 {
			t.Fatal("oversized archive returned a payload")
		}
	}
	data, err := readArchivedSubtitle(t.Context(), archiveFixture(t, 2, 16, true))
	if err != nil || !validSubtitlePayload(data) {
		t.Fatalf("small archive: %v", err)
	}
}

func TestSubtitleArchiveCancellationRejectsPayload(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	data, err := readArchivedSubtitle(ctx, archiveFixture(t, 1, 0, true))
	if !errors.Is(err, context.Canceled) || len(data) != 0 {
		t.Fatalf("cancelled archive: %v", err)
	}
	reader := subtitleContextReader{ctx, strings.NewReader("payload")}
	if n, err := reader.Read(make([]byte, 16)); n != 0 || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled reader consumed input")
	}
}
