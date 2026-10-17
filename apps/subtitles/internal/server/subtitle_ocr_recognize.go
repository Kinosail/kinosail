package server

import (
	"bytes"
	"context"
	"errors"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/workload"
)

func recognizeSubtitleImage(ctx context.Context, executable, language string, image []byte) (string, []subtitleDraftWord, error) {
	command := exec.CommandContext(ctx, executable, "stdin", "stdout", "-l", language, "--psm", "6", "tsv")
	command.Env = append(os.Environ(), "OMP_THREAD_LIMIT=1")
	command.Stdin = bytes.NewReader(image)
	buffer := &subtitleLimitedOutput{limit: 1 << 20}
	command.Stdout = buffer
	if err := workload.RunBackground(command); err != nil {
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
