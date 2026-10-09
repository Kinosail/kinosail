package server

import (
	"context"
	"os"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

// Collection binds two measured source clocks to the supplied private packet.
// It cannot grant generated-asset, cache, or public-readiness admission.
func (manager *hlsManager) measureCopiedHLSSourceAudio(parent context.Context, item library.Item, recipe hlsRecipe, policy string, first [32]byte, micros, original int64) (*copiedHLSAudioProof, error) {
	ctx, release, err := manager.copiedHLSClockAdmission(parent)
	if err != nil {
		return nil, errCopiedHLSIndex
	}
	defer release()
	if !validCopiedHLSSourceAudioRequest(ctx, first, micros, original) ||
		recipe.mode != "remux" || recipe.audio != 0 || recipe.offset != float64(micros)/1_000_000 ||
		len(recipe.omitted) != 0 || manager.probe == nil || manager.probe.executable == "" || manager.ffmpeg == "" {
		return nil, errCopiedHLSIndex
	}
	before, err := manager.copiedHLSSourceAudioIdentity(ctx, item, recipe, policy, nil)
	if err != nil {
		return nil, err
	}
	native, err := copiedHLSSourceAudioOutput(ctx, manager.probe.executable, copiedHLSSourceAudioProbeArguments(item.Path), 24_000)
	if err != nil {
		return nil, err
	}
	if _, err = manager.copiedHLSSourceAudioIdentity(ctx, item, recipe, policy, before); err != nil {
		return nil, err
	}
	normalized, err := copiedHLSSourceAudioOutput(ctx, manager.ffmpeg, copiedHLSSourceAudioNormalizeArguments(item.Path), 1040)
	if err != nil {
		return nil, err
	}
	proof, err := deriveCopiedHLSSourceAudio(ctx, native, normalized, first, micros, original)
	if err != nil {
		return nil, err
	}
	if _, err = manager.copiedHLSSourceAudioIdentity(ctx, item, recipe, policy, before); err != nil {
		return nil, err
	}
	return proof, nil
}

func (manager *hlsManager) copiedHLSSourceAudioIdentity(ctx context.Context, item library.Item, recipe hlsRecipe, policy string, before os.FileInfo) (os.FileInfo, error) {
	if ctx.Err() != nil || manager.index == nil || !manager.index.Safe(item.Path) ||
		manager.validateHLSPolicy(ctx, item, recipe, policy) != nil {
		return nil, errCopiedHLSIndex
	}
	current, err := os.Lstat(item.Path)
	if err != nil || !current.Mode().IsRegular() || current.Size() <= 0 ||
		(before != nil && !sameCopiedHLSFile(before, current)) || ctx.Err() != nil {
		return nil, errCopiedHLSIndex
	}
	return current, nil
}

func copiedHLSSourceAudioOutput(ctx context.Context, executable string, arguments []string, maximumLines int) ([]byte, error) {
	var output strings.Builder
	err := copiedHLSLines(ctx, executable, arguments, 2<<20, maximumLines, func(line string) error {
		_, _ = output.WriteString(line)
		return output.WriteByte('\n')
	})
	if err != nil || ctx.Err() != nil {
		return nil, errCopiedHLSIndex
	}
	return []byte(output.String()), nil
}

func copiedHLSSourceAudioProbeArguments(source string) []string {
	return []string{
		"-v", "error", "-threads", "1", "-select_streams", "a:0", "-read_intervals", "%+22",
		"-show_packets", "-show_frames", "-show_streams", "-show_data_hash", "sha256",
		"-show_entries", "packet=pts,dts,duration,data_hash,side_data_list:frame=pts,nb_samples,side_data_list:stream=codec_name,profile,sample_rate,channels,time_base",
		"-of", "json", source,
	}
}

func copiedHLSSourceAudioNormalizeArguments(source string) []string {
	return []string{
		"-nostdin", "-v", "error", "-xerror", "-threads", "1", "-filter_threads", "1", "-i", source,
		"-map", "0:a:0", "-vn", "-sn", "-dn", "-frames:a", "1024",
		"-c:a", "pcm_s16le", "-threads:a", "1", "-f", "framemd5", "pipe:1",
	}
}
