package documentdb

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
)

const maxDocuments = 256

var playerDocuments = []string{
	"agent_connections.json", "api_keys.json", "collections.json", "history.json", "lists.json", "media_shares.json", "metadata.json",
	"playback_markers.json", "playlist_order.json", "playlists.json", "profiles.json", "progress.json", "media-experience.json",
	"sessions.json", "settings.json", "smart_playlists.json", "updates.json",
}

// Validator applies app-specific semantic rules after common JSON validation.
type Validator func(name string, data []byte) error

// Config defines the app-owned document vocabulary and migration policy.
type Config struct {
	Documents               []string
	RetiredDocuments        []string
	Validate                Validator
	ValidateBeforeMigration bool
}

// PlayerDocuments returns the canonical Player state vocabulary.
func PlayerDocuments() []string { return slices.Clone(playerDocuments) }

// PlayerConfig returns the canonical Player document-store policy.
func PlayerConfig() Config {
	return Config{Documents: PlayerDocuments(), RetiredDocuments: []string{"live-tv.json"}}
}

func validateConfig(config Config) (Config, map[string]struct{}, error) {
	if len(config.Documents) == 0 || len(config.Documents) > maxDocuments || len(config.RetiredDocuments) > maxDocuments {
		return Config{}, nil, errors.New("invalid document configuration")
	}
	config.Documents = slices.Clone(config.Documents)
	config.RetiredDocuments = slices.Clone(config.RetiredDocuments)
	allowed := make(map[string]struct{}, len(config.Documents))
	all := make(map[string]struct{}, len(config.Documents)+len(config.RetiredDocuments))
	for _, name := range config.Documents {
		if err := addName(name, all); err != nil {
			return Config{}, nil, err
		}
		allowed[name] = struct{}{}
	}
	for _, name := range config.RetiredDocuments {
		if err := addName(name, all); err != nil {
			return Config{}, nil, err
		}
	}
	return config, allowed, nil
}

func addName(name string, names map[string]struct{}) error {
	if !validName(name) {
		return errors.New("invalid document configuration")
	}
	if _, duplicate := names[name]; duplicate {
		return errors.New("invalid document configuration")
	}
	names[name] = struct{}{}
	return nil
}

func validName(name string) bool {
	return name != "" && name == strings.TrimSpace(name) && len(name) <= 255 && !strings.ContainsAny(name, `/\`) && filepath.Base(name) == name && filepath.Ext(name) == ".json"
}
