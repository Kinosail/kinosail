package server

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestLocalSubtitleOCRRecognizesDistinctCropsOnceInParallel(t *testing.T) {
	t.Parallel()
	evidence := t.TempDir()
	ffmpeg := subtitleProcessFixture(t, `printf 'n: 0 pts: 0 pts_time:1\nn: 1 pts: 1 pts_time:2\nn: 2 pts: 2 pts_time:3\nn: 3 pts: 3 pts_time:4\n' >&2
for repeat in 1 2; do
dd if=/dev/zero bs=2073600 count=1 2>/dev/null
dd if=/dev/zero bs=1036800 count=1 2>/dev/null
dd if=/dev/zero bs=1036800 count=1 2>/dev/null | tr '\000' '\377'
done`)
	tesseract := subtitleProcessFixture(t, `size=$(wc -c)
marker='`+evidence+`'/running.$$
: > "$marker"
for attempt in $(seq 1 60); do
if [ "$(ls '`+evidence+`' | grep -c '^running')" -ge 2 ]; then break; fi
sleep 0.05
done
printf '%s %s\n' "$OMP_THREAD_LIMIT" "$(ls '`+evidence+`' | grep -c '^running')" >> '`+evidence+`'/calls
rm "$marker"
word=Hello
if [ "$size" -lt 2000000 ]; then word=World; fi
printf 'level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext\n5\t1\t1\t1\t1\t1\t0\t0\t20\t10\t95\t%s\n' "$word"`)
	document, _, err := localSubtitleOCR(t.Context(), ffmpeg, tesseract, "media", 0, "eng", 10)
	if err != nil || len(document.Cues) != 4 {
		t.Fatalf("OCR = %#v, %v", document.Cues, err)
	}
	for position, want := range []string{"Hello", "World", "Hello", "World"} {
		if document.Cues[position].Text != want || document.Cues[position].Start != time.Duration(position+1)*time.Second {
			t.Fatalf("cue %d = %#v; want %q at %ds", position, document.Cues[position], want, position+1)
		}
	}
	calls, err := os.ReadFile(filepath.Join(evidence, "calls"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(calls)), "\n")
	if len(lines) != 2 {
		t.Fatalf("tesseract ran %d times for two distinct crops: %q", len(lines), calls)
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, "1 ") {
			t.Fatalf("tesseract thread limit = %q; want OMP_THREAD_LIMIT=1", line)
		}
	}
	if runtime.GOMAXPROCS(0) > 1 && !strings.Contains(string(calls), " 2") {
		t.Fatalf("distinct crops were recognized one at a time: %q", calls)
	}
}
