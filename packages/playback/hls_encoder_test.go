package playback

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

func TestHLSSeekRequiresOriginalEncoder(t *testing.T) { //nolint:cyclop,gocognit // All rejected seeks must preserve the same original encoder identity.
	root := t.TempDir()
	options := transcodepolicy.Settings{Codec: "h264", Accelerator: "cuda", Encoder: "h264_nvenc", Device: "0"}
	if err := BindHLSEncoder(root, options, false); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".encoder")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := BindHLSEncoder(root, options, true); err != nil {
		t.Fatal(err)
	}
	for _, changed := range []transcodepolicy.Settings{
		{Codec: "h264", Accelerator: "none", Encoder: "libx264"},
		{Codec: "h264", Accelerator: "cuda", Encoder: "h264_nvenc", Device: "1"},
		{Codec: "hevc", Accelerator: "cuda", Encoder: "hevc_nvenc", Device: "0", OutputHDR: "hdr10"},
	} {
		if err := BindHLSEncoder(root, changed, true); err == nil {
			t.Fatalf("seek changed existing encoding: %#v", changed)
		}
		if data, err := os.ReadFile(path); err != nil || string(data) != string(original) {
			t.Fatal("rejected seek rewrote encoder identity")
		}
	}
	for _, data := range []string{"", "malformed", string(original) + ` {}`, strings.Repeat("x", 1025)} {
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil { //nolint:gosec // path is the fixed .encoder file inside t.TempDir, never the malformed test data.
			t.Fatal(err)
		}
		if err := BindHLSEncoder(root, options, true); err == nil {
			t.Fatal("seek accepted corrupt encoder metadata")
		}
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := BindHLSEncoder(root, options, true); err == nil {
		t.Fatal("seek accepted missing encoder metadata")
	}
}

func TestInvalidHLSEncoderIdentityHasNoSideEffects(t *testing.T) {
	for _, options := range []transcodepolicy.Settings{{}, {Encoder: strings.Repeat("x", 1025)}} {
		root := filepath.Join(t.TempDir(), "unused")
		if err := BindHLSEncoder(root, options, false); err == nil {
			t.Fatal("invalid encoder identity was accepted")
		}
		if _, err := os.Stat(root); !os.IsNotExist(err) {
			t.Fatalf("invalid identity created storage: %v", err)
		}
	}
}

func TestMasterCacheRequiresCompletePolicyIdentity(t *testing.T) {
	root := t.TempDir()
	source, master := filepath.Join(root, "source"), filepath.Join(root, "index.m3u8")
	if err := os.WriteFile(source, []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	policy := "automatic:h264:auto:policy=2:source:recipe:hls=11"
	if err := os.WriteFile(master, []byte("#EXTM3U\n#KINOSAIL-TRANSCODER:"+policy+"\n#EXT-X-STREAM-INF:BANDWIDTH=1\n360p/index.m3u8\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !MasterFresh(master, source, policy) || MasterFresh(master, source, "automatic:h264:auto") {
		t.Fatal("cache matching accepted a partial policy")
	}
}

func TestSourceCPUFiltersDisableDecodeRetries(t *testing.T) {
	for _, recipe := range []HLSRecipe{{Mode: "transcode", Burn: "text"}, {Mode: "transcode", Omitted: []Range{{Start: 1, End: 2}}}} {
		options := SourceTranscoding(transcodepolicy.Settings{Codec: "h264", Accelerator: "cuda", HardwareDecode: true}, MediaFacts{Video: VideoFacts{Codec: "h264", BitDepth: 8}}, recipe)
		if options.HardwareDecode || options.Accelerator != "cuda" {
			t.Fatalf("CPU filters advertised hardware decode: %#v", options)
		}
	}
}
