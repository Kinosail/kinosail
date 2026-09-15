package transcodepolicy

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest/mp4fixture"
)

func TestCheckRunsBoundedSyntheticWorkflow(t *testing.T) {
	ffmpeg, err := exec.LookPath("true")
	if err != nil {
		t.Fatal(err)
	}
	result := Check(t.Context(), ffmpeg, Settings{Codec: "h264", Accelerator: "none", Encoder: "libx264", Preset: "veryfast", CRF: "22"}, "Software")
	if result.Status != "failed" || result.Stage != "output" || result.Backend != "Software (libx264)" {
		t.Fatalf("check = %#v", result)
	}
}

func TestPendingCheckProjectsStableBackend(t *testing.T) {
	software := PendingCheck(Settings{Codec: "h264", Accelerator: "none", Encoder: "libx264"}, "Software")
	want := CheckResult{Status: "not-run", Codec: "h264", CodecName: "AVC / H.264", Accelerator: "none", Backend: "Software (libx264)"}
	if !reflect.DeepEqual(software, want) {
		t.Fatalf("software pending check = %#v", software)
	}
	hardware := PendingCheck(Settings{Codec: "hevc", Accelerator: "qsv"}, "Intel")
	if hardware.Backend != "Intel" || hardware.CodecName != "HEVC / H.265" {
		t.Fatalf("hardware pending check = %#v", hardware)
	}
}

func TestRunCheckPreservesSetupSourceEncodeAndSuccessResults(t *testing.T) { //nolint:cyclop,gocognit // One workflow table verifies every stable stage and message.
	options := Settings{Codec: "h264", Accelerator: "qsv", Encoder: "h264_qsv", Preset: "veryfast", CRF: "22"}
	original := options
	setup := runCheck(t.Context(), "ffmpeg", options, "Intel", func() (string, error) { return "", errors.New("no temporary storage") }, func(context.Context, string, ...string) error { return nil })
	if setup.Status != "failed" || setup.Stage != "setup" || setup.Message != "Kinosail could not prepare the local transcoder check." {
		t.Fatalf("setup failure = %#v", setup)
	}
	for _, test := range []struct {
		name, stage, message string
		failCall             int
	}{
		{"source", "source", "FFmpeg could not create the local test clip. Check that FFmpeg includes H.264 and AAC.", 1},
		{"encode", "encode", "Intel could not encode the local test clip. Choose Automatic or Software, save, and test again. If you selected hardware, make sure its GPU device is available to the Server container.", 2},
		{"passed", "", "Kinosail encoded and decoded a local HLS clip with Intel. This is a smoke check, not a device or performance certification.", 0},
	} {
		directory := filepath.Join(t.TempDir(), "check")
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
		calls := 0
		var arguments [][]string
		result := runCheck(t.Context(), "ffmpeg", options, "Intel", func() (string, error) { return directory, nil }, func(_ context.Context, command string, values ...string) error {
			calls++
			if command != "ffmpeg" {
				t.Fatalf("command = %q", command)
			}
			arguments = append(arguments, append([]string(nil), values...))
			if calls == test.failCall {
				return errors.New("failed")
			}
			if calls == 2 {
				return os.WriteFile(filepath.Join(directory, "init.mp4"), mp4fixture.Initialization(320, 180, "h264", "aac", ""), 0o600)
			}
			return nil
		})
		wantStatus := "failed"
		if test.name == "passed" {
			wantStatus = "passed"
		}
		if result.Status != wantStatus || result.Stage != test.stage || result.Message != test.message {
			t.Errorf("%s result = %#v", test.name, result)
		}
		if _, err := os.Stat(directory); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s temporary directory remained: %v", test.name, err)
		}
		if len(arguments) == 0 || !strings.Contains(strings.Join(arguments[0], " "), "testsrc2=size=640x360:rate=24:duration=1") {
			t.Errorf("%s source arguments = %v", test.name, arguments)
		}
		if test.failCall != 1 && (len(arguments) < 2 || !strings.Contains(strings.Join(arguments[1], " "), "-c:v h264_qsv") || !strings.Contains(strings.Join(arguments[1], " "), "-c:a aac")) {
			t.Errorf("%s encode arguments = %v", test.name, arguments)
		}
	}
	if options != original {
		t.Fatalf("synthetic check changed selection: %#v", options)
	}
}
