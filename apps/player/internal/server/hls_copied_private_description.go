package server

import (
	"encoding/binary"

	"github.com/MikeO7/kinosail/packages/isobmff"
)

// A global codec list does not associate a description with its track.
// Validate each complete track and its unique description independently.
func copiedHLSPrivateTrackConfiguration(data []byte, track copiedHLSPrivateTrack) bool {
	config, err := isobmff.Parse(data)
	if err != nil || len(config.Tracks) != 1 || !copiedHLSPrivateTrackCodec(track, config.Tracks[0]) {
		return false
	}
	entry, err := copiedHLSPrivateTrackEntry(data, track.handler)
	if err != nil {
		return false
	}
	if track.handler == "soun" {
		return copiedHLSPrivateAudioEntry(entry.data, track.id)
	}
	return len(entry.data) >= 78 && binary.BigEndian.Uint16(entry.data[6:8]) == 1
}

func copiedHLSPrivateTrackEntry(data []byte, handler string) (copiedHLSPrivateBox, error) {
	children, err := copiedHLSPrivateBoxes(data, "tkhd", "edts", "mdia")
	if err != nil {
		return copiedHLSPrivateBox{}, err
	}
	media, err := copiedHLSPrivateOne(children, "mdia")
	if err != nil {
		return copiedHLSPrivateBox{}, err
	}
	children, err = copiedHLSPrivateBoxes(media.data, "mdhd", "hdlr", "minf")
	if err != nil {
		return copiedHLSPrivateBox{}, err
	}
	information, err := copiedHLSPrivateOne(children, "minf")
	if err != nil {
		return copiedHLSPrivateBox{}, err
	}
	return copiedHLSPrivateInformationEntry(information.data, handler)
}

func copiedHLSPrivateInformationEntry(data []byte, handler string) (copiedHLSPrivateBox, error) {
	children, err := copiedHLSPrivateBoxes(data, "vmhd", "smhd", "dinf", "stbl", "free")
	if err != nil {
		return copiedHLSPrivateBox{}, err
	}
	table, err := copiedHLSPrivateOne(children, "stbl")
	if err != nil {
		return copiedHLSPrivateBox{}, err
	}
	children, err = copiedHLSPrivateBoxes(table.data, "stsd", "stts", "stsc", "stsz", "stco", "co64", "free")
	if err != nil {
		return copiedHLSPrivateBox{}, err
	}
	description, err := copiedHLSPrivateOne(children, "stsd")
	if err != nil {
		return copiedHLSPrivateBox{}, err
	}
	return copiedHLSPrivateDescriptionEntry(description.data, handler)
}

func copiedHLSPrivateDescriptionEntry(data []byte, handler string) (copiedHLSPrivateBox, error) {
	if len(data) < 8 || binary.BigEndian.Uint32(data[:4]) != 0 || binary.BigEndian.Uint32(data[4:8]) != 1 {
		return copiedHLSPrivateBox{}, errCopiedHLSIndex
	}
	kind := "avc1"
	if handler == "soun" {
		kind = "mp4a"
	}
	entries, err := copiedHLSPrivateBoxes(data[8:], kind)
	if err != nil || len(entries) != 1 {
		return copiedHLSPrivateBox{}, errCopiedHLSIndex
	}
	return entries[0], nil
}

func copiedHLSPrivateAudioEntry(data []byte, trackID uint32) bool {
	if len(data) < 28 || binary.BigEndian.Uint16(data[6:8]) != 1 ||
		binary.BigEndian.Uint16(data[8:10]) != 0 || binary.BigEndian.Uint16(data[16:18]) != 2 ||
		binary.BigEndian.Uint32(data[24:28]) != 48000<<16 {
		return false
	}
	children, err := copiedHLSPrivateBoxes(data[28:], "esds", "btrt")
	if err != nil {
		return false
	}
	descriptor, err := copiedHLSPrivateOne(children, "esds")
	return err == nil && copiedHLSPrivateAudioDescriptor(descriptor.data, trackID)
}
