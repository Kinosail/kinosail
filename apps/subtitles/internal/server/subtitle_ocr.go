package server

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func subtitleOCRLanguage(language string) string {
	language = strings.ToLower(language)
	languages := map[string]string{"en": "eng", "es": "spa", "fr": "fra", "de": "deu", "it": "ita", "pt": "por", "nl": "nld", "sv": "swe", "da": "dan", "no": "nor", "fi": "fin", "pl": "pol", "cs": "ces", "ro": "ron", "hu": "hun", "tr": "tur", "el": "ell", "ru": "rus", "uk": "ukr", "ar": "ara", "he": "heb", "hi": "hin", "ja": "jpn", "ko": "kor", "zh": "chi_sim"}
	if strings.HasPrefix(language, "zh-hant") || language == "zh-tw" || language == "zh-hk" {
		return "chi_tra"
	}
	return languages[strings.Split(language, "-")[0]]
}

var subtitleFrameTime = regexp.MustCompile(`\bn:\s*\d+\s+pts:\s*\S+\s+pts_time:([0-9.eE+\-]+)`)

func localSubtitleOCR(ctx context.Context, ffmpeg, tesseract, media string, stream int, language string, duration float64) (cleanedSubtitle, []subtitleDraftWord, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	// Render only the bitmap subtitle plane onto a plain canvas. Film pixels
	// never reach OCR. Identical frames disappear; clear frames close each cue.
	filter := fmt.Sprintf("color=c=black:s=1920x1080:r=25:d=%.3f[bg];[0:%d]scale=1920:1080[subs];[bg][subs]overlay=eof_action=pass:repeatlast=0,format=gray,negate,mpdecimate=hi=1:lo=1:frac=0,showinfo[out]", duration, stream)
	command := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-nostdin", "-loglevel", "info", "-i", media, "-filter_complex", filter, "-map", "[out]", "-an", "-t", strconv.FormatFloat(duration, 'f', 3, 64), "-fps_mode", "vfr", "-pix_fmt", "gray", "-f", "rawvideo", "pipe:1")
	output, err := command.StdoutPipe()
	if err != nil {
		return cleanedSubtitle{}, nil, err
	}
	logs, err := command.StderrPipe()
	if err != nil {
		return cleanedSubtitle{}, nil, err
	}
	if err = command.Start(); err != nil {
		return cleanedSubtitle{}, nil, err
	}
	times := make(chan float64, 8)
	go func() {
		defer close(times)
		scanner := bufio.NewScanner(logs)
		scanner.Buffer(make([]byte, 4096), 64<<10)
		for scanner.Scan() {
			match := subtitleFrameTime.FindStringSubmatch(scanner.Text())
			if len(match) != 2 {
				continue
			}
			at, parseErr := strconv.ParseFloat(match[1], 64)
			if parseErr != nil || math.IsNaN(at) || math.IsInf(at, 0) || at < 0 || at > duration+1 {
				cancel()
				return
			}
			select {
			case times <- at:
			case <-ctx.Done():
				return
			}
		}
		if scanner.Err() != nil {
			cancel()
		}
	}()
	cues, words, readErr := readSubtitleOCRFrames(ctx, output, times, tesseract, language, duration)
	if readErr != nil {
		cancel()
	}
	waitErr := command.Wait()
	if readErr != nil {
		return cleanedSubtitle{}, nil, readErr
	}
	if waitErr != nil {
		return cleanedSubtitle{}, nil, waitErr
	}
	document, err := subtitleDocument(cues, 0)
	if err == nil {
		document.Original = append([]byte(nil), document.Data...)
		document.TimingEvidence = "unverified"
	}
	return document, words, err
}

