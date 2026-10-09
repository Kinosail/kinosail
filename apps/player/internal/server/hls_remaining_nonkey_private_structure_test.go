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
	result := remainingNonKeyPrivateFixtureRewrite(t, data, "moov", 0, func(movie []byte) []byte {
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
	if name == "misbound-audio-stsd" {
		remainingNonKeyPrivateFixtureMovedTable(t, data, result)
	}
	return result
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
				box.data, changed = mutate(bytes.Clone(box.data)), true
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

// The moved-table counterexample must retain both original sample descriptions,
// unchanged track metadata and the AAC table physically inside the video track.
func remainingNonKeyPrivateFixtureMovedTable(t *testing.T, original, changed []byte) {
	t.Helper()
	beforeMovie := remainingNonKeyPrivateFixtureSelect(t, original, "moov", 0)
	afterMovie := remainingNonKeyPrivateFixtureSelect(t, changed, "moov", 0)
	beforeAudio := remainingNonKeyPrivateFixtureSelect(t, beforeMovie, "trak", 1)
	afterAudio := remainingNonKeyPrivateFixtureSelect(t, afterMovie, "trak", 1)
	beforeVideo := remainingNonKeyPrivateFixtureSelect(t, beforeMovie, "trak", 0)
	afterVideo := remainingNonKeyPrivateFixtureSelect(t, afterMovie, "trak", 0)
	for _, pair := range [][2][]byte{{beforeVideo, afterVideo}, {beforeAudio, afterAudio}} {
		if !bytes.Equal(remainingNonKeyPrivateFixtureSelect(t, pair[0], "tkhd", 0),
			remainingNonKeyPrivateFixtureSelect(t, pair[1], "tkhd", 0)) {
			t.Fatal("private AAC moved-table witness changed track identity")
		}
		remainingNonKeyPrivateFixtureMediaIdentity(t, pair[0], pair[1])
	}
	audioTable := remainingNonKeyPrivateFixtureAudioTable(t, beforeMovie)
	originalMedia := remainingNonKeyPrivateFixtureSelect(t, beforeVideo, "mdia", 0)
	originalInfo := remainingNonKeyPrivateFixtureSelect(t, originalMedia, "minf", 0)
	originalVideoTable := remainingNonKeyPrivateFixtureSelect(t, originalInfo, "stbl", 0)
	videoMedia := remainingNonKeyPrivateFixtureSelect(t, afterVideo, "mdia", 0)
	videoInfo := remainingNonKeyPrivateFixtureSelect(t, videoMedia, "minf", 0)
	if !bytes.Equal(audioTable, remainingNonKeyPrivateFixtureSelect(t, videoInfo, "stbl", 1)) ||
		!bytes.Equal(originalVideoTable, remainingNonKeyPrivateFixtureSelect(t, videoInfo, "stbl", 0)) {
		t.Fatal("private AAC moved-table witness changed the AAC table")
	}
	audioMedia := remainingNonKeyPrivateFixtureSelect(t, afterAudio, "mdia", 0)
	audioInfo := remainingNonKeyPrivateFixtureSelect(t, audioMedia, "minf", 0)
	for _, box := range remainingNonKeyPrivateFixtureChildren(t, audioInfo) {
		if box.kind == "stbl" {
			t.Fatal("private AAC moved-table witness retained the audio table")
		}
	}
}

func remainingNonKeyPrivateFixtureMediaIdentity(t *testing.T, original, changed []byte) {
	t.Helper()
	before := remainingNonKeyPrivateFixtureSelect(t, original, "mdia", 0)
	after := remainingNonKeyPrivateFixtureSelect(t, changed, "mdia", 0)
	for _, kind := range []string{"mdhd", "hdlr"} {
		if !bytes.Equal(remainingNonKeyPrivateFixtureSelect(t, before, kind, 0),
			remainingNonKeyPrivateFixtureSelect(t, after, kind, 0)) {
			t.Fatal("private AAC moved-table witness changed media metadata")
		}
	}
}
