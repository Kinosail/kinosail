package catalog

import (
	"errors"
	"maps"
	"math"
	"sync"
	"time"
)

var ErrInvalidProgressState = errors.New("progress state is invalid")

type PlaybackState struct {
	Seconds      float64   `json:"seconds,omitempty"`
	ReaderOffset float64   `json:"readerOffset,omitempty"`
	ReaderPage   int       `json:"readerPage,omitempty"`
	Watched      bool      `json:"watched,omitempty"`
	Dismissed    bool      `json:"dismissed,omitempty"`
	Updated      time.Time `json:"updated,omitempty"`
	Session      string    `json:"session,omitempty"`
	Revision     uint64    `json:"revision,omitempty"`
}

type ProgressChange func(PlaybackState) (PlaybackState, bool, error)

type ProgressStorage struct {
	Mutex               *sync.RWMutex
	PersistMutex        *sync.Mutex
	Values              *map[string]PlaybackState
	File                string
	Persist             func(string, any) error
	PersistEntry        func(string, PlaybackState) error
}

func (storage ProgressStorage) Update(key string, change ProgressChange) (PlaybackState, PlaybackState, bool, error) {
	storage.PersistMutex.Lock()
	defer storage.PersistMutex.Unlock()
	return storage.commit(key, change)
}

func (storage ProgressStorage) commit(key string, change ProgressChange) (PlaybackState, PlaybackState, bool, error) {
	storage.Mutex.Lock()
	previous, current, changed, err := progressChange(*storage.Values, key, change)
	var values map[string]PlaybackState
	wholeDocument := storage.File != "" && storage.PersistEntry == nil
	if err == nil && changed && wholeDocument {
		values = maps.Clone(*storage.Values)
		values[key] = current
	}
	storage.Mutex.Unlock()
	if err != nil || !changed {
		return previous, current, changed, err
	}
	if err := storage.persistChange(key, current, values); err != nil {
		return previous, current, false, err
	}
	storage.Mutex.Lock()
	if wholeDocument {
		*storage.Values = values
	} else {
		if *storage.Values == nil {
			*storage.Values = make(map[string]PlaybackState)
		}
		(*storage.Values)[key] = current
	}
	storage.Mutex.Unlock()
	return previous, current, true, nil
}

func progressChange(values map[string]PlaybackState, key string, change ProgressChange) (PlaybackState, PlaybackState, bool, error) {
	previous := values[key]
	current, changed, err := change(previous)
	if err != nil || !changed {
		return previous, current, changed, err
	}
	_, exists := values[key]
	if !ValidProgressKey(key) || !ValidPlaybackState(current) || !exists && len(values) >= 1_000_000 {
		return previous, current, false, ErrInvalidProgressState
	}
	return previous, current, true, nil
}

func ValidateStoredProgress(values map[string]PlaybackState) error {
	if len(values) > 1_000_000 {
		return errors.New("too many persisted progress records")
	}
	for key, state := range values {
		if !ValidProgressKey(key) || !ValidPlaybackState(state) {
			return errors.New("persisted progress is invalid")
		}
	}
	return nil
}

func ValidProgressKey(key string) bool {
	return key != "" && len(key) <= 256
}

func ValidPlaybackState(state PlaybackState) bool {
	return progressRange(state.Seconds, 1e9) &&
		progressRange(state.ReaderOffset, 1) && (state.ReaderPage > 0 || state.ReaderOffset == 0) &&
		state.ReaderPage >= 0 && state.ReaderPage <= 10_000_000 && len(state.Session) <= 128
}

func (storage ProgressStorage) persistChange(key string, current PlaybackState, values map[string]PlaybackState) error {
	if storage.File == "" {
		return nil
	}
	if storage.PersistEntry != nil {
		return storage.PersistEntry(key, current)
	}
	return storage.Persist(storage.File, values)
}

func progressRange(value, maximum float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= maximum
}
