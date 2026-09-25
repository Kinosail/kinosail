package credentials

import (
	"errors"
	"testing"
)

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("random source failed") }

func TestHashReportsRandomSourceFailure(t *testing.T) {
	if encoded, err := hashWithReader("windward-owner-2026", failingReader{}); err == nil || encoded != "" {
		t.Fatalf("hash = %q, %v", encoded, err)
	}
}
