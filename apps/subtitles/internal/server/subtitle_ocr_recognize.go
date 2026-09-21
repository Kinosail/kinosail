package server

import (
	"bytes"
	"context"
	"errors"
	"math"
	"os/exec"
	"strconv"
	"strings"
)

func recognizeSubtitleFrame(ctx context.Context, executable, language string, pixels []byte) (string, []subtitleDraftWord, error) {
	image := subtitleOCRImage(pixels, 1920, 1080)
	if len(image) == 0 {
		return "", []subtitleDraftWord{}, nil
	}
	command := exec.CommandContext(ctx, executable, "stdin", "stdout", "-l", language, "--psm", "6", "tsv")
	command.Stdin = bytes.NewReader(image)
	buffer := &subtitleLimitedOutput{limit: 1 << 20}
	command.Stdout = buffer
	if err := command.Run(); err != nil {
		return "", nil, err
	}
	text, recognized, err := parseSubtitleOCR(buffer.Bytes())
	if err != nil {
		return "", nil, err
	}
	if text == "" {
		return "", nil, errors.New("visible bitmap text could not be recognized")
	}
	return text, recognized, nil
}

func validSubtitleOCRCoordinates(fields []string) bool {
	for index, field := range fields {
		number, err := strconv.Atoi(field)
		if err != nil || number < 0 || number > 1000000 || index == 0 && (number < 1 || number > 5) {
			return false
		}
	}
	return true
}

func subtitleOCRWord(fields []string) (string, float64, bool) {
	confidence, err := strconv.ParseFloat(fields[10], 64)
	word := strings.TrimSpace(fields[11])
	valid := err == nil && !math.IsNaN(confidence) && !math.IsInf(confidence, 0) && confidence >= 0 && confidence <= 100 && validSubtitleDraftText(word)
	return word, confidence, valid
}
