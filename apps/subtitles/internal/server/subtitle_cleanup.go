package server

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

type subtitleCleanupFile struct {
	Path     string
	Media    string
	Size     int64
	Modified int64
}

type subtitleCleanupPlan struct {
	Files   []subtitleCleanupFile
	Skipped int
	Digest  string
}

func planSubtitleCleanup(index *libraryIndex, language, forced string) (subtitleCleanupPlan, error) { //nolint:cyclop,gocognit // Selection, deduplication, and skipped-file accounting belong to one preview plan.
	canonical, err := validateSubtitleLanguages([]string{language})
	if err != nil || !oneOf(forced, "keep", "delete") {
		return subtitleCleanupPlan{}, errors.New("choose a supported language and forced subtitle option")
	}
	if index == nil || index.Index == nil {
		return subtitleCleanupPlan{}, errors.New("subtitle library is unavailable")
	}
	items, err := index.Snapshot()
	if err != nil {
		return subtitleCleanupPlan{}, err
	}
	plan := subtitleCleanupPlan{Files: make([]subtitleCleanupFile, 0)}
	seen := make(map[string]bool)
	for _, item := range items {
		if item.Kind != "video" {
			continue
		}
		for _, path := range item.Subtitles {
			if seen[path] {
				continue
			}
			seen[path] = true
			file, skipped := cleanupSidecarCandidate(index, item, path, canonical[0], forced)
			if skipped {
				plan.Skipped++
			} else if file != nil {
				plan.Files = append(plan.Files, *file)
			}
		}
	}
	slices.SortFunc(plan.Files, func(a, b subtitleCleanupFile) int { return strings.Compare(a.Path, b.Path) })
	plan.Digest = subtitleCleanupDigest(canonical[0], forced, plan.Files)
	return plan, nil
}

func cleanupSidecarCandidate(index *libraryIndex, item library.Item, path, language, forced string) (*subtitleCleanupFile, bool) {
	if !cleanupSidecarName(item.Path, path) {
		return nil, true
	}
	mediaBase := strings.TrimSuffix(item.Path, filepath.Ext(item.Path))
	_, tagged := subtitleTrackLanguage(path, mediaBase, nil)
	if tagged == "" {
		return nil, true
	}
	if subtitleLanguageMatches(language, tagged) && (forced == "keep" || subtitleRoleFromPath(path) != "forced") {
		return nil, false
	}
	root, name, err := openCleanupSidecar(index, item, path)
	if err != nil {
		return nil, true
	}
	info, err := root.Lstat(name)
	_ = root.Close()
	if err != nil || !info.Mode().IsRegular() {
		return nil, true
	}
	return &subtitleCleanupFile{Path: path, Media: item.Path, Size: info.Size(), Modified: info.ModTime().UnixNano()}, false
}

func cleanupSidecarName(media, subtitle string) bool {
	if !filepath.IsAbs(media) || !filepath.IsAbs(subtitle) || filepath.Dir(media) != filepath.Dir(subtitle) {
		return false
	}
	ext := strings.ToLower(filepath.Ext(subtitle))
	if ext != ".srt" && ext != ".vtt" {
		return false
	}
	base := strings.TrimSuffix(media, filepath.Ext(media))
	suffix := strings.TrimSuffix(subtitle, filepath.Ext(subtitle))
	return strings.HasPrefix(suffix, base+".")
}

func openCleanupSidecar(index *libraryIndex, item library.Item, path string) (*os.Root, string, error) {
	if !cleanupSidecarName(item.Path, path) {
		return nil, "", errors.New("subtitle path is invalid")
	}
	for _, allowed := range index.Roots() {
		relative, err := filepath.Rel(allowed.Path, filepath.Dir(path))
		if err != nil || !filepath.IsLocal(relative) {
			continue
		}
		root, err := os.OpenRoot(allowed.Path)
		if err != nil {
			continue
		}
		parent, err := root.OpenRoot(relative)
		_ = root.Close()
		if err == nil {
			return parent, filepath.Base(path), nil
		}
	}
	return nil, "", errors.New("subtitle path is outside the Library")
}

func subtitleCleanupDigest(language, forced string, files []subtitleCleanupFile) string {
	hash := sha256.New()
	_, _ = fmt.Fprintf(hash, "%q:%q\n", language, forced)
	for _, file := range files {
		_, _ = fmt.Fprintf(hash, "%q:%q:%d:%d\n", file.Media, file.Path, file.Size, file.Modified)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func applySubtitleCleanup(index *libraryIndex, settings *settingsStore, language, forced, digest string) (int, error) {
	if len(digest) != 64 {
		return 0, errors.New("subtitle cleanup preview has expired")
	}
	if settings == nil {
		return 0, errors.New("subtitle settings are unavailable")
	}
	plan, err := planSubtitleCleanup(index, language, forced)
	if err != nil {
		return 0, err
	}
	if plan.Digest != digest {
		return 0, errors.New("subtitle files changed; preview again")
	}
	if !slices.Equal(settings.subtitleLanguages(), []string{language}) {
		if err := settings.setSubtitleLanguages([]string{language}); err != nil {
			return 0, err
		}
	}
	removed := 0
	for _, file := range plan.Files {
		if err := removeCleanupSidecar(index, file); err != nil {
			return removed, err
		}
		removed++
	}
	if removed > 0 {
		index.RequestRefresh()
	}
	return removed, nil
}

func removeCleanupSidecar(index *libraryIndex, file subtitleCleanupFile) error {
	root, name, err := openCleanupSidecar(index, library.Item{Path: file.Media}, file.Path)
	if err != nil {
		return err
	}
	defer root.Close()
	info, err := root.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Size() != file.Size || info.ModTime().UnixNano() != file.Modified {
		return errors.New("subtitle files changed; preview again")
	}
	return root.Remove(name)
}
