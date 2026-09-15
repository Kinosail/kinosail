package isobmff

import (
	"encoding/binary"
	"math"
)

func sampleEntry(format string, entry []byte) (Track, error) { //nolint:cyclop // Each sample-entry type validates its fixed header before reading child boxes.
	var track Track
	var offset int
	switch format {
	case "avc1", "avc3", "hvc1", "hev1", "av01", "vp09", "dvh1", "dvhe":
		if len(entry) < 78 {
			return Track{}, ErrInvalid
		}
		track.Width, track.Height = int(binary.BigEndian.Uint16(entry[24:])), int(binary.BigEndian.Uint16(entry[26:]))
		if track.Width == 0 || track.Height == 0 {
			return Track{}, ErrInvalid
		}
		track.Range, offset = "SDR", 78
	case "mp4a", "Opus", "ac-3", "ec-3", "fLaC", ".mp3":
		if len(entry) < 28 || binary.BigEndian.Uint16(entry[8:]) != 0 {
			return Track{}, ErrInvalid
		}
		offset = 28
		track.Codec = map[string]string{"Opus": "opus", "ac-3": "ac-3", "ec-3": "ec-3", "fLaC": "fLaC", ".mp3": "mp4a.6B"}[format]
	default:
		return Track{}, ErrInvalid
	}
	err := boxes(entry[offset:], func(kind string, payload []byte) error {
		return track.configuration(format, kind, payload)
	})
	if err != nil || track.Codec == "" {
		return Track{}, ErrInvalid
	}
	return track, nil
}

func (track *Track) configuration(format, kind string, payload []byte) error {
	switch kind {
	case "avcC", "hvcC", "av1C", "vpcC", "esds", "dvcC", "dvvC":
		codec, err := codecConfiguration(format, kind, payload)
		if err != nil {
			return err
		}
		if codec != "" {
			track.Codec = codec
		}
		track.readBitDepth(kind, payload)
	case "pasp":
		return track.pixelAspect(payload)
	case "colr":
		track.readColor(payload)
	}
	return nil
}

// readBitDepth follows codecConfiguration validation. Dolby Vision may ignore hvcC, so that box retains its length guard.
func (track *Track) readBitDepth(kind string, payload []byte) {
	switch kind {
	case "avcC":
		if payload[1] == 66 || payload[1] == 77 || payload[1] == 88 || payload[1] == 100 {
			track.BitDepth = 8
		}
	case "hvcC":
		if len(payload) >= 19 {
			track.BitDepth = 8 + int(payload[17]&7)
		}
	case "av1C":
		track.BitDepth = av1BitDepth(payload[2])
	case "vpcC":
		track.BitDepth = int(payload[6] >> 4)
	}
}

func (track *Track) pixelAspect(payload []byte) error {
	if len(payload) != 8 || track.Width < 0 || track.Width > math.MaxInt32 {
		return ErrInvalid
	}
	numerator, denominator := binary.BigEndian.Uint32(payload), binary.BigEndian.Uint32(payload[4:])
	if numerator == 0 || denominator == 0 || numerator > 10000 || denominator > 10000 {
		return ErrInvalid
	}
	width := (uint64(track.Width)*uint64(numerator) + uint64(denominator)/2) / uint64(denominator)
	if width > math.MaxInt32 {
		return ErrInvalid
	}
	track.Width = int(width)
	return nil
}

func (track *Track) readColor(payload []byte) {
	if len(payload) < 10 || (string(payload[:4]) != "nclx" && string(payload[:4]) != "nclc") {
		return
	}
	switch binary.BigEndian.Uint16(payload[6:]) {
	case 16:
		track.Range = "PQ"
	case 18:
		track.Range = "HLG"
	}
}
