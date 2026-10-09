package server

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

func (manager *hlsManager) qualifyCopiedAAC(parent context.Context, item library.Item, recipe hlsRecipe, force bool) error {
	if recipe.mode != "remux" || recipe.audio != 0 || recipe.dialogueBoost || recipe.normalizeLoudness || len(recipe.omitted) != 0 || runtime.GOOS != "linux" || !strings.EqualFold(filepath.Ext(item.Path), ".mp4") {
		return nil
	}
	if parent.Err() != nil || manager.index == nil || !manager.index.Safe(item.Path) {
		return errCopiedHLSIndex
	}
	base, err := manager.baseHLSSettings(item, recipe)
	if err != nil {
		return err
	}
	before, err := os.Lstat(item.Path)
	if err != nil || !before.Mode().IsRegular() || before.Size() <= 0 {
		return errCopiedHLSIndex
	}
	key := hlsRecipeKey(item.ID, recipe)
	if _, _, known := manager.copiedAACPolicy(key, base.Cache, before); known {
		return parent.Err()
	}
	directory := filepath.Join(manager.cache, key)
	if !force && !manager.copiedHLSTimelinePresent(directory) && !manager.copiedAACMarkerPresent(directory) {
		return nil // An ordinary unindexed cold generation keeps its existing producer.
	}
	ctx, release, err := manager.copiedHLSClockAdmission(parent)
	if err != nil {
		return err
	}
	defer release()
	if manager.index == nil || !manager.index.Safe(item.Path) || ctx.Err() != nil {
		return errCopiedHLSIndex
	}
	file, source, err := openCopiedHLSSourceAudio(ctx, item.Path, before)
	if err != nil {
		return err
	}
	defer file.Close()
	selected, track, err := copiedAACSourceGrid(ctx, manager.probe.executable, source)
	after, statErr := os.Lstat(item.Path)
	opened, openErr := file.Stat()
	current, policyErr := manager.baseHLSSettings(item, recipe)
	if err != nil || statErr != nil || openErr != nil || policyErr != nil || current.Cache != base.Cache ||
		!sameCopiedHLSFile(before, after) || !sameCopiedHLSFile(before, opened) || !manager.index.Safe(item.Path) || ctx.Err() != nil {
		return errCopiedHLSIndex
	}
	return manager.recordCopiedAACPolicy(key, base.Cache, before, selected, track)
}

func (manager *hlsManager) copiedAACMarkerPresent(directory string) bool {
	root, err := manager.openCopiedHLSRoot(directory)
	if err != nil {
		return false
	}
	defer root.Close()
	data, err := copiedHLSCacheFile(root, ".source", 16<<10)
	return err == nil && copiedAACPolicyRequired(string(data))
}

func copiedAACSourceGrid(ctx context.Context, executable, source string) (bool, int, error) {
	format, video, audio := "", "", 0
	selected, track, streamCount := false, 0, 0
	arguments := []string{"-v", "error", "-threads", "1", "-show_streams", "-show_format", "-show_entries",
		"stream=index,codec_type,codec_name,profile,sample_rate,channels,time_base:format=format_name", "-of", "compact=p=0", source}
	err := copiedHLSLines(ctx, executable, arguments, 64<<10, 64, func(line string) error {
		fields := copiedHLSFields(line)
		if name, ok := fields["format_name"]; ok {
			if format != "" || name == "" {
				return errCopiedHLSIndex
			}
			format = name
			return nil
		}
		index, err := strconv.Atoi(fields["index"])
		if err != nil || index < 0 || index > 255 || fields["codec_type"] == "" || fields["codec_name"] == "" {
			return errCopiedHLSIndex
		}
		streamCount++
		switch fields["codec_type"] {
		case "video":
			if video == "" {
				video = fields["codec_name"]
			}
		case "audio":
			if audio == 0 {
				track = index
				selected = fields["codec_name"] == "aac" && fields["profile"] == "LC" && fields["sample_rate"] == "48000" &&
					fields["channels"] == "2" && fields["time_base"] == "1/48000"
			}
			audio++
		}
		return nil
	})
	if err != nil || streamCount == 0 || format == "" || video == "" || ctx.Err() != nil {
		return false, 0, errCopiedHLSIndex
	}
	return selected && video == "h264" && strings.Contains(","+format+",", ",mp4,"), track, nil
}
