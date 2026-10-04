// Command hls-readiness projects the exact shared readiness gates for synthetic E2E media.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/MikeO7/kinosail/packages/playback"
)

func main() {
	if len(os.Args) != 3 {
		os.Exit(2)
	}
	source, root := os.Args[1], os.Args[2]
	labels, qualities := []string{}, []playback.PlaybackQuality{}
	entries, _ := os.ReadDir(root)
	for _, entry := range entries {
		if entry.IsDir() && playback.QualityDirectory(entry.Name()) {
			labels = append(labels, entry.Name())
			qualities = append(qualities, playback.PlaybackQuality{Label: entry.Name()})
		}
	}
	variants := []map[string]any{}
	for _, label := range labels {
		directory := filepath.Join(root, label)
		manifest, err := playback.ReadHLSPlaylist(filepath.Join(directory, "index.m3u8"))
		variants = append(variants, map[string]any{
			"quality": label, "fresh": playback.Fresh(filepath.Join(directory, "index.m3u8"), source),
			"variantReady": playback.VariantReady(source, directory), "boundedRead": err == nil,
			"manifestBytes": len(manifest),
		})
	}
	_, masterErr := os.Stat(filepath.Join(root, "index.m3u8"))
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
		"sourceVersion": playback.SourceVersion(source), "variants": variants,
		"variantsReady": playback.VariantsReady(source, root, qualities), "masterPresent": masterErr == nil,
	})
}
