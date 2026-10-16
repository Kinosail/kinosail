package catalog

import (
	"path/filepath"
	"strings"
)

type allowedRoots struct {
	resolved []string
	complete bool
}

func SafePath(path string, allowed []string) bool {
	return resolveAllowed(allowed).contains(path)
}

func resolveAllowed(candidates []string) *allowedRoots {
	allowed := &allowedRoots{resolved: make([]string, 0, len(candidates)), complete: true}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		root, err := filepath.EvalSymlinks(candidate)
		if err != nil {
			allowed.complete = false
			continue
		}
		allowed.resolved = append(allowed.resolved, root)
	}
	return allowed
}

func (allowed *allowedRoots) contains(path string) bool {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	for _, root := range allowed.resolved {
		relative, err := filepath.Rel(root, resolved)
		if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func (index *Index) configuredRoots() []string {
	index.mu.RLock()
	defer index.mu.RUnlock()
	candidates := make([]string, 1, len(index.roots)+1)
	candidates[0] = index.cache
	for _, root := range index.roots {
		candidates = append(candidates, root.Path)
	}
	return candidates
}

func (index *Index) resolveRoots() {
	index.resolveMu.Lock()
	defer index.resolveMu.Unlock()
	index.allowed.Store(resolveAllowed(index.configuredRoots()))
}

func (index *Index) Safe(path string) bool {
	allowed := index.allowed.Load()
	if allowed == nil || !allowed.complete {
		allowed = resolveAllowed(index.configuredRoots())
	}
	return allowed.contains(path)
}
