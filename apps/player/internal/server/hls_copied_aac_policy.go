package server

import (
	"os"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

const maximumCopiedAACPolicies = 4096

type copiedAACPolicyDecision struct {
	base     string
	source   os.FileInfo
	selected bool
	track    int
}

// A completed unsupported result is distinct from absent/failed qualification.
func (manager *hlsManager) copiedAACPolicy(key, base string, source os.FileInfo) (bool, int, bool) {
	manager.copiedMetadata.mu.Lock()
	value, found := manager.copiedMetadata.aacPolicies[key]
	manager.copiedMetadata.mu.Unlock()
	known := found && value.base == base && sameCopiedHLSFile(value.source, source)
	return value.selected && known, value.track, known
}

func (manager *hlsManager) recordCopiedAACPolicy(key, base string, source os.FileInfo, selected bool, track int) error {
	if key == "" || len(key) > 1024 || base == "" || len(base) > 4096 || source == nil || !source.Mode().IsRegular() ||
		source.Size() <= 0 || selected && (track < 0 || track > 255) {
		return errCopiedHLSIndex
	}
	metadata := &manager.copiedMetadata
	metadata.mu.Lock()
	defer metadata.mu.Unlock()
	previous, exists := metadata.aacPolicies[key]
	if exists && previous.base == base && sameCopiedHLSFile(previous.source, source) && previous.selected && !selected {
		return errCopiedHLSIndex
	}
	if !exists && len(metadata.aacPolicies) >= maximumCopiedAACPolicies {
		return errCopiedHLSIndex // Never erase a positive to make room.
	}
	if metadata.aacPolicies == nil {
		metadata.aacPolicies = make(map[string]copiedAACPolicyDecision)
	}
	metadata.aacPolicies[key] = copiedAACPolicyDecision{base: base, source: source, selected: selected, track: track}
	return nil
}

func (manager *hlsManager) copiedAACSettings(item library.Item, recipe hlsRecipe, options transcodeSettings) (transcodeSettings, error) {
	source, err := os.Lstat(item.Path)
	if err != nil {
		return transcodeSettings{}, err
	}
	key := hlsRecipeKey(item.ID, recipe)
	selected, _, known := manager.copiedAACPolicy(key, options.Cache, source)
	manager.copiedMetadata.mu.Lock()
	previous, found := manager.copiedMetadata.aacPolicies[key]
	manager.copiedMetadata.mu.Unlock()
	if found && previous.selected && !known {
		return transcodeSettings{}, errHLSIdentityChanged
	}
	if selected {
		if !found || !previous.selected || previous.base != options.Cache || !sameCopiedHLSFile(previous.source, source) {
			return transcodeSettings{}, errHLSIdentityChanged
		}
		token, err := copiedAACSourceToken(previous.source)
		if err != nil {
			return transcodeSettings{}, err
		}
		after, err := os.Lstat(item.Path)
		if err != nil || !sameCopiedHLSFile(previous.source, after) {
			return transcodeSettings{}, errHLSIdentityChanged
		}
		options.Cache += ":copied-source=" + token + ":copied-aac=2"
	}
	return options, nil
}

func copiedAACPolicyRequired(policy string) bool {
	return strings.HasSuffix(policy, ":copied-aac=2")
}

func copiedAACBasePolicy(policy string) string {
	if copiedAACPolicyRequired(policy) {
		if index := strings.LastIndex(policy, ":copied-source="); index >= 0 {
			return policy[:index]
		}
	}
	return policy
}
