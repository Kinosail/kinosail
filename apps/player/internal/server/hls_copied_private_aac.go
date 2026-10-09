package server

import "encoding/binary"

// This fixed diagnostic scope accepts AAC-LC, stereo, 48000 Hz and 1024 samples.
// Unsupported extensions and descriptor ambiguities cannot acquire identity.
func copiedHLSPrivateAudioDescriptor(data []byte, trackID uint32) bool {
	es, err := copiedHLSPrivateElementaryStream(data, trackID)
	if err != nil {
		return false
	}
	decoder, rest, err := copiedHLSPrivateDescriptor(es, 4)
	if err != nil || len(decoder) < 13 || decoder[0] != 0x40 || decoder[1] != 0x15 {
		return false
	}
	if !copiedHLSPrivateSyncDescriptor(rest) {
		return false
	}
	configuration, trailing, err := copiedHLSPrivateDescriptor(decoder[13:], 5)
	return err == nil && len(trailing) == 0 && copiedHLSPrivateAACConfiguration(configuration)
}

func copiedHLSPrivateElementaryStream(data []byte, trackID uint32) ([]byte, error) {
	if len(data) < 4 || binary.BigEndian.Uint32(data[:4]) != 0 {
		return nil, errCopiedHLSIndex
	}
	es, rest, err := copiedHLSPrivateDescriptor(data[4:], 3)
	if err != nil || len(rest) != 0 || len(es) < 3 ||
		uint32(binary.BigEndian.Uint16(es[:2])) != trackID || es[2] != 0 {
		return nil, errCopiedHLSIndex
	}
	return es[3:], nil
}

func copiedHLSPrivateSyncDescriptor(data []byte) bool {
	sl, trailing, err := copiedHLSPrivateDescriptor(data, 6)
	return err == nil && len(trailing) == 0 && len(sl) == 1 && sl[0] == 2
}

func copiedHLSPrivateDescriptor(data []byte, tag byte) ([]byte, []byte, error) {
	if len(data) < 2 || data[0] != tag {
		return nil, nil, errCopiedHLSIndex
	}
	size, position := uint64(0), uint64(1)
	for count := 0; count < 4; count++ {
		if position >= uint64(len(data)) {
			return nil, nil, errCopiedHLSIndex
		}
		value := data[position]
		position++
		size = size<<7 | uint64(value&0x7f)
		if value&0x80 == 0 {
			if size > uint64(len(data))-position {
				return nil, nil, errCopiedHLSIndex
			}
			return data[position : position+size], data[position+size:], nil
		}
	}
	return nil, nil, errCopiedHLSIndex
}

func copiedHLSPrivateAACConfiguration(data []byte) bool {
	if len(data) != 2 && len(data) != 5 {
		return false
	}
	header := binary.BigEndian.Uint16(data[:2])
	if header>>11 != 2 || header>>7&15 != 3 || header>>3&15 != 2 || header&7 != 0 {
		return false
	}
	if len(data) == 2 {
		return true
	}
	// The only supported extension explicitly declares SBR absent. Its
	// remaining seven padding bits must be zero; no implicit rate is inferred.
	extension := binary.BigEndian.Uint16(data[2:4])
	return extension>>5 == 0x2b7 && extension&31 == 5 && data[4] == 0
}
