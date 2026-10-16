package server

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var referenceSubtitleTime = regexp.MustCompile(`^(?:(\d{1,3}):)?(\d{2}):(\d{2})[,.](\d{3})$`)

func referenceParseSubtitleTime(value string) (time.Duration, bool) {
	match := referenceSubtitleTime.FindStringSubmatch(value)
	if match == nil {
		return 0, false
	}
	hours, _ := strconv.Atoi(match[1])
	minutes, _ := strconv.Atoi(match[2])
	seconds, _ := strconv.Atoi(match[3])
	millis, _ := strconv.Atoi(match[4])
	if minutes > 59 || seconds > 59 {
		return 0, false
	}
	return time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second + time.Duration(millis)*time.Millisecond, true
}

func referenceCleanSubtitleLine(line string) string {
	line = subtitleVoice.ReplaceAllString(line, "$1: ")
	line = subtitleASSTags.ReplaceAllStringFunc(line, func(block string) string {
		var style strings.Builder
		for _, match := range subtitleASSStyle.FindAllStringSubmatch(block, -1) {
			if match[2] == "1" {
				style.WriteString("<" + match[1] + ">")
			} else {
				style.WriteString("</" + match[1] + ">")
			}
		}
		return style.String()
	})
	line = subtitleTags.ReplaceAllStringFunc(line, func(tag string) string {
		tag = strings.ToLower(tag)
		if oneOf(tag, "<i>", "</i>", "<b>", "</b>", "<u>", "</u>") {
			return tag
		}
		return ""
	})
	line = html.UnescapeString(line)
	line = strings.Map(func(character rune) rune {
		if unicode.IsControl(character) && character != '\t' {
			return -1
		}
		return character
	}, line)
	return norm.NFC.String(strings.Join(strings.Fields(line), " "))
}

var subtitleParseSamples = []string{
	"", "00:00:01,000", "00:00:01.000", "1:00:00,000", "12:34:56,789", "123:59:59,999", "1234:00:00,000", ":00:00,000",
	"00:60:00,000", "00:00:60,000", "00:59:59,999", "0:00:01,000", "00:0:01,000", "00:00:1,000", "00:00:01,00", "00:00:01,0000",
	"00:00:01;000", "00:00:01,000 ", " 00:00:01,000", "00:00:01,00a", "aa:00:01,000", "00:00:01", "00:01,000", "1:01,000",
	"001:00:00,000", "٠٠:٠٠:٠١,٠٠٠", "00:00:01,000\n", "-1:00:00,000", "+1:00:00,000", "1::00:00,000",
	"Hello there", "  spaced\tline  ", "<i>Italic</i> text", "<v Narrator>Story</v>", "{\\i1}ASS{\\i0} line", "Tom &amp; Jerry",
	"Café", "plain\x07bell", "a & b", "{not ass}", "<font color=\"red\">Red</font>", "<<>>", "&#39;quoted&#39;",
	" nbsp ", "​zero width", "MIXED <B>bold</B> & {\\b1}ass{\\b0}",
}

func TestSubtitleTimestampParserMatchesRegularExpression(t *testing.T) {
	t.Parallel()
	for _, sample := range subtitleParseSamples {
		got, gotOK := parseSubtitleTime(sample)
		want, wantOK := referenceParseSubtitleTime(sample)
		if got != want || gotOK != wantOK {
			t.Fatalf("parseSubtitleTime(%q) = %v, %v; want %v, %v", sample, got, gotOK, want, wantOK)
		}
	}
}

func TestSubtitleLineCleanerMatchesRegularExpressionPasses(t *testing.T) {
	t.Parallel()
	for _, sample := range subtitleParseSamples {
		if got, want := cleanSubtitleLine(sample), referenceCleanSubtitleLine(sample); got != want {
			t.Fatalf("cleanSubtitleLine(%q) = %q; want %q", sample, got, want)
		}
	}
}

func FuzzSubtitleParsingMatchesRegularExpressions(f *testing.F) {
	for _, sample := range subtitleParseSamples {
		f.Add(sample)
	}
	f.Fuzz(func(t *testing.T, sample string) {
		got, gotOK := parseSubtitleTime(sample)
		want, wantOK := referenceParseSubtitleTime(sample)
		if got != want || gotOK != wantOK {
			t.Fatalf("parseSubtitleTime(%q) = %v, %v; want %v, %v", sample, got, gotOK, want, wantOK)
		}
		if got, want := cleanSubtitleLine(sample), referenceCleanSubtitleLine(sample); got != want {
			t.Fatalf("cleanSubtitleLine(%q) = %q; want %q", sample, got, want)
		}
	})
}

func BenchmarkCleanSubtitleFeatureLength(b *testing.B) {
	var document strings.Builder
	for cue := range 1500 {
		start := time.Duration(cue) * 3 * time.Second
		fmt.Fprintf(&document, "%d\n%s --> %s\nLine number %d of the feature.\nSecond line, still plain.\n\n", cue+1, subtitleBenchmarkTime(start), subtitleBenchmarkTime(start+2*time.Second), cue)
	}
	data := []byte(document.String())
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := cleanSubtitle(data); err != nil {
			b.Fatal(err)
		}
	}
}

func subtitleBenchmarkTime(value time.Duration) string {
	return fmt.Sprintf("%02d:%02d:%02d,%03d", int(value.Hours()), int(value.Minutes())%60, int(value.Seconds())%60, value.Milliseconds()%1000)
}
