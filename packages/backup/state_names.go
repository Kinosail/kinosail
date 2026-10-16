package backup

import (
	"path/filepath"
	"slices"
	"strings"
)

const (
	maxStateDocuments = 256
	auditJournal      = "audit.jsonl"
)

var (
	stateFiles  = []string{auditJournal, "configuration.json", "server-identity.json"}
	secretFiles = []string{"audit_key.json", "secrets.json", "viewing_imports.json"}
)

type stateNames struct {
	documents, entries, plain, recovery []string
}

func newStateNames(documents, entries []string) (stateNames, bool) {
	if len(documents) == 0 || len(documents) > maxStateDocuments || len(entries) > len(documents) {
		return stateNames{}, false
	}
	reserved := slices.Concat(stateFiles, secretFiles, []string{manifestName})
	names := stateNames{documents: sortedUnique(documents), entries: sortedUnique(entries)}
	if len(names.documents) != len(documents) || len(names.entries) != len(entries) {
		return stateNames{}, false
	}
	for _, name := range names.documents {
		if !validDocumentName(name) || slices.Contains(reserved, name) {
			return stateNames{}, false
		}
	}
	for _, name := range names.entries {
		if !slices.Contains(names.documents, name) {
			return stateNames{}, false
		}
	}
	names.plain = sortedUnique(slices.Concat(stateFiles, names.documents))
	names.recovery = sortedUnique(slices.Concat(names.plain, secretFiles))
	return names, true
}

func sortedUnique(names []string) []string {
	sorted := slices.Clone(names)
	slices.Sort(sorted)
	return slices.Compact(sorted)
}

func validDocumentName(name string) bool {
	return name != "" && len(name) <= 255 && name == strings.TrimSpace(name) && !strings.ContainsAny(name, `/\`) && filepath.IsLocal(name) && filepath.Base(name) == name && filepath.Ext(name) == ".json"
}

func (names stateNames) archived(includeSecrets bool) []string {
	if includeSecrets {
		return names.recovery
	}
	return names.plain
}

func (names stateNames) written(includeSecrets bool) []string {
	if includeSecrets {
		return names.recovery
	}
	return slices.DeleteFunc(slices.Clone(names.plain), func(name string) bool { return name == auditJournal })
}

func (names stateNames) document(name string) bool {
	_, found := slices.BinarySearch(names.documents, name)
	return found
}

func (names stateNames) entry(name string) bool {
	_, found := slices.BinarySearch(names.entries, name)
	return found
}

func (names stateNames) maxSize(name string) int {
	switch {
	case name == auditJournal:
		return maxAuditFileSize
	case names.entry(name):
		return maxEntryDocumentSize
	case names.document(name):
		return databaseDocumentLimit
	}
	return maxFileSize
}
