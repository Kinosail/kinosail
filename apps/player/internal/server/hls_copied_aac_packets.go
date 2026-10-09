package server

import "strconv"

type copiedAACPacketReader struct {
	rows                   []copiedHLSAudioPacket
	streams, expectedTrack int
}

func (reader *copiedAACPacketReader) add(line string) error {
	fields := copiedHLSFields(line)
	if _, ok := fields["codec_name"]; ok {
		index, err := strconv.Atoi(fields["index"])
		if err != nil || reader.streams != 0 || !validCopiedAACStream(fields, index, reader.expectedTrack) {
			return errCopiedHLSIndex
		}
		reader.streams++
		return nil
	}
	packet, err := copiedAACParsedPacket(fields)
	if err != nil || len(reader.rows) >= 128 {
		return errCopiedHLSIndex
	}
	reader.rows = append(reader.rows, packet)
	return nil
}

func validCopiedAACStream(fields map[string]string, index, expectedTrack int) bool {
	return fields["codec_name"] == "aac" && fields["profile"] == "LC" && fields["sample_rate"] == "48000" &&
		fields["channels"] == "2" && fields["time_base"] == "1/48000" && copiedAACTrackMatches(index, expectedTrack)
}

func copiedAACTrackMatches(index, expected int) bool {
	return expected < 0 || index == expected
}

func copiedAACParsedPacket(fields map[string]string) (copiedHLSAudioPacket, error) {
	pts, ptsErr := strconv.ParseInt(fields["pts"], 10, 64)
	dts, dtsErr := strconv.ParseInt(fields["dts"], 10, 64)
	duration, durationErr := strconv.ParseInt(fields["duration"], 10, 64)
	packet := copiedHLSAudioPacket{PTS: pts, DTS: dts, Duration: duration, Hash: fields["data_hash"]}
	if ptsErr != nil || dtsErr != nil || durationErr != nil || !validCopiedAACPacket(packet) {
		return packet, errCopiedHLSIndex
	}
	return packet, nil
}
