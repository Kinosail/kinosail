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
	if !copiedAACRecipeSupported(item, recipe) {
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

func copiedAACRecipeSupported(item library.Item, recipe hlsRecipe) bool {
	return recipe.mode == "remux" && recipe.audio == 0 && !recipe.dialogueBoost && !recipe.normalizeLoudness &&
		len(recipe.omitted) == 0 && runtime.GOOS == "linux" && strings.EqualFold(filepath.Ext(item.Path), ".mp4")
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
	var grid copiedAACSourceDescription
	arguments := []string{"-v", "error", "-threads", "1", "-show_streams", "-show_format", "-show_entries",
		"stream=index,codec_type,codec_name,profile,sample_rate,channels,time_base:format=format_name", "-of", "compact=p=0", source}
	err := copiedHLSLines(ctx, executable, arguments, 64<<10, 64, grid.add)
	if err != nil || grid.streams == 0 || grid.format == "" || grid.video == "" || ctx.Err() != nil {
		return false, 0, errCopiedHLSIndex
	}
	return grid.selected && grid.video == "h264" && strings.Contains(","+grid.format+",", ",mp4,"), grid.track, nil
}

type copiedAACSourceDescription struct {
	format, video         string
	audio, track, streams int
	selected              bool
}

func (grid *copiedAACSourceDescription) add(line string) error {
	fields := copiedHLSFields(line)
	if name, ok := fields["format_name"]; ok {
		if grid.format != "" || name == "" {
			return errCopiedHLSIndex
		}
		grid.format = name
		return nil
	}
	index, err := strconv.Atoi(fields["index"])
	if err != nil || !validCopiedAACSourceStream(fields, index) {
		return errCopiedHLSIndex
	}
	grid.streams++
	switch fields["codec_type"] {
	case "video":
		if grid.video == "" {
			grid.video = fields["codec_name"]
		}
	case "audio":
		if grid.audio == 0 {
			grid.track = index
			grid.selected = validCopiedAACStream(fields, index, -1)
		}
		grid.audio++
	}
	return nil
}

func validCopiedAACSourceStream(fields map[string]string, index int) bool {
	return index >= 0 && index <= 255 && fields["codec_type"] != "" && fields["codec_name"] != ""
}
