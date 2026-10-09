package server

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

// Offsets address exact known fixture headers; no production parser supplies
// either the mutation coordinates or independently expected packet hashes.
func remainingNonKeyPrivateAudioDamage(t *testing.T, name string, initialization, fragment []byte) ([]byte, []byte) {
	t.Helper()
	switch name {
	case "empty-init":
		return nil, fragment
	case "empty-first":
		return initialization, nil
	case "truncated-first":
		return initialization, fragment[:len(fragment)-1]
	case "duplicate-moov":
		offset := remainingNonKeyPrivateAudioHeader(t, initialization, "moov", 0) - 4
		return append(initialization, initialization[offset:]...), fragment
	case "duplicate-mdat":
		offset := remainingNonKeyPrivateAudioHeader(t, fragment, "mdat", 0) - 4
		return initialization, append(fragment, fragment[offset:]...)
	case "ambiguous-audio":
		offset := remainingNonKeyPrivateAudioHeader(t, initialization, "hdlr", 0) + 12
		copy(initialization[offset:offset+4], "soun")
	case "absent-audio-track":
		offset := remainingNonKeyPrivateAudioHeader(t, fragment, "tfhd", 1) + 8
		binary.BigEndian.PutUint32(fragment[offset:offset+4], 999)
	default:
		remainingNonKeyPrivateAudioFieldDamage(t, name, initialization, fragment)
	}
	return initialization, fragment
}

func remainingNonKeyPrivateAudioFieldDamage(t *testing.T, name string, initialization, fragment []byte) {
	t.Helper()
	if name == "empty-edit" || name == "negative-edit" || name == "edit-rate" {
		remainingNonKeyPrivateAudioEditDamage(t, name, initialization)
		return
	}
	run := remainingNonKeyPrivateAudioHeader(t, fragment, "trun", 1)
	switch name {
	case "audio-sample-count":
		binary.BigEndian.PutUint32(fragment[run+8:run+12], 4097)
	case "audio-offset-header":
		binary.BigEndian.PutUint32(fragment[run+12:run+16], 0)
	case "audio-offset-overflow":
		binary.BigEndian.PutUint32(fragment[run+12:run+16], math.MaxUint32)
	case "audio-overlaps-video":
		video := remainingNonKeyPrivateAudioHeader(t, fragment, "trun", 0)
		copy(fragment[run+12:run+16], fragment[video+12:video+16])
	case "zero-audio-size":
		remainingNonKeyPrivateAudioZeroSize(t, fragment, run)
	default:
		t.Fatal("private AAC unknown fixture damage")
	}
}

func remainingNonKeyPrivateAudioEditDamage(t *testing.T, name string, data []byte) {
	t.Helper()
	edit := remainingNonKeyPrivateAudioHeader(t, data, "elst", 1)
	if data[edit+4] != 0 {
		t.Fatal("private AAC independently mutated edit version changed")
	}
	switch name {
	case "empty-edit":
		binary.BigEndian.PutUint32(data[edit+8:edit+12], 0)
	case "negative-edit":
		binary.BigEndian.PutUint32(data[edit+16:edit+20], math.MaxUint32)
	case "edit-rate":
		binary.BigEndian.PutUint16(data[edit+20:edit+22], 2)
	}
}

func remainingNonKeyPrivateAudioZeroSize(t *testing.T, data []byte, run int) {
	t.Helper()
	flags := binary.BigEndian.Uint32(data[run+4:run+8]) & 0xffffff
	if flags&1 == 0 || flags&0x200 == 0 {
		t.Fatal("private AAC independent sample-size fixture shape changed")
	}
	offset := run + 16
	if flags&4 != 0 {
		offset += 4
	}
	if flags&0x100 != 0 {
		offset += 4
	}
	binary.BigEndian.PutUint32(data[offset:offset+4], 0)
}

func remainingNonKeyPrivateAudioHeader(t *testing.T, data []byte, kind string, ordinal int) int {
	t.Helper()
	start := 0
	for current := 0; current <= ordinal; current++ {
		position := bytes.Index(data[start:], []byte(kind))
		if position < 0 || start+position < 4 {
			t.Fatal("private AAC independently mutated fixture header missing")
		}
		start += position + 4
	}
	return start - 4
}
