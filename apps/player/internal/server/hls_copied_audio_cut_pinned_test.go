package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"
	"time"
)

// Isolated gap: public fixed-media E2E has no AAC packet exactly on a fractional
// seek boundary. These real pinned-BSF controls retain all copied payload hashes.
// They do not grant general audio time-base, cache, decoder or producer admission.
func TestCopiedHLSAACCutPinnedPacketBoundaries(t *testing.T) {
	if os.Getenv("KINOSAIL_PINNED_AAC_CUT") != "1" {
		t.Skip("hosted pinned FFmpeg boundary control")
	}
	encoder, probe := copiedHLSAACControlTools(t)
	directory := t.TempDir()
	source := filepath.Join(directory, "source.m4a")
	copiedHLSAACControlCommand(t, encoder, "-nostdin", "-v", "error", "-threads", "1",
		"-f", "lavfi", "-i", "sine=frequency=731:sample_rate=48000", "-t", "0.256",
		"-c:a", "aac", "-threads", "1", "-b:a", "128k", "-y", source)
	hashes := copiedHLSAACControlHashes(t, probe, source)
	if len(hashes) < 6 {
		t.Fatal("pinned AAC control packet bound")
	}
	copiedHLSAACControlInputRescale(t, encoder, probe, source, directory)
	cases := []copiedHLSAACControlCase{
		{"preserve_935_through_938", copiedHLSKey{PTS: 960000, DTS: 956000}, 48000, -4600, 1},
		{"fractional_seek_exact_dts", copiedHLSKey{PTS: 960001, DTS: 956001}, 48000, -5024, 1},
		{"fractional_dts_packet_before_key", copiedHLSKey{PTS: 256000, DTS: 255731}, 12800, -2033, 2},
		{"half_tick_packet_before_key", copiedHLSKey{PTS: 256000, DTS: 255730}, 12800, -2037, 2},
	}
	for _, selected := range cases {
		t.Run(selected.name, func(t *testing.T) {
			copiedHLSAACControlCaseOutput(t, selected, encoder, probe, source, directory, hashes)
		})
	}
	data, err := os.ReadFile(source)
	if err != nil || len(data) > 1<<20 {
		t.Fatal("pinned AAC control source bound")
	}
	t.Logf("syntheticControlSHA256=%x; sourcePackets=%d; no productionAcceptance", sha256.Sum256(data), len(hashes))
}

type copiedHLSAACControlCase struct {
	name        string
	key         copiedHLSKey
	denominator int64
	base        int64
	want        int
}

func copiedHLSAACControlTools(t *testing.T) (string, string) {
	t.Helper()
	encoder, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatal("pinned AAC control executable unavailable")
	}
	probe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Fatal("pinned AAC control probe unavailable")
	}
	return encoder, probe
}

func copiedHLSAACControlCaseOutput(t *testing.T, selected copiedHLSAACControlCase, encoder, probe, source, directory string, hashes []string) {
	t.Helper()
	timeline := copiedHLSAudioBoundaryTimeline(selected.key)
	timeline.Denominator = selected.denominator
	timeline.TimeBase = 1 / float64(selected.denominator)
	timeline.Keys[0] = copiedHLSKey{PTS: 12 * selected.denominator, DTS: 11 * selected.denominator}
	arguments, err := copiedHLSSeekArguments([]string{"-c:a", "copy"}, timeline, 1)
	if err != nil {
		t.Fatal("pinned AAC control rejected")
	}
	output := filepath.Join(directory, selected.name+".nut")
	copiedHLSAACControlOutput(t, encoder, source, output, selected.base, copiedHLSAudioOption(arguments, "-bsf:a"))
	actual := copiedHLSAACControlHashes(t, probe, output)
	if !reflect.DeepEqual(actual, hashes[selected.want:]) {
		t.Fatalf("complete copied AAC suffix differs: got %d want %d", len(actual), len(hashes)-selected.want)
	}
	t.Logf("all %d copied AAC payloads retained in order; sourceFirstOrdinal=%d", len(actual), selected.want)
	if selected.name == "fractional_seek_exact_dts" {
		broken := "noise=amount=0:drop=lt(pts*tb+20.000020\\,956001*1/48000)"
		negative := filepath.Join(directory, "negative.nut")
		copiedHLSAACControlOutput(t, encoder, source, negative, selected.base, broken)
		if !reflect.DeepEqual(copiedHLSAACControlHashes(t, probe, negative), hashes[2:]) {
			t.Fatal("fractional literal-seconds negative control stopped losing its exact-boundary packet")
		}
		t.Log("negative control loses the exact-boundary packet")
	}
}

