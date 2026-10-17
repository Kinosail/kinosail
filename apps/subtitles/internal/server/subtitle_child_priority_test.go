package server

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestSubtitleGenerationAndSpeechChildrenRunAtBackgroundPriority(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("process priority is observed through Linux procfs")
	}
	t.Parallel()
	log := filepath.Join(t.TempDir(), "priorities")
	record := "cut -d ' ' -f 19 /proc/self/stat >> '" + log + "'\n"
	transcript := `{"result":{"language":"en"},"transcription":[{"offsets":{"from":1000,"to":3000},"text":"Hello world","tokens":[{"text":" Hello","p":0.95,"t_dtw":110,"offsets":{"from":1000,"to":2000}}]}]}`
	wavFFmpeg := subtitleProcessFixture(t, record+"exit 0")
	whisper := subtitleProcessFixture(t, record+`while [ "$#" -gt 0 ]; do
if [ "$1" = --output-file ]; then shift; output="$1"; fi
shift
done
printf '%s' '`+transcript+`' > "$output.json"`)
	if _, _, err := localSubtitleTranscription(t.Context(), wavFFmpeg, whisper, "media", 0, "en", "model", "small", 10, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	frameFFmpeg := subtitleProcessFixture(t, record+"printf 'n: 0 pts: 0 pts_time:1\\n' >&2\ndd if=/dev/zero bs=2073600 count=1 2>/dev/null")
	tesseract := subtitleProcessFixture(t, record+"cat >/dev/null\nprintf 'level\\tpage_num\\tblock_num\\tpar_num\\tline_num\\tword_num\\tleft\\ttop\\twidth\\theight\\tconf\\ttext\\n5\\t1\\t1\\t1\\t1\\t1\\t0\\t0\\t20\\t10\\t95\\tHello\\n'")
	if _, _, err := localSubtitleOCR(t.Context(), frameFFmpeg, tesseract, "media", 0, "eng", 10); err != nil {
		t.Fatal(err)
	}
	speechFFmpeg := subtitleProcessFixture(t, record+"dd if=/dev/zero bs=1920000 count=1 2>/dev/null")
	if _, err := newSubtitleSynchronizer(speechFFmpeg).extractAudio(t.Context(), "film.mkv", "0:a:0"); err != nil {
		t.Fatal(err)
	}
	requireSubtitleBackgroundChildren(t, log, 5)
}

func requireSubtitleBackgroundChildren(t *testing.T, log string, want int) {
	t.Helper()
	stat, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		t.Fatal(err)
	}
	own, err := strconv.Atoi(strings.Fields(string(stat[bytes.LastIndexByte(stat, ')')+1:]))[16])
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	priorities := strings.Fields(string(data))
	if len(priorities) != want {
		t.Fatalf("child priorities = %q, want %d children", priorities, want)
	}
	for _, value := range priorities {
		if nice, err := strconv.Atoi(value); err != nil || nice < min(19, max(own, 10)) {
			t.Fatalf("background child nice = %q, server nice = %d", value, own)
		}
	}
}
