package mediaprobe

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/privatefile"
	"github.com/MikeO7/kinosail/packages/workload"
	"golang.org/x/text/language"
)

const ocrLanguageDataLimit = 64 << 20

var (
	ocrLanguages sync.Map
	tessdataName = regexp.MustCompile(`^[a-z]{3}(?:_[a-z]{3,4})?$`)
	webVTTText   = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
)

func (probe *Probe) ocrLanguage(ctx context.Context, options ImageTextOptions) (string, string) {
	name := OCRLanguage(options.Language)
	if name == "" || !validFFmpegExecutable(options.Tesseract) {
		return "", ""
	}
	if slices.Contains(availableOCRLanguages(ctx, options.Tesseract), name) {
		return name, ""
	}
	if options.TessdataDir != "" && privatefile.Exists(filepath.Join(options.TessdataDir, name+".traineddata"), ocrLanguageDataLimit) {
		return name, options.TessdataDir
	}
	return "", ""
}

func OCRLanguage(code string) string {
	tag, err := language.Parse(strings.TrimSpace(code))
	if err != nil {
		return ""
	}
	script, _ := tag.Script()
	region, _ := tag.Region()
	name := ocrLanguageCode(code)
	switch {
	case name == "zho" && (script.String() == "Hant" || region.String() == "TW" || region.String() == "HK" || region.String() == "MO"):
		return "chi_tra"
	case name == "zho":
		return "chi_sim"
	case name == "nob" || name == "nno":
		return "nor"
	case name == "kur":
		return "kmr"
	case (name == "srp" && script.String() == "Latn") || (name == "aze" || name == "uzb") && script.String() == "Cyrl":
		return name + "_" + strings.ToLower(script.String())
	default:
		return name
	}
}

func InstalledOCRLanguages(ctx context.Context, tesseract string) []string {
	if !validFFmpegExecutable(tesseract) {
		return nil
	}
	return slices.Clone(availableOCRLanguages(ctx, tesseract))
}

func availableOCRLanguages(ctx context.Context, tesseract string) []string {
	if cached, found := ocrLanguages.Load(tesseract); found {
		return cached.([]string)
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	output := boundedOutput{remaining: 64 << 10}
	command := exec.CommandContext(ctx, tesseract, "--list-langs")
	command.Stdout = &output
	var languages []string
	if err := command.Run(); err == nil && !output.overflow {
		for _, line := range strings.Split(output.String(), "\n") {
			if line = strings.TrimSpace(line); tessdataName.MatchString(line) && line != "osd" {
				languages = append(languages, line)
			}
		}
	}
	if ctx.Err() != nil {
		return nil
	}
	if len(languages) == 0 {
		slog.Info("image subtitle text recognition is unavailable", "operation", "subtitles.ocr", "outcome", "no-languages")
	} else {
		slog.Info("image subtitle text recognition is available", "operation", "subtitles.ocr", "languages", len(languages))
	}
	actual, _ := ocrLanguages.LoadOrStore(tesseract, languages)
	return actual.([]string)
}

func recognizeImageSubtitle(ctx context.Context, tesseract, recognized, data, directory string) ([]byte, error) {
	cues, err := imageSubtitleCues(directory)
	if err != nil || len(cues) == 0 {
		return nil, errors.Join(errImageSubtitleUnreadable, err)
	}
	pages := make([]string, len(cues))
	for index, cue := range cues {
		pages[index] = cue.path
	}
	list := filepath.Join(directory, "pages.txt")
	if err = os.WriteFile(list, []byte(strings.Join(pages, "\n")+"\n"), 0o600); err != nil {
		return nil, err
	}
	output := boundedOutput{remaining: embeddedSubtitleLimit + 1}
	arguments := []string{list, "stdout", "-l", recognized, "--psm", "6"}
	if data != "" {
		arguments = append(arguments, "--tessdata-dir", data)
	}
	command := exec.CommandContext(ctx, tesseract, arguments...)
	command.Stdout = &output
	if err = workload.RunContext(ctx, command); err != nil || ctx.Err() != nil {
		return nil, errors.Join(ctx.Err(), err)
	}
	if output.overflow {
		return nil, errEmbeddedSubtitleOutput
	}
	texts := strings.Split(output.String(), "\f")
	if len(texts) == len(cues)+1 && strings.TrimSpace(texts[len(cues)]) == "" {
		texts = texts[:len(cues)]
	}
	if len(texts) != len(cues) {
		return nil, errors.New("image subtitle text recognition returned an unexpected page count")
	}
	return imageSubtitleWebVTT(cues, texts)
}

func imageSubtitleWebVTT(cues []imageCue, texts []string) ([]byte, error) {
	var document strings.Builder
	document.WriteString("WEBVTT\n")
	written := 0
	for index, cue := range cues {
		var lines []string
		for line := range strings.SplitSeq(texts[index], "\n") {
			line = strings.Join(strings.Fields(strings.ReplaceAll(line, "|", "I")), " ")
			if strings.IndexFunc(line, func(character rune) bool { return unicode.IsLetter(character) || unicode.IsDigit(character) }) >= 0 {
				lines = append(lines, webVTTText.Replace(line))
			}
		}
		if len(lines) == 0 {
			continue
		}
		timing := playback.FormatVTTTime(float64(cue.start)/1000) + " --> " + playback.FormatVTTTime(float64(cue.end)/1000)
		if cue.top {
			timing += " line:10%"
		}
		document.WriteString("\n" + timing + "\n" + strings.Join(lines, "\n") + "\n")
		written++
	}
	if written == 0 {
		return nil, errImageSubtitleUnreadable
	}
	return []byte(document.String()), nil
}
