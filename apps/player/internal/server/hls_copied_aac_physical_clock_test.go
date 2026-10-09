package server

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Gap: HTTP cannot replace first-packet clocks while atomically rebinding only
// their byte certificate. The unchanged first payload isolates clock proof.
func TestCopiedAACPersistedFirstClockAndDurationCannotInheritWitness(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("qualified retained-source producer is Linux-only")
	}
	for _, damage := range []string{"decode", "duration"} {
		t.Run(damage, func(t *testing.T) {
			manager, directory, held := copiedAACGenerationFixture(t)
			item, recipe, policy := held.item, held.recipe, held.policy
			held.close()
			initialization, first, original, track := copiedAACPhysicalControl(t, directory)
			copiedAACDamageClock(t, first, track, damage)
			facts, err := parseCopiedHLSPrivateAudio(t.Context(), initialization, first)
			if err != nil || facts.FirstPacket != original.FirstPacket {
				t.Fatal("damage changed payload identity or structural admission")
			}
			copiedAACPublishDamagedFirst(t, directory, first, held.certificateData)
			if _, err := manager.readCopiedHLSTimelineContext(t.Context(), directory, policy); err == nil {
				t.Fatal("self-consistent byte certificate admitted an incompatible physical first clock")
			}
			if err := manager.copiedAACCacheAsset(t.Context(), item, recipe, directory, "360p/init.mp4"); err == nil {
				t.Fatal("direct initialization bypassed persisted first-clock rejection")
			}
		})
	}
}

func copiedAACPhysicalControl(t *testing.T, directory string) ([]byte, []byte, *copiedHLSPrivateAudioFacts, uint32) {
	t.Helper()
	first, err := os.ReadFile(filepath.Join(directory, "360p/segment-00000.m4s"))
	if err != nil {
		t.Fatal(err)
	}
	initialization, err := os.ReadFile(filepath.Join(directory, "360p/init.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	tracks, err := copiedHLSPrivateInitialization(initialization)
	if err != nil {
		t.Fatal(err)
	}
	facts, err := parseCopiedHLSPrivateAudio(t.Context(), initialization, first)
	if err != nil {
		t.Fatal(err)
	}
	return initialization, first, facts, tracks.audio.id
}

func copiedAACDamageClock(t *testing.T, first []byte, track uint32, damage string) {
	t.Helper()
	decode, run, header := copiedAACAudioBoxes(t, first, track)
	if damage == "duration" {
		copiedAACDamageFirstDuration(t, run, header)
		return
	}
	if decode.data[0] == 0 {
		binary.BigEndian.PutUint32(decode.data[4:], 1)
	} else {
		binary.BigEndian.PutUint64(decode.data[4:], 1)
	}
}

func copiedAACDamageFirstDuration(t *testing.T, run, header copiedHLSPrivateBox) {
	t.Helper()
	flags, count, position, err := copiedHLSPrivateRunHeader(run.data)
	if err != nil || count == 0 {
		t.Fatal("otherwise-valid first sample run missing")
	}
	position += 4
	if flags&4 != 0 {
		position += 4
	}
	if flags&0x100 != 0 {
		binary.BigEndian.PutUint32(run.data[position:], 1025)
		return
	}
	flags = binary.BigEndian.Uint32(header.data[:4]) & 0xffffff
	position, err = copiedHLSPrivateDescription(header.data, flags)
	if err != nil {
		t.Fatal(err)
	}
	binary.BigEndian.PutUint32(header.data[position:], 1025)
}

func copiedAACPublishDamagedFirst(t *testing.T, directory string, first, originalCertificate []byte) {
	t.Helper()
	var certificate copiedHLSClockCertificate
	if json.Unmarshal(originalCertificate, &certificate) != nil {
		t.Fatal("certificate missing")
	}
	certificate.First = sha256.Sum256(first)
	data, err := json.Marshal(certificate)
	if err != nil {
		t.Fatal(err)
	}
	writeHLSLoadingFile(t, filepath.Join(directory, "360p/segment-00000.m4s"), string(first))
	writeHLSLoadingFile(t, filepath.Join(directory, ".copy-clock"), string(data))
}

func copiedAACAudioBoxes(t *testing.T, first []byte, track uint32) (copiedHLSPrivateBox, copiedHLSPrivateBox, copiedHLSPrivateBox) {
	t.Helper()
	boxes, err := copiedHLSPrivateBoxes(first, "styp", "moof", "mdat", "free")
	if err != nil {
		t.Fatal(err)
	}
	movie, err := copiedHLSPrivateOne(boxes, "moof")
	if err != nil {
		t.Fatal(err)
	}
	children, err := copiedHLSPrivateBoxes(movie.data, "mfhd", "traf")
	if err != nil {
		t.Fatal(err)
	}
	for _, child := range children {
		if child.kind != "traf" {
			continue
		}
		decode, run, header, selected := copiedAACTrackBoxes(t, child.data, track)
		if selected {
			return decode, run, header
		}
	}
	t.Fatal("otherwise-valid selected AAC track missing")
	return copiedHLSPrivateBox{}, copiedHLSPrivateBox{}, copiedHLSPrivateBox{}
}

func copiedAACTrackBoxes(t *testing.T, data []byte, track uint32) (copiedHLSPrivateBox, copiedHLSPrivateBox, copiedHLSPrivateBox, bool) {
	t.Helper()
	parts, err := copiedHLSPrivateBoxes(data, "tfhd", "tfdt", "trun")
	if err != nil {
		t.Fatal(err)
	}
	header, err := copiedHLSPrivateOne(parts, "tfhd")
	if err != nil {
		t.Fatal(err)
	}
	defaults, err := copiedHLSPrivateFragmentDefaults(header.data)
	if err != nil {
		t.Fatal(err)
	}
	if defaults.id != track {
		return copiedHLSPrivateBox{}, copiedHLSPrivateBox{}, copiedHLSPrivateBox{}, false
	}
	decode, err := copiedHLSPrivateOne(parts, "tfdt")
	if err != nil {
		t.Fatal(err)
	}
	run, err := copiedHLSPrivateOne(parts, "trun")
	if err != nil {
		t.Fatal(err)
	}
	return decode, run, header, true
}
