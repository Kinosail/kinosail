package server

import (
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func subtitleDocument(cues []subtitleCue, duplicates int) (cleanedSubtitle, error) {
	if len(cues) == 0 || len(cues) > maximumSubtitleCues {
		return cleanedSubtitle{}, errors.New("subtitle has no valid cues or exceeds the cue limit")
	}
	var output strings.Builder
	maximumCPS := 0.0
	for index, cue := range cues {
		if cue.Start < 0 || cue.End <= cue.Start || cue.End > 36*time.Hour {
			return cleanedSubtitle{}, errors.New("aligned subtitle timing is invalid")
		}
		plain := subtitleTags.ReplaceAllString(cue.Text, "")
		cps := float64(utf8.RuneCountInString(plain)) / (cue.End - cue.Start).Seconds()
		maximumCPS = max(maximumCPS, cps)
		output.WriteString(strconv.Itoa(index + 1))
		output.WriteByte('\n')
		output.WriteString(formatSubtitleTime(cue.Start))
		output.WriteString(" --> ")
		output.WriteString(formatSubtitleTime(cue.End))
		output.WriteByte('\n')
		output.WriteString(cue.Text)
		output.WriteString("\n\n")
	}
	if output.Len() > 4<<20 {
		return cleanedSubtitle{}, errors.New("converted subtitle is too large")
	}
	return cleanedSubtitle{Data: []byte(output.String()), Cues: cues, MaxCPS: maximumCPS, Duplicates: duplicates}, nil
}

func formatSubtitleTime(value time.Duration) string {
	hours := int(value / time.Hour)
	value -= time.Duration(hours) * time.Hour
	minutes := int(value / time.Minute)
	value -= time.Duration(minutes) * time.Minute
	seconds := int(value / time.Second)
	millis := int((value - time.Duration(seconds)*time.Second) / time.Millisecond)
	return leftPad(hours) + ":" + leftPad(minutes) + ":" + leftPad(seconds) + "," + threeDigits(millis)
}

func leftPad(value int) string {
	if value < 10 {
		return "0" + strconv.Itoa(value)
	}
	return strconv.Itoa(value)
}

func threeDigits(value int) string {
	if value < 10 {
		return "00" + strconv.Itoa(value)
	}
	if value < 100 {
		return "0" + strconv.Itoa(value)
	}
	return strconv.Itoa(value)
}