func readSubtitleOCRFrames(ctx context.Context, output io.Reader, times <-chan float64, executable, language string, duration float64) ([]subtitleCue, []subtitleDraftWord, error) {
	pixels := make([]byte, 1920*1080)
	cues, words := []subtitleCue{}, []subtitleDraftWord{}
	previous, previousWords, start, last := "", []subtitleDraftWord{}, 0.0, -1.0
	closeCue := func(end float64) {
		if previous == "" || end <= start {
			return
		}
		cues = append(cues, subtitleCue{Start: time.Duration(start * float64(time.Second)), End: time.Duration(end * float64(time.Second)), Text: previous})
		for _, word := range previousWords {
			word.Start, word.End = start, end
			words = append(words, word)
		}
	}
	for count := 0; ; count++ {
		_, err := io.ReadFull(output, pixels)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil || count >= 40000 {
			return nil, nil, errors.New("bitmap subtitle frame limit exceeded")
		}
		var at float64
		select {
		case value, ok := <-times:
			if !ok {
				return nil, nil, errors.New("bitmap subtitle timing is unavailable")
			}
			at = value
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		}
		if at < last {
			return nil, nil, errors.New("bitmap subtitle times are out of order")
		}
		last = at
		image := subtitleOCRImage(pixels, 1920, 1080)
		text, recognized := "", []subtitleDraftWord{}
		if len(image) != 0 {
			command := exec.CommandContext(ctx, executable, "stdin", "stdout", "-l", language, "--psm", "6", "tsv")
			command.Stdin = bytes.NewReader(image)
			buffer := &subtitleLimitedOutput{limit: 1 << 20}
			command.Stdout = buffer
			if err = command.Run(); err != nil {
				return nil, nil, err
			}
			text, recognized, err = parseSubtitleOCR(buffer.Bytes())
			if err != nil {
				return nil, nil, err
			}
			if text == "" {
				return nil, nil, errors.New("visible bitmap text could not be recognized")
			}
		}
		if text != previous {
			closeCue(at)
			previous, previousWords, start = text, recognized, at
		}
		if len(cues) > 20000 || len(words) > 100000 {
			return nil, nil, errors.New("bitmap subtitle output exceeds the limit")
		}
	}
	closeCue(duration)
	if len(cues) > 20000 || len(words) > 100000 {
		return nil, nil, errors.New("bitmap subtitle output exceeds the limit")
	}
	return cues, words, nil
}

type subtitleLimitedOutput struct {
	bytes.Buffer
	limit int
}

func (output *subtitleLimitedOutput) Write(data []byte) (int, error) {
	if output.Len()+len(data) > output.limit {
		return 0, errors.New("local subtitle output exceeds the limit")
	}
	return output.Buffer.Write(data)
}

func subtitleOCRImage(pixels []byte, width, height int) []byte {
	if width <= 0 || height <= 0 || width > 1920 || height > 1080 || len(pixels) != width*height {
		return nil
	}
	left, top, right, bottom := width, height, -1, -1
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if pixels[y*width+x] < 180 {
				left, top, right, bottom = min(left, x), min(top, y), max(right, x), max(bottom, y)
			}
		}
	}
	if right < left {
		return nil
	}
	left, top, right, bottom = max(0, left-20), max(0, top-20), min(width-1, right+20), min(height-1, bottom+20)
	image := []byte(fmt.Sprintf("P5\n%d %d\n255\n", right-left+1, bottom-top+1))
	for y := top; y <= bottom; y++ {
		image = append(image, pixels[y*width+left:y*width+right+1]...)
	}
	return image
}

func parseSubtitleOCR(data []byte) (string, []subtitleDraftWord, error) {
	invalid := errors.New("OCR output is invalid")
	if len(data) == 0 || len(data) > 1<<20 || !utf8.Valid(data) {
		return "", nil, invalid
	}
	rows := strings.Split(strings.TrimRight(string(data), "\r\n"), "\n")
	if len(rows) > 4096 || rows[0] != "level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext" {
		return "", nil, invalid
	}
	var text strings.Builder
	words := []subtitleDraftWord{}
	previous := ""
	for _, row := range rows[1:] {
		fields := strings.SplitN(strings.TrimRight(row, "\r"), "\t", 12)
		if len(fields) != 12 {
			return "", nil, invalid
		}
		for index, field := range fields[:10] {
			number, parseErr := strconv.Atoi(field)
			if parseErr != nil || number < 0 || number > 1000000 || index == 0 && (number < 1 || number > 5) {
				return "", nil, invalid
			}
		}
		if fields[0] != "5" {
			continue
		}
		confidence, err := strconv.ParseFloat(fields[10], 64)
		word := strings.TrimSpace(fields[11])
		if err != nil || math.IsNaN(confidence) || math.IsInf(confidence, 0) || confidence < 0 || confidence > 100 || !validSubtitleDraftText(word) {
			return "", nil, invalid
		}
		if word == "" {
			continue
		}
		line := strings.Join(fields[1:5], ":")
		if text.Len() > 0 {
			if line == previous {
				text.WriteByte(' ')
			} else {
				text.WriteByte('\n')
			}
		}
		text.WriteString(word)
		previous = line
		if text.Len() > 4096 {
			return "", nil, invalid
		}
		words = append(words, subtitleDraftWord{Text: word, Confidence: confidence / 100})
	}
	return text.String(), words, nil
}
