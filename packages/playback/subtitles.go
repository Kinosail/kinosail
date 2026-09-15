package playback

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func MapWebVTT(data []byte, timeline Timeline) []byte { //nolint:cyclop,gocognit // Each independent cue is parsed, mapped, or removed in one pass.
	blocks := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n\n")
	result := make([]string, 0, len(blocks))
	for _, block := range blocks {
		lines, mapped := strings.Split(block, "\n"), false
		for index, line := range lines {
			left, right, found := strings.Cut(line, " --> ")
			if !found {
				continue
			}
			end, settings, _ := strings.Cut(right, " ")
			startSeconds, startErr := ParseVTTTime(left)
			endSeconds, endErr := ParseVTTTime(end)
			if startErr != nil || endErr != nil {
				break
			}
			startSeconds, endSeconds = timeline.PresentationTime(startSeconds), timeline.PresentationTime(endSeconds)
			if endSeconds <= startSeconds {
				mapped, lines = true, nil
				break
			}
			lines[index] = FormatVTTTime(startSeconds) + " --> " + FormatVTTTime(endSeconds)
			if settings != "" {
				lines[index] += " " + settings
			}
			mapped = true
			break
		}
		if lines != nil && (mapped || strings.TrimSpace(block) != "") {
			result = append(result, strings.Join(lines, "\n"))
		}
	}
	return []byte(strings.Join(result, "\n\n"))
}

func ParseVTTTime(value string) (float64, error) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, errors.New("subtitle timestamp is invalid")
	}
	seconds, err := strconv.ParseFloat(parts[len(parts)-1], 64)
	if err != nil {
		return 0, errors.New("subtitle timestamp is invalid")
	}
	minutes, err := strconv.Atoi(parts[len(parts)-2])
	if err != nil {
		return 0, errors.New("subtitle timestamp is invalid")
	}
	hours := 0
	if len(parts) == 3 {
		hours, err = strconv.Atoi(parts[0])
	}
	if err != nil {
		return 0, errors.New("subtitle timestamp is invalid")
	}
	return float64(hours*3600+minutes*60) + seconds, nil
}

func FormatVTTTime(value float64) string {
	milliseconds := int64(value*1000 + 0.5)
	hours, remainder := milliseconds/3_600_000, milliseconds%3_600_000
	minutes, remainder := remainder/60_000, remainder%60_000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, remainder/1000, remainder%1000)
}