func copiedHLSAACControlOutput(t *testing.T, executable, source, output string, base int64, bsf string) {
	t.Helper()
	// Timestamp engineering isolates BSF numeric semantics; no payload is reencoded.
	before := fmt.Sprintf("setts=pts=PTS-STARTPTS+(%d):dts=DTS-STARTDTS+(%d)", base, base)
	chain := before + "," + bsf + ",setts=pts=PTS+24000:dts=DTS+24000"
	copiedHLSAACControlCommand(t, executable, "-nostdin", "-v", "error", "-threads", "1", "-copyts",
		"-i", source, "-map", "0:a:0", "-copypriorss:a", "1", "-c:a", "copy", "-bsf:a", chain,
		"-avoid_negative_ts", "disabled", "-f", "nut", "-y", output)
}

func copiedHLSAACControlCommand(t *testing.T, executable string, arguments ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	var output bytes.Buffer
	err := copiedHLSLines(ctx, executable, arguments, 1<<20, 4096, func(line string) error {
		if output.Len()+len(line)+1 > 1<<20 {
			return errCopiedHLSIndex
		}
		output.WriteString(line)
		output.WriteByte('\n')
		return nil
	})
	if err != nil {
		t.Fatal("bounded joined pinned AAC control command failed")
	}
	return output.Bytes()
}

type copiedHLSAACControlPacket struct {
	PTS  *int64 `json:"pts"`
	Hash string `json:"data_hash"`
}

var copiedHLSAACControlPayloadHash = regexp.MustCompile(`^SHA256:[a-f0-9]{64}$`)

func copiedHLSAACControlPackets(t *testing.T, probe, path string) []copiedHLSAACControlPacket {
	t.Helper()
	output := copiedHLSAACControlCommand(t, probe, "-v", "error", "-threads", "1",
		"-select_streams", "a:0", "-show_packets", "-show_streams", "-show_data_hash", "sha256",
		"-show_entries", "packet=pts,data_hash:stream=time_base", "-of", "json", path)
	var data struct {
		Packets []copiedHLSAACControlPacket `json:"packets"`
		Streams []struct {
			TimeBase string `json:"time_base"`
		} `json:"streams"`
	}
	if json.Unmarshal(output, &data) != nil || len(data.Packets) == 0 || len(data.Packets) > 32 ||
		len(data.Streams) != 1 || data.Streams[0].TimeBase != "1/48000" {
		t.Fatal("bounded pinned AAC control packet clock shape")
	}
	for _, packet := range data.Packets {
		if packet.PTS == nil || !copiedHLSAACControlPayloadHash.MatchString(packet.Hash) {
			t.Fatal("pinned AAC control packet identity shape")
		}
	}
	return data.Packets
}

func copiedHLSAACControlHashes(t *testing.T, probe, path string) []string {
	t.Helper()
	packets := copiedHLSAACControlPackets(t, probe, path)
	result := make([]string, len(packets))
	for index, packet := range packets {
		result[index] = packet.Hash
	}
	return result
}

func copiedHLSAACControlInputRescale(t *testing.T, encoder, probe, source, directory string) {
	t.Helper()
	shifted := filepath.Join(directory, "shifted.nut")
	copiedHLSAACControlCommand(t, encoder, "-nostdin", "-v", "error", "-threads", "1", "-copyts",
		"-i", source, "-map", "0:a:0", "-copypriorss:a", "1", "-c:a", "copy", "-bsf:a",
		"setts=pts=PTS-STARTPTS+954977:dts=DTS-STARTDTS+954977",
		"-avoid_negative_ts", "disabled", "-f", "nut", "-y", shifted)
	before := copiedHLSAACControlPackets(t, probe, shifted)
	seeked := filepath.Join(directory, "seeked.nut")
	copiedHLSAACControlCommand(t, encoder, "-nostdin", "-v", "error", "-threads", "1",
		"-seek_timestamp", "1", "-ss", "20.000020", "-i", shifted, "-map", "0:a:0",
		"-copypriorss:a", "1", "-c:a", "copy", "-bsf:a", "setts=pts=PTS+24000:dts=DTS+24000",
		"-avoid_negative_ts", "disabled", "-f", "nut", "-y", seeked)
	after := copiedHLSAACControlPackets(t, probe, seeked)
	positions := make(map[string][]int64)
	for _, packet := range before {
		positions[packet.Hash] = append(positions[packet.Hash], *packet.PTS)
	}
	for _, packet := range after {
		matches := positions[packet.Hash]
		if len(matches) != 1 || matches[0]-*packet.PTS+24000 != 960001 {
			t.Fatal("actual fractional input seek did not rebase by 960001 ticks")
		}
	}
	t.Logf("actual seek 20.000020s rebases all %d uniquely copied packets by 960001 ticks at 1/48000", len(after))
}
