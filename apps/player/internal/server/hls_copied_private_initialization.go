package server

import (
	"bytes"
	"encoding/binary"
	"strings"

	"github.com/MikeO7/kinosail/packages/isobmff"
)

type copiedHLSPrivateTrack struct {
	id, scale uint32
	handler   string
	edit      int64
}

type copiedHLSPrivateTracks struct{ audio, video copiedHLSPrivateTrack }

func copiedHLSPrivateInitialization(data []byte) (copiedHLSPrivateTracks, error) {
	var tracks copiedHLSPrivateTracks
	config, err := isobmff.Parse(data)
	if err != nil || len(config.Tracks) != 2 {
		return tracks, errCopiedHLSIndex
	}
	top, err := copiedHLSPrivateBoxes(data, "ftyp", "moov", "free")
	if err != nil {
		return tracks, err
	}
	if _, err := copiedHLSPrivateOne(top, "ftyp"); err != nil {
		return tracks, err
	}
	movie, err := copiedHLSPrivateOne(top, "moov")
	if err != nil {
		return tracks, err
	}
	return copiedHLSPrivateMovie(movie.data, config)
}

func copiedHLSPrivateMovie(data []byte, config isobmff.Initialization) (copiedHLSPrivateTracks, error) {
	var tracks copiedHLSPrivateTracks
	children, err := copiedHLSPrivateBoxes(data, "mvhd", "trak", "mvex", "udta", "free")
	if err != nil {
		return tracks, err
	}
	header, err := copiedHLSPrivateOne(children, "mvhd")
	if err != nil {
		return tracks, err
	}
	if _, err := copiedHLSPrivateScale(header.data); err != nil {
		return tracks, err
	}
	return copiedHLSPrivateMovieTracks(children, config)
}

func copiedHLSPrivateMovieTracks(children []copiedHLSPrivateBox, config isobmff.Initialization) (copiedHLSPrivateTracks, error) {
	var tracks copiedHLSPrivateTracks
	count := 0
	for _, box := range children {
		if box.kind != "trak" {
			continue
		}
		if count >= len(config.Tracks) {
			return tracks, errCopiedHLSIndex
		}
		track, err := copiedHLSPrivateTrackMetadata(box.data)
		if err != nil || !copiedHLSPrivateTrackConfiguration(box.data, track) {
			return tracks, errCopiedHLSIndex
		}
		if err := tracks.add(track); err != nil {
			return tracks, err
		}
		count++
	}
	if !tracks.complete(count) || copiedHLSPrivateMovieDefaults(children, tracks) != nil {
		return tracks, errCopiedHLSIndex
	}
	return tracks, nil
}

func (tracks *copiedHLSPrivateTracks) complete(count int) bool {
	return count == 2 && tracks.audio.id != 0 && tracks.video.id != 0 && tracks.audio.id != tracks.video.id
}

func (tracks *copiedHLSPrivateTracks) add(track copiedHLSPrivateTrack) error {
	switch track.handler {
	case "vide":
		if tracks.video.id != 0 {
			return errCopiedHLSIndex
		}
		tracks.video = track
	case "soun":
		if tracks.audio.id != 0 {
			return errCopiedHLSIndex
		}
		tracks.audio = track
	default:
		return errCopiedHLSIndex
	}
	return nil
}

func copiedHLSPrivateTrackCodec(track copiedHLSPrivateTrack, config isobmff.Track) bool {
	if track.handler == "soun" {
		return config.Codec == "mp4a.40.2" && config.Width == 0 && track.scale == 48000
	}
	return track.handler == "vide" && strings.HasPrefix(config.Codec, "avc1.") && config.Width > 0 && config.Height > 0
}

