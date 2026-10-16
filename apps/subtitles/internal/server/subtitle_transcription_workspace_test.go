package server

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestLocalSubtitleTranscriptionUsesCacheWorkspaceAndCoreThreads(t *testing.T) {
	t.Parallel()
	cache, evidence := t.TempDir(), t.TempDir()
	ffmpegLog, whisperLog := filepath.Join(evidence, "ffmpeg"), filepath.Join(evidence, "whisper")
	ffmpeg := subtitleProcessFixture(t, `for argument in "$@"; do last="$argument"; done
printf '%s' "$last" > '`+ffmpegLog+`'
: > "$last"`)
	transcript := `{"result":{"language":"en"},"transcription":[{"offsets":{"from":1000,"to":3000},"text":"Hello","tokens":[{"text":" Hello","p":0.95,"t_dtw":110,"offsets":{"from":1000,"to":3000}}]}]}`
	whisper := subtitleProcessFixture(t, `printf '%s ' "$@" > '`+whisperLog+`'
while [ "$#" -gt 0 ]; do
if [ "$1" = --output-file ]; then shift; output="$1"; fi
shift
done
printf '%s' '`+transcript+`' > "$output.json"`)
	if _, _, err := localSubtitleTranscription(t.Context(), ffmpeg, whisper, "media", 0, "en", "model", "small", 10, cache); err != nil {
		t.Fatal(err)
	}
	wav, err := os.ReadFile(ffmpegLog)
	if err != nil || !strings.HasPrefix(string(wav), cache+string(filepath.Separator)) {
		t.Fatalf("audio workspace = %q, %v; want inside %s", wav, err, cache)
	}
	if remaining, err := os.ReadDir(cache); err != nil || len(remaining) != 0 {
		t.Fatalf("audio workspace remains after transcription: %v, %v", remaining, err)
	}
	arguments, err := os.ReadFile(whisperLog)
	threads := strconv.Itoa(min(runtime.GOMAXPROCS(0), 8))
	if err != nil || !strings.Contains(string(arguments), "--threads "+threads+" ") {
		t.Fatalf("whisper arguments = %q, %v; want --threads %s", arguments, err, threads)
	}
}
