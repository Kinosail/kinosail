// Package webassets owns browser assets shared by Kinosail applications.
package webassets

import (
	"bytes"
	_ "embed"
	"errors"
	"strconv"
	"strings"
)

var (
	//go:embed static/player-app.css
	playerBaseCSS []byte
	//go:embed static/player-stage.css
	playerStageCSS []byte
	PlayerCSS      = append(append([]byte(nil), playerBaseCSS...), playerStageCSS...)
	//go:embed static/subtitles-app.css.patch
	subtitlesCSSPatch []byte

	// SubtitlesCSS derives the Subtitles stylesheet from Player's stylesheet.
	SubtitlesCSS = mustApplyStylePatch(PlayerCSS, subtitlesCSSPatch)
)

var errInvalidStylePatch = errors.New("invalid embedded stylesheet patch")

func mustApplyStylePatch(source, patch []byte) []byte {
	result, err := applyStylePatch(source, patch)
	if err != nil {
		panic(err)
	}
	return result
}

func applyStylePatch(source, patch []byte) ([]byte, error) {
	sourceLines := splitStyleLines(source)
	patchLines := splitStyleLines(patch)
	if len(patchLines) < 2 || !bytes.HasPrefix(patchLines[0], []byte("--- ")) || !bytes.HasPrefix(patchLines[1], []byte("+++ ")) {
		return nil, errInvalidStylePatch
	}
	state := stylePatchState{sourceLines: sourceLines, patchLines: patchLines, patchIndex: 2, result: make([]byte, 0, len(source)+len(patch)/2)}
	if err := state.apply(); err != nil {
		return nil, err
	}
	return state.result, nil
}

type stylePatchState struct {
	sourceLines             [][]byte
	patchLines              [][]byte
	result                  []byte
	sourceIndex, patchIndex int
}

func (state *stylePatchState) apply() error {
	for state.patchIndex < len(state.patchLines) {
		if err := state.applyHunk(); err != nil {
			return err
		}
	}
	for _, line := range state.sourceLines[state.sourceIndex:] {
		state.result = append(state.result, line...)
	}
	return nil
}

func (state *stylePatchState) applyHunk() error { //nolint:cyclop // Unified hunk validation remains below the repository complexity ceiling.
	oldStart, oldCount, newCount, err := parseStyleHunk(state.patchLines[state.patchIndex])
	if err != nil {
		return err
	}
	hunkStart := oldStart - 1
	if oldCount == 0 {
		hunkStart = oldStart
	}
	if hunkStart < state.sourceIndex || hunkStart > len(state.sourceLines) {
		return errInvalidStylePatch
	}
	for _, line := range state.sourceLines[state.sourceIndex:hunkStart] {
		state.result = append(state.result, line...)
	}
	state.sourceIndex = hunkStart
	state.patchIndex++
	removed, added := 0, 0
	for state.patchIndex < len(state.patchLines) && !bytes.HasPrefix(state.patchLines[state.patchIndex], []byte("@@ ")) {
		lineRemoved, lineAdded, err := state.applyLine(state.patchLines[state.patchIndex])
		if err != nil {
			return err
		}
		removed += lineRemoved
		added += lineAdded
		state.patchIndex++
	}
	if removed != oldCount || added != newCount {
		return errInvalidStylePatch
	}
	return nil
}

func (state *stylePatchState) applyLine(line []byte) (int, int, error) {
	if len(line) == 0 {
		return 0, 0, errInvalidStylePatch
	}
	switch line[0] {
	case '-':
		if !state.matchesSource(line[1:]) {
			return 0, 0, errInvalidStylePatch
		}
		state.sourceIndex++
		return 1, 0, nil
	case '+':
		state.result = append(state.result, line[1:]...)
		return 0, 1, nil
	case ' ':
		if !state.matchesSource(line[1:]) {
			return 0, 0, errInvalidStylePatch
		}
		state.result = append(state.result, line[1:]...)
		state.sourceIndex++
		return 1, 1, nil
	default:
		return 0, 0, errInvalidStylePatch
	}
}

func (state *stylePatchState) matchesSource(line []byte) bool {
	return state.sourceIndex < len(state.sourceLines) && bytes.Equal(line, state.sourceLines[state.sourceIndex])
}

func parseStyleHunk(line []byte) (int, int, int, error) {
	fields := strings.Fields(string(line))
	if len(fields) < 4 || fields[0] != "@@" || fields[3] != "@@" || !strings.HasPrefix(fields[1], "-") || !strings.HasPrefix(fields[2], "+") {
		return 0, 0, 0, errInvalidStylePatch
	}
	oldStart, oldCount, err := parseStyleRange(fields[1][1:])
	if err != nil {
		return 0, 0, 0, err
	}
	_, newCount, err := parseStyleRange(fields[2][1:])
	return oldStart, oldCount, newCount, err
}

func parseStyleRange(value string) (int, int, error) {
	parts := strings.SplitN(value, ",", 2)
	start, err := strconv.Atoi(parts[0])
	if err != nil || start < 0 {
		return 0, 0, errInvalidStylePatch
	}
	count := 1
	if len(parts) == 2 {
		count, err = strconv.Atoi(parts[1])
		if err != nil || count < 0 {
			return 0, 0, errInvalidStylePatch
		}
	}
	return start, count, nil
}

func splitStyleLines(content []byte) [][]byte {
	if len(content) == 0 {
		return nil
	}
	lines := bytes.SplitAfter(content, []byte("\n"))
	if len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	return lines
}