func copiedHLSPrivateTrackMetadata(data []byte) (copiedHLSPrivateTrack, error) {
	var track copiedHLSPrivateTrack
	children, err := copiedHLSPrivateBoxes(data, "tkhd", "edts", "mdia")
	if err != nil {
		return track, err
	}
	header, err := copiedHLSPrivateOne(children, "tkhd")
	if err != nil {
		return track, err
	}
	track.id, err = copiedHLSPrivateTrackID(header.data)
	if err != nil {
		return track, err
	}
	media, err := copiedHLSPrivateOne(children, "mdia")
	if err != nil {
		return track, err
	}
	track.scale, track.handler, err = copiedHLSPrivateMedia(media.data)
	if err != nil || track.handler != "soun" {
		return track, err
	}
	section, err := copiedHLSPrivateOne(children, "edts")
	if err != nil {
		return track, err
	}
	edits, err := copiedHLSPrivateBoxes(section.data, "elst")
	if err != nil {
		return track, err
	}
	edit, err := copiedHLSPrivateOne(edits, "elst")
	if err != nil {
		return track, err
	}
	track.edit, err = copiedHLSPrivateEdit(edit.data)
	return track, err
}

func copiedHLSPrivateTrackID(data []byte) (uint32, error) {
	if len(data) < 4 || data[0] > 1 || binary.BigEndian.Uint32(data[:4])&0x00fffff8 != 0 {
		return 0, errCopiedHLSIndex
	}
	offset, length := 12, 84
	if data[0] == 1 {
		offset, length = 20, 96
	}
	if len(data) != length || binary.BigEndian.Uint32(data[offset:offset+4]) == 0 {
		return 0, errCopiedHLSIndex
	}
	return binary.BigEndian.Uint32(data[offset : offset+4]), nil
}

func copiedHLSPrivateMedia(data []byte) (uint32, string, error) {
	children, err := copiedHLSPrivateBoxes(data, "mdhd", "hdlr", "minf")
	if err != nil {
		return 0, "", err
	}
	header, err := copiedHLSPrivateOne(children, "mdhd")
	if err != nil {
		return 0, "", err
	}
	scale, err := copiedHLSPrivateScale(header.data)
	if err != nil {
		return 0, "", err
	}
	handler, err := copiedHLSPrivateOne(children, "hdlr")
	if err != nil || len(handler.data) < 24 || binary.BigEndian.Uint32(handler.data[:4]) != 0 {
		return 0, "", errCopiedHLSIndex
	}
	return scale, string(handler.data[8:12]), nil
}

func copiedHLSPrivateScale(data []byte) (uint32, error) {
	if len(data) < 4 || data[0] > 1 || binary.BigEndian.Uint32(data[:4])&0xffffff != 0 {
		return 0, errCopiedHLSIndex
	}
	offset := 12
	if data[0] == 1 {
		offset = 20
	}
	if len(data) < offset+8 || binary.BigEndian.Uint32(data[offset:offset+4]) == 0 {
		return 0, errCopiedHLSIndex
	}
	return binary.BigEndian.Uint32(data[offset : offset+4]), nil
}

func copiedHLSPrivateEdit(data []byte) (int64, error) {
	if len(data) < 8 || data[0] > 1 || binary.BigEndian.Uint32(data[:4])&0xffffff != 0 ||
		binary.BigEndian.Uint32(data[4:8]) != 1 {
		return 0, errCopiedHLSIndex
	}
	var value int64
	var err error
	if data[0] == 0 {
		value, err = copiedHLSPrivateEdit32(data)
	} else {
		value, err = copiedHLSPrivateEdit64(data)
	}
	if err != nil || value < 0 {
		return 0, errCopiedHLSIndex
	}
	return value, nil
}

func copiedHLSPrivateEdit32(data []byte) (int64, error) {
	var value int32
	if len(data) != 20 || binary.BigEndian.Uint32(data[8:12]) != 0 ||
		binary.BigEndian.Uint32(data[16:20]) != 0x00010000 || binary.Read(bytes.NewReader(data[12:16]), binary.BigEndian, &value) != nil {
		return 0, errCopiedHLSIndex
	}
	return int64(value), nil
}

func copiedHLSPrivateEdit64(data []byte) (int64, error) {
	var value int64
	if len(data) != 28 || binary.BigEndian.Uint64(data[8:16]) != 0 ||
		binary.BigEndian.Uint32(data[24:28]) != 0x00010000 || binary.Read(bytes.NewReader(data[16:24]), binary.BigEndian, &value) != nil {
		return 0, errCopiedHLSIndex
	}
	return value, nil
}
