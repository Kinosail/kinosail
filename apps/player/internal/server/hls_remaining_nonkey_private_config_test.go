package server

import (
	"bytes"
	"context"
	"encoding/binary"
	"testing"
)

// These mutations address the independently known actual encoder fixture.
// They do not use the production reader to locate or validate configuration.
func remainingNonKeyPrivateAudioConfigurationRejects(t *testing.T, ctx context.Context, initialization, fragment []byte) {
	t.Helper()
	for _, name := range []string{
		"entry-channels", "entry-rate", "entry-fractional-rate",
		"asc-rate", "asc-channels", "asc-short-frame", "trex-description", "trex-track",
		"trex-duplicate-track", "missing-trex", "missing-audio-minf", "duplicate-audio-minf",
		"misbound-audio-stsd", "duplicate-audio-stsd",
	} {
		t.Run(name, func(t *testing.T) {
			damage := remainingNonKeyPrivateAudioConfigurationDamage(t, name, bytes.Clone(initialization))
			remainingNonKeyPrivateAudioRejectInput(t, ctx, damage, fragment, "nonkey inconsistent private AAC configuration acquired identity")
		})
	}
}

func remainingNonKeyPrivateAudioConfigurationDamage(t *testing.T, name string, data []byte) []byte {
	t.Helper()
	switch name {
	case "entry-channels", "entry-rate", "entry-fractional-rate":
		remainingNonKeyPrivateAudioEntryDamage(t, name, data)
	case "asc-rate", "asc-channels", "asc-short-frame":
		remainingNonKeyPrivateAudioASCDamage(t, name, data)
	case "trex-description", "trex-track", "trex-duplicate-track", "missing-trex":
		remainingNonKeyPrivateAudioTREXDamage(t, name, data)
	default:
		return remainingNonKeyPrivateAudioStructureDamage(t, name, data)
	}
	return data
}

func remainingNonKeyPrivateAudioEntryDamage(t *testing.T, name string, data []byte) {
	t.Helper()
	entry := remainingNonKeyPrivateAudioHeader(t, data, "mp4a", 0)
	if entry+32 > len(data) || binary.BigEndian.Uint16(data[entry+12:entry+14]) != 0 ||
		binary.BigEndian.Uint16(data[entry+20:entry+22]) != 2 ||
		binary.BigEndian.Uint32(data[entry+28:entry+32]) != 48000<<16 {
		t.Fatal("private AAC independent sample-entry fixture shape changed")
	}
	switch name {
	case "entry-channels":
		binary.BigEndian.PutUint16(data[entry+20:entry+22], 1)
	case "entry-rate":
		binary.BigEndian.PutUint32(data[entry+28:entry+32], 44100<<16)
	case "entry-fractional-rate":
		binary.BigEndian.PutUint32(data[entry+28:entry+32], (48000<<16)|1)
	}
}

func remainingNonKeyPrivateAudioASCDamage(t *testing.T, name string, data []byte) {
	t.Helper()
	asc := remainingNonKeyPrivateAudioASCFixture(t, data)
	switch name {
	case "asc-rate":
		asc[0], asc[1] = 0x12, asc[1]&0x7f
	case "asc-channels":
		asc[1] = asc[1]&0x87 | 1<<3
	case "asc-short-frame":
		asc[1] |= 4
	}
}

func remainingNonKeyPrivateAudioASCFixture(t *testing.T, data []byte) []byte {
	t.Helper()
	esds := remainingNonKeyPrivateAudioHeader(t, data, "esds", 0)
	box := data[esds-4:]
	size := binary.BigEndian.Uint32(box[:4])
	if uint64(size) > uint64(len(box)) || size < 12 {
		t.Fatal("private AAC independent descriptor fixture bounds changed")
	}
	descriptor := box[12:size]
	es := remainingNonKeyPrivateAudioDescriptor(t, descriptor, 3)
	if len(es) < 3 || es[2] != 0 {
		t.Fatal("private AAC independent ES descriptor shape changed")
	}
	decoder := remainingNonKeyPrivateAudioDescriptor(t, es[3:], 4)
	if len(decoder) < 13 || decoder[0] != 0x40 {
		t.Fatal("private AAC independent decoder descriptor shape changed")
	}
	asc := remainingNonKeyPrivateAudioDescriptor(t, decoder[13:], 5)
	if len(asc) < 2 || asc[0] != 0x11 || asc[1] != 0x90 {
		t.Fatal("private AAC independent LC48000 stereo1024 fixture changed")
	}
	return asc
}

func remainingNonKeyPrivateAudioDescriptor(t *testing.T, data []byte, tag byte) []byte {
	t.Helper()
	if len(data) < 2 || data[0] != tag {
		t.Fatal("private AAC independent descriptor tag changed")
	}
	size, position := uint64(0), uint64(1)
	for count := 0; count < 4; count++ {
		if position >= uint64(len(data)) {
			t.Fatal("private AAC independent descriptor length truncated")
		}
		value := data[position]
		position++
		size = size<<7 | uint64(value&0x7f)
		if value&0x80 == 0 {
			if size > uint64(len(data))-position {
				t.Fatal("private AAC independent descriptor payload truncated")
			}
			return data[position : position+size]
		}
	}
	t.Fatal("private AAC independent descriptor length unterminated")
	return nil
}

func remainingNonKeyPrivateAudioTREXDamage(t *testing.T, name string, data []byte) {
	t.Helper()
	audio := remainingNonKeyPrivateAudioHeader(t, data, "trex", 1)
	video := remainingNonKeyPrivateAudioHeader(t, data, "trex", 0)
	if audio+28 > len(data) || binary.BigEndian.Uint32(data[audio+8:audio+12]) != 2 ||
		binary.BigEndian.Uint32(data[audio+12:audio+16]) != 1 {
		t.Fatal("private AAC independent TREX fixture shape changed")
	}
	switch name {
	case "trex-description":
		binary.BigEndian.PutUint32(data[audio+12:audio+16], 2)
	case "trex-track":
		binary.BigEndian.PutUint32(data[audio+8:audio+12], 999)
	case "trex-duplicate-track":
		copy(data[audio+8:audio+12], data[video+8:video+12])
	case "missing-trex":
		copy(data[audio:audio+4], "free")
	}
}
