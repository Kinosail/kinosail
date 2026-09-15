package credentials

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"errors"
	"testing"
)

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("random source failed") }

func TestLoadCommonPasswordsRejectsInvalidSources(t *testing.T) {
	validButIncomplete := compressedPasswordList(t, "only-one-password\n")
	truncated := compressedPasswordList(t, "password\n")
	truncated = truncated[:len(truncated)-4]
	for name, source := range map[string]string{
		"base64":     "!",
		"gzip":       base64.StdEncoding.EncodeToString([]byte("not gzip")),
		"truncated":  truncated,
		"incomplete": validButIncomplete,
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("invalid password list did not panic")
				}
			}()
			loadCommonPasswords(source)
		})
	}
}

func TestHashReportsRandomSourceFailure(t *testing.T) {
	if encoded, err := hashWithReader("windward-owner-2026", failingReader{}); err == nil || encoded != "" {
		t.Fatalf("hash = %q, %v", encoded, err)
	}
}

func compressedPasswordList(t *testing.T, value string) string {
	t.Helper()
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write([]byte(value)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(compressed.Bytes())
}
