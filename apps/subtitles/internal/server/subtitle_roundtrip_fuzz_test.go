package server

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"
)

// Generated documents contain one ordinary dialogue cue and valid bounded
// timings. Conversion must preserve millisecond timing and remain readable.
func FuzzSubtitleTimingConversionRoundtrip(f *testing.F) {
	for _, start := range []uint32{0, 1, 1001, 3599999, 3600001} {
		for format := uint8(0); format < 4; format++ {
			f.Add(start, uint16(2000), format)
		}
	}
	f.Fuzz(func(t *testing.T, rawStart uint32, rawDuration uint16, format uint8) {
		start, duration := int(rawStart%(35*3600000)), int(rawDuration%60000)+1
		if format%4 == 3 {
			start, duration = start/10*10, (duration/10+1)*10
		}
		input := roundtripSubtitleInput(start, start+duration, format%4)
		cleaned, err := convertSubtitle([]byte(input), "en", subtitleConversionOptions{})
		if err != nil {
			t.Fatalf("valid format %d failed conversion: %v", format%4, err)
		}
		if len(cleaned.Cues) != 1 {
			t.Fatalf("converted cue count = %d", len(cleaned.Cues))
		}
		cue := cleaned.Cues[0]
		if cue.Start/time.Millisecond != time.Duration(start) || cue.End/time.Millisecond != time.Duration(start+duration) {
			t.Fatalf("format %d changed millisecond timing: got %s to %s, want %d to %d ms", format%4, cue.Start, cue.End, start, start+duration)
		}
		again, err := convertSubtitle(cleaned.Data, "en", subtitleConversionOptions{})
		if err != nil || !bytes.Equal(again.Data, cleaned.Data) {
			t.Fatalf("converted subtitle did not round trip: %v", err)
		}
	})
}

func roundtripSubtitleInput(start, end int, format uint8) string {
	left, right := formatSubtitleTime(time.Duration(start)*time.Millisecond), formatSubtitleTime(time.Duration(end)*time.Millisecond)
	switch format {
	case 1:
		return "WEBVTT\n\n" + strings.ReplaceAll(left+" --> "+right, ",", ".") + "\nCafé 世界\n"
	case 2:
		return fmt.Sprintf(`<tt><body><div><p begin="%d.%03ds" end="%d.%03ds">Café 世界</p></div></body></tt>`, start/1000, start%1000, end/1000, end%1000)
	case 3:
		return "[Script Info]\n[Events]\nFormat: Start, End, Text\nDialogue: " + strings.ReplaceAll(left[:len(left)-1], ",", ".") + "," + strings.ReplaceAll(right[:len(right)-1], ",", ".") + ",Café 世界\n"
	default:
		return "1\n" + left + " --> " + right + "\nCafé 世界\n"
	}
}
