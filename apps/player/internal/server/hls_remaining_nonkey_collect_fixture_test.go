package server

import (
	"encoding/binary"
	"testing"
)

// Independent fixed-fixture inspection, not a production parser or eligibility
// rule. Generated private initialization has a unique single audio edit entry.
func remainingNonKeyCollectorAudioEdit(t *testing.T, data []byte) int64 {
	t.Helper()
	movie := remainingNonKeyCollectorBox(t, data, "moov")
	var audio []byte
	for len(movie) > 0 {
		size, kind, payload := remainingNonKeyCollectorNextBox(t, movie)
		movie = movie[size:]
		if kind != "trak" {
			continue
		}
		media := remainingNonKeyCollectorBox(t, payload, "mdia")
		handler := remainingNonKeyCollectorBox(t, media, "hdlr")
		if len(handler) < 12 || string(handler[8:12]) != "soun" {
			continue
		}
		if audio != nil {
			t.Fatal("source-clock private fixture has ambiguous audio tracks")
		}
		audio = remainingNonKeyCollectorBox(t, remainingNonKeyCollectorBox(t, payload, "edts"), "elst")
	}
	if len(audio) < 8 || binary.BigEndian.Uint32(audio[4:8]) != 1 {
		t.Fatal("source-clock private fixture audio edit missing")
	}
	switch audio[0] {
	case 0:
		if len(audio) == 20 {
			return int64(int32(binary.BigEndian.Uint32(audio[12:16])))
		}
	case 1:
		if len(audio) == 28 {
			return int64(binary.BigEndian.Uint64(audio[16:24]))
		}
	}
	t.Fatal("source-clock private fixture edit shape")
	return 0
}

func remainingNonKeyCollectorBox(t *testing.T, data []byte, requested string) []byte {
	t.Helper()
	var result []byte
	for len(data) > 0 {
		size, kind, payload := remainingNonKeyCollectorNextBox(t, data)
		if kind == requested {
			if result != nil {
				t.Fatal("source-clock private fixture duplicate box")
			}
			result = payload
		}
		data = data[size:]
	}
	if result == nil {
		t.Fatal("source-clock private fixture missing box")
	}
	return result
}

func remainingNonKeyCollectorNextBox(t *testing.T, data []byte) (int, string, []byte) {
	t.Helper()
	if len(data) < 8 {
		t.Fatal("source-clock private fixture box header")
	}
	size := int(binary.BigEndian.Uint32(data))
	if size < 8 || size > len(data) {
		t.Fatal("source-clock private fixture box extent")
	}
	return size, string(data[4:8]), data[8:size]
}
