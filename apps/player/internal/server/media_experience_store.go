package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"unicode"

	"github.com/MikeO7/kinosail-player/internal/database"
	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/library"
)

const (
	experienceLimit = 4096
	experienceBytes = 8 << 20
)

var (
	errMediaExperience   = errors.New("media preferences could not be saved")
	experienceKeyPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

type mediaBookmark struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Seconds *float64 `json:"seconds,omitempty"`
	Offset  *float64 `json:"offset,omitempty"`
	Page    *int     `json:"page,omitempty"`
}

func (value mediaBookmark) validate() error {
	if !experienceKeyPattern.MatchString(value.ID) || !validBookmarkTitle(value.Title) || (value.Seconds == nil) == (value.Page == nil) {
		return errors.New("invalid bookmark")
	}
	return value.validatePosition()
}

func (value mediaBookmark) validatePosition() error {
	if value.Seconds != nil && !inMediaRange(*value.Seconds, 0, 31_536_000) {
		return errors.New("invalid bookmark position")
	}
	if value.Page != nil && (*value.Page < 1 || *value.Page > 10_000) {
		return errors.New("invalid bookmark page")
	}
	if value.Offset != nil && (value.Page == nil || !inMediaRange(*value.Offset, 0, 1)) {
		return errors.New("invalid bookmark offset")
	}
	return nil
}

func validBookmarkTitle(title string) bool {
	return title == strings.TrimSpace(title) && title != "" && len(title) <= 160 && !strings.ContainsFunc(title, unicode.IsControl)
}

type mediaExperienceState struct {
	Defaults  map[string]mediaPreferences    `json:"defaults"`
	Overrides map[string]playbackPreferences `json:"overrides"`
	Bookmarks map[string][]mediaBookmark     `json:"bookmarks"`
}

type mediaExperienceStore struct {
	mu      sync.Mutex
	value   mediaExperienceState
	file    string
	persist func(string, any) error
	err     error
}

func experienceKey(viewer, scope string) string {
	digest := sha256.Sum256([]byte(viewer + "\x00" + scope))
	return hex.EncodeToString(digest[:])
}

func validateExperience(value mediaExperienceState) error {
	if max(len(value.Defaults), len(value.Overrides), len(value.Bookmarks)) > experienceLimit {
		return errMediaExperience
	}
	for key, prefs := range value.Defaults {
		if !experienceKeyPattern.MatchString(key) || prefs.validate() != nil {
			return errMediaExperience
		}
	}
	for key, prefs := range value.Overrides {
		if !experienceKeyPattern.MatchString(key) || prefs.validate() != nil {
			return errMediaExperience
		}
	}
	for key, bookmarks := range value.Bookmarks {
		if !validStoredBookmarks(key, bookmarks) {
			return errMediaExperience
		}
	}
	return nil
}

func validStoredBookmarks(key string, bookmarks []mediaBookmark) bool {
	if !experienceKeyPattern.MatchString(key) || len(bookmarks) > 100 {
		return false
	}
	seen := map[string]bool{}
	for _, bookmark := range bookmarks {
		if bookmark.validate() != nil || seen[bookmark.ID] {
			return false
		}
		seen[bookmark.ID] = true
	}
	return true
}

func newMediaExperienceStore(dataDir string, db *database.Store) *mediaExperienceStore {
	store := &mediaExperienceStore{value: mediaExperienceState{Defaults: map[string]mediaPreferences{}, Overrides: map[string]playbackPreferences{}, Bookmarks: map[string][]mediaBookmark{}}, persist: statePersistence(db)}
	if dataDir == "" {
		return store
	}
	store.file = filepath.Join(dataDir, "media-experience.json")
	var raw json.RawMessage
	if found, err := loadState(db, store.file, &raw); err != nil {
		store.err = errMediaExperience
	} else if found {
		var value mediaExperienceState
		if httpguard.DecodeJSON(bytes.NewReader(raw), experienceBytes, &value, true) != nil || validateExperience(value) != nil {
			store.err = errMediaExperience
		} else {
			if value.Defaults != nil {
				store.value.Defaults = value.Defaults
			}
			if value.Overrides != nil {
				store.value.Overrides = value.Overrides
			}
			if value.Bookmarks != nil {
				store.value.Bookmarks = value.Bookmarks
			}
		}
	}
	return store
}

// The caller holds mu; publish only after the durable write succeeds.
func (store *mediaExperienceStore) change(change func(*mediaExperienceState) error) error {
	if store.err != nil {
		return store.err
	}
	next := mediaExperienceState{Defaults: maps.Clone(store.value.Defaults), Overrides: maps.Clone(store.value.Overrides), Bookmarks: maps.Clone(store.value.Bookmarks)}
	if err := change(&next); err != nil {
		return err
	}
	if err := validateExperience(next); err != nil {
		return err
	}
	raw, err := json.Marshal(next)
	if err != nil || len(raw) > experienceBytes {
		return errMediaExperience
	}
	if store.file != "" {
		if err := store.persist(store.file, next); err != nil {
			return errMediaExperience
		}
	}
	store.value = next
	return nil
}

func (store *mediaExperienceStore) defaults(viewer string) mediaPreferences {
	if value, found := store.value.Defaults[experienceKey(viewer, "defaults")]; found {
		return value
	}
	return defaultMediaPreferences()
}

func (store *mediaExperienceStore) playback(viewer string, item library.Item) playbackPreferences {
	store.mu.Lock()
	defer store.mu.Unlock()
	if value, found := store.value.Overrides[experienceKey(viewer, mediaPreferenceScope(item))]; found {
		return value
	}
	return store.defaults(viewer).Playback
}

func (store *mediaExperienceStore) bookmarks(key string) []mediaBookmark {
	return slices.Clone(store.value.Bookmarks[key])
}
