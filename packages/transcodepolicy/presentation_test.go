package transcodepolicy

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestSoftwareLadderToneMapsOnceAtTheLargestRung(t *testing.T) {
	options := Settings{Codec: "h264", Accelerator: "none", Encoder: "libx264", ToneMap: true, Deinterlace: true, CRF: "22", Preset: "veryfast"}
	graph := PresentationFilterGraph(options, []string{"960", "1280", "1920"})
	for _, expected := range []string{"[0:v:0]bwdif=", "split=3[t0][t1][t2]", "[t0]scale=w=960:", "[t1]scale=w=1280:", "[t2]null", "[v0];", "[v1];", "[v2]", "sidedata=mode=delete:type=DOVI_METADATA"} {
		if !strings.Contains(graph, expected) {
			t.Fatalf("graph lacks %q: %s", expected, graph)
		}
	}
	if strings.Count(graph, "tonemap=hable") != 1 || strings.Count(graph, "bwdif=") != 1 {
		t.Fatalf("expensive filters repeat per rung: %s", graph)
	}
	if strings.Index(graph, "scale=w=1920") > strings.Index(graph, "zscale=t=linear") {
		t.Fatalf("tone mapping runs before the largest rung is scaled: %s", graph)
	}
	for _, options := range []Settings{
		{Codec: "h264", Accelerator: "none", Encoder: "libx264"},
		{Codec: "h264", Accelerator: "vaapi", Encoder: "h264_vaapi", ToneMap: true},
		{Codec: "hevc", Accelerator: "none", Encoder: "libx265", OutputHDR: "hdr10"},
	} {
		if graph := PresentationFilterGraph(options, []string{"1280", "1920"}); graph != "" {
			t.Errorf("%+v received a shared graph: %s", options, graph)
		}
	}
}

func TestInstalledFFmpegAcceptsSoftwareRecipes(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg is required to verify encoder arguments")
	}
	for _, encoder := range []struct{ codec, name string }{{"vp9", "libvpx-vp9"}, {"av1", "libaom-av1"}, {"av1", "libsvtav1"}, {"h264", "libx264"}} {
		t.Run(encoder.name, func(t *testing.T) {
			if output, _ := exec.CommandContext(t.Context(), ffmpeg, "-hide_banner", "-encoders").Output(); !HasCapability(string(output), encoder.name) {
				t.Skip("FFmpeg lacks " + encoder.name)
			}
			result := Check(t.Context(), ffmpeg, Settings{Codec: encoder.codec, Accelerator: "none", Encoder: encoder.name, Preset: "veryfast", CRF: "30"}, "Software")
			if result.Status != "passed" {
				t.Fatalf("check = %#v", result)
			}
		})
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("FFprobe is required to verify tone-mapped renditions")
	}
	if help, _ := exec.CommandContext(t.Context(), ffmpeg, "-hide_banner", "-h", "filter=sidedata").Output(); !strings.Contains(string(help), "DOVI_METADATA") {
		t.Skip("FFmpeg lacks the Dolby Vision side data types that the production recipe removes")
	}
	directory := t.TempDir()
	source := filepath.Join(directory, "hdr.mkv")
	create := exec.CommandContext(t.Context(), ffmpeg, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=1280x720:rate=24:duration=1", "-vf", "format=yuv420p10le,zscale=pin=bt709:tin=bt709:min=bt709:rin=limited:p=bt2020:t=smpte2084:m=bt2020nc:r=limited,setfield=tff", "-c:v", "ffv1", "-pix_fmt", "yuv420p10le", "-color_primaries", "bt2020", "-color_trc", "smpte2084", "-colorspace", "bt2020nc", source)
	if output, err := create.CombinedOutput(); err != nil {
		t.Fatalf("HDR source: %v %s", err, output)
	}
	options := Settings{Codec: "h264", Accelerator: "none", Encoder: "libx264", ToneMap: true, Deinterlace: true, CRF: "30", Preset: "ultrafast"}
	widths := []string{"640", "960", "1280"}
	arguments := []string{"-hide_banner", "-loglevel", "error", "-filter_complex_threads", "2", "-i", source, "-filter_complex", PresentationFilterGraph(options, widths)}
	for index := range widths {
		_, video := VideoArguments(options, widths[index])
		arguments = append(arguments, "-map", "[v"+strconv.Itoa(index)+"]")
		arguments = append(arguments, video[2:]...)
		arguments = append(arguments, filepath.Join(directory, widths[index]+".mp4"))
	}
	if output, err := exec.CommandContext(t.Context(), ffmpeg, arguments...).CombinedOutput(); err != nil {
		t.Fatalf("FFmpeg rejected the shared tone-map graph: %v %s", err, output)
	}
	for _, width := range widths {
		probe, err := exec.CommandContext(t.Context(), ffprobe, "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,pix_fmt,color_transfer", "-of", "csv=p=0", filepath.Join(directory, width+".mp4")).Output()
		if err != nil {
			t.Fatal(err)
		}
		if fields := strings.Split(strings.TrimSpace(string(probe)), ","); len(fields) < 3 || fields[0] != width || fields[1] != "yuv420p" || fields[2] != "bt709" {
			t.Fatalf("rendition %s = %q", width, probe)
		}
	}
}
