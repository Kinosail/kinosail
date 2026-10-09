package server

import (
	"bytes"
	"encoding/binary"
	"testing"
)

type remainingNonKeyPrivateFixtureBox struct {
	kind string
	data []byte
}

// Only initialization ancestors are rewritten. Actual sample entries and the
// fragment payload remain unchanged, exposing global-list codec misassociation.
func remainingNonKeyPrivateAudioStructureDamage(t *testing.T, name string, data []byte) []byte {
	t.Helper()
	return remainingNonKeyPrivateFixtureRewrite(t, data, "moov", 0, func(movie []byte) []byte {
		if name == "misbound-audio-stsd" {
			table := remainingNonKeyPrivateFixtureAudioTable(t, movie)
			movie = remainingNonKeyPrivateFixtureAudioMedia(t, movie, func(media []byte) []byte {
				return remainingNonKeyPrivateFixtureRewrite(t, media, "minf", 0, func(info []byte) []byte {
					return remainingNonKeyPrivateFixtureRewrite(t, info, "stbl", 0, func(_ []byte) []byte { return nil })
				})
			})
			return remainingNonKeyPrivateFixtureRewrite(t, movie, "trak", 0, func(track []byte) []byte {
				return remainingNonKeyPrivateFixtureRewrite(t, track, "mdia", 0, func(media []byte) []byte {
					return remainingNonKeyPrivateFixtureRewrite(t, media, "minf", 0, func(info []byte) []byte {
						return append(info, remainingNonKeyPrivateFixtureEncode("stbl", table)...)
					})
				})
			})
		}
		return remainingNonKeyPrivateAudioOtherStructure(t, name, movie)
	})
}

func remainingNonKeyPrivateAudioOtherStructure(t *testing.T, name string, movie []byte) []byte {
	t.Helper()
	return remainingNonKeyPrivateFixtureAudioMedia(t, movie, func(media []byte) []byte {
		switch name {
		case "missing-audio-minf":
			return remainingNonKeyPrivateFixtureRewrite(t, media, "minf", 0, func(_ []byte) []byte { return nil })
		case "duplicate-audio-minf":
			info := remainingNonKeyPrivateFixtureSelect(t, media, "minf", 0)
			empty := remainingNonKeyPrivateFixtureRewrite(t, info, "stbl", 0, func(_ []byte) []byte { return nil })
			return append(media, remainingNonKeyPrivateFixtureEncode("minf", empty)...)
		case "duplicate-audio-stsd":
			return remainingNonKeyPrivateFixtureRewrite(t, media, "minf", 0, func(info []byte) []byte {
				return remainingNonKeyPrivateFixtureRewrite(t, info, "stbl", 0, func(table []byte) []byte {
					description := remainingNonKeyPrivateFixtureSelect(t, table, "stsd", 0)
					return append(table, remainingNonKeyPrivateFixtureEncode("stsd", description)...)
				})
			})
		default:
			t.Fatal("private AAC unknown configuration structure damage")
			return nil
		}
	})
}

func remainingNonKeyPrivateFixtureAudioMedia(t *testing.T, movie []byte, mutate func([]byte) []byte) []byte {
	t.Helper()
	return remainingNonKeyPrivateFixtureRewrite(t, movie, "trak", 1, func(track []byte) []byte {
		return remainingNonKeyPrivateFixtureRewrite(t, track, "mdia", 0, mutate)
	})
}

func remainingNonKeyPrivateFixtureAudioTable(t *testing.T, movie []byte) []byte {
	t.Helper()
	track := remainingNonKeyPrivateFixtureSelect(t, movie, "trak", 1)
	media := remainingNonKeyPrivateFixtureSelect(t, track, "mdia", 0)
	info := remainingNonKeyPrivateFixtureSelect(t, media, "minf", 0)
	return bytes.Clone(remainingNonKeyPrivateFixtureSelect(t, info, "stbl", 0))
}

func remainingNonKeyPrivateFixtureSelect(t *testing.T, data []byte, kind string, ordinal int) []byte {
	t.Helper()
	for _, box := range remainingNonKeyPrivateFixtureChildren(t, data) {
		if box.kind != kind {
			continue
		}
		if ordinal == 0 {
			return box.data
		}
		ordinal--
	}
	t.Fatal("private AAC independent fixture section missing")
	return nil
}

func remainingNonKeyPrivateFixtureRewrite(t *testing.T, data []byte, kind string, ordinal int, mutate func([]byte) []byte) []byte {
	t.Helper()
	var result []byte
	changed := false
	for _, box := range remainingNonKeyPrivateFixtureChildren(t, data) {
		if box.kind == kind {
			if ordinal == 0 {
				box.data, changed = mutate(box.data), true
			}
			ordinal--
		}
		if box.data != nil {
			result = append(result, remainingNonKeyPrivateFixtureEncode(box.kind, box.data)...)
		}
	}
	if !changed {
		t.Fatal("private AAC independent rewrite section missing")
	}
	return result
}

func remainingNonKeyPrivateFixtureChildren(t *testing.T, data []byte) []remainingNonKeyPrivateFixtureBox {
	t.Helper()
	var boxes []remainingNonKeyPrivateFixtureBox
	for position := uint64(0); position < uint64(len(data)); {
		if len(boxes) >= 64 || uint64(len(data))-position < 8 {
			t.Fatal("private AAC independent fixture child bounds changed")
		}
		size := uint64(binary.BigEndian.Uint32(data[position : position+4]))
		header := uint64(8)
		if size == 1 {
			if uint64(len(data))-position < 16 {
				t.Fatal("private AAC independent extended fixture header truncated")
			}
			size, header = binary.BigEndian.Uint64(data[position+8:position+16]), 16
		}
		if size < header || size > uint64(len(data))-position {
			t.Fatal("private AAC independent fixture extent changed")
		}
		boxes = append(boxes, remainingNonKeyPrivateFixtureBox{
			kind: string(data[position+4 : position+8]),
			data: data[position+header : position+size],
		})
		position += size
	}
	return boxes
}

func remainingNonKeyPrivateFixtureEncode(kind string, data []byte) []byte {
	result := make([]byte, 16, 16+len(data))
	binary.BigEndian.PutUint32(result[:4], 1)
	copy(result[4:8], kind)
	binary.BigEndian.PutUint64(result[8:16], uint64(len(data))+16)
	return append(result, data...)
}
