package server

import "encoding/binary"

// Every absent TFHD description index resolves through its matching TREX.
// The pinned fragmented-copy format uses description1 and zero TREX defaults;
// mandatory TFHD duration, size and flags continue to override those defaults.
func copiedHLSPrivateMovieDefaults(children []copiedHLSPrivateBox, tracks copiedHLSPrivateTracks) error {
	section, err := copiedHLSPrivateOne(children, "mvex")
	if err != nil {
		return err
	}
	defaults, err := copiedHLSPrivateBoxes(section.data, "trex")
	if err != nil || len(defaults) != 2 {
		return errCopiedHLSIndex
	}
	seen := make(map[uint32]bool, 2)
	for _, box := range defaults {
		id, err := copiedHLSPrivateDefaultTrack(box.data)
		if err != nil || seen[id] || id != tracks.audio.id && id != tracks.video.id {
			return errCopiedHLSIndex
		}
		seen[id] = true
	}
	return nil
}

func copiedHLSPrivateDefaultTrack(data []byte) (uint32, error) {
	if len(data) != 24 || binary.BigEndian.Uint32(data[:4]) != 0 ||
		binary.BigEndian.Uint32(data[8:12]) != 1 {
		return 0, errCopiedHLSIndex
	}
	id := binary.BigEndian.Uint32(data[4:8])
	if id == 0 {
		return 0, errCopiedHLSIndex
	}
	for position := 12; position < 24; position += 4 {
		if binary.BigEndian.Uint32(data[position:position+4]) != 0 {
			return 0, errCopiedHLSIndex
		}
	}
	return id, nil
}
