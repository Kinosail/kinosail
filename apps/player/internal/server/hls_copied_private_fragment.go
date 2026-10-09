package server

import (
	"context"
	"encoding/binary"
)

type copiedHLSPrivateSpan struct {
	id                   uint32
	start, end, firstEnd uint64
}

func copiedHLSPrivateFirstAudio(ctx context.Context, data []byte, tracks copiedHLSPrivateTracks) ([]byte, error) {
	boxes, err := copiedHLSPrivateBoxes(data, "styp", "moof", "mdat", "free")
	if err != nil {
		return nil, err
	}
	movie, err := copiedHLSPrivateOne(boxes, "moof")
	if err != nil {
		return nil, err
	}
	media, err := copiedHLSPrivateOne(boxes, "mdat")
	if err != nil || movie.end != media.start {
		return nil, errCopiedHLSIndex
	}
	spans, err := copiedHLSPrivateFragmentTracks(ctx, movie, media)
	if err != nil || !copiedHLSPrivateCompleteSpans(spans, media) {
		return nil, errCopiedHLSIndex
	}
	return copiedHLSPrivateAudioPayload(data, spans, tracks)
}

func copiedHLSPrivateFragmentTracks(ctx context.Context, movie, media copiedHLSPrivateBox) ([]copiedHLSPrivateSpan, error) {
	children, err := copiedHLSPrivateBoxes(movie.data, "mfhd", "traf")
	if err != nil {
		return nil, err
	}
	if !copiedHLSPrivateFirstSequence(children) {
		return nil, errCopiedHLSIndex
	}
	var spans []copiedHLSPrivateSpan
	for _, box := range children {
		if box.kind != "traf" {
			continue
		}
		if len(spans) >= 2 || ctx.Err() != nil {
			return nil, errCopiedHLSIndex
		}
		span, err := copiedHLSPrivateFragmentTrack(ctx, box.data, movie.start, media)
		if err != nil {
			return nil, err
		}
		spans = append(spans, span)
	}
	return spans, nil
}

func copiedHLSPrivateFragmentTrack(ctx context.Context, data []byte, base uint64, media copiedHLSPrivateBox) (copiedHLSPrivateSpan, error) {
	var span copiedHLSPrivateSpan
	children, err := copiedHLSPrivateBoxes(data, "tfhd", "tfdt", "trun")
	if err != nil {
		return span, err
	}
	header, err := copiedHLSPrivateOne(children, "tfhd")
	if err != nil {
		return span, err
	}
	defaults, err := copiedHLSPrivateFragmentDefaults(header.data)
	if err != nil {
		return span, err
	}
	decode, err := copiedHLSPrivateOne(children, "tfdt")
	if err != nil || !copiedHLSPrivateDecodeShape(decode.data) {
		return span, errCopiedHLSIndex
	}
	run, err := copiedHLSPrivateOne(children, "trun")
	if err != nil {
		return span, err
	}
	return copiedHLSPrivateSampleRun(ctx, run.data, defaults, base, media)
}

// Decode clocks remain unsigned. This packet identity reader checks their
// serialized shape without inventing a signed clock or presentation mapping.
func copiedHLSPrivateDecodeShape(data []byte) bool {
	if len(data) < 4 || data[0] > 1 || binary.BigEndian.Uint32(data[:4])&0xffffff != 0 {
		return false
	}
	return data[0] == 0 && len(data) == 8 || data[0] == 1 && len(data) == 12
}

func copiedHLSPrivateCompleteSpans(spans []copiedHLSPrivateSpan, media copiedHLSPrivateBox) bool {
	if len(spans) != 2 || spans[0].id == spans[1].id {
		return false
	}
	if spans[0].start > spans[1].start {
		spans[0], spans[1] = spans[1], spans[0]
	}
	return spans[0].start == media.start+media.header && spans[0].end == spans[1].start && spans[1].end == media.end
}

func copiedHLSPrivateAudioPayload(data []byte, spans []copiedHLSPrivateSpan, tracks copiedHLSPrivateTracks) ([]byte, error) {
	var packet []byte
	for _, span := range spans {
		switch span.id {
		case tracks.audio.id:
			packet = data[span.start:span.firstEnd]
		case tracks.video.id:
		default:
			return nil, errCopiedHLSIndex
		}
	}
	if len(packet) == 0 {
		return nil, errCopiedHLSIndex
	}
	return packet, nil
}

func copiedHLSPrivateFirstSequence(children []copiedHLSPrivateBox) bool {
	header, err := copiedHLSPrivateOne(children, "mfhd")
	return err == nil && len(header.data) == 8 && binary.BigEndian.Uint32(header.data[:4]) == 0 &&
		binary.BigEndian.Uint32(header.data[4:8]) == 1
}
