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

var durablePlayerDocuments = []string{"agent_connections.json", "api_keys.json", "media_shares.json", "profiles.json", "sessions.json", "settings.json"}

type Validator func(name string, data []byte) error

type Config struct {
	Documents               []string
	RetiredDocuments        []string
	DurableDocuments        []string
	Validate                Validator
	ValidateBeforeMigration bool
}

func PlayerDocuments() []string { return slices.Clone(playerDocuments) }

func PlayerConfig() Config {
	return Config{Documents: PlayerDocuments(), RetiredDocuments: []string{"live-tv.json"}, DurableDocuments: slices.Clone(durablePlayerDocuments)}
}

func validateConfig(config Config) (Config, map[string]struct{}, error) {
	if len(config.Documents) == 0 || len(config.Documents) > maxDocuments || len(config.RetiredDocuments) > maxDocuments || len(config.DurableDocuments) > len(config.Documents) {
		return Config{}, nil, errors.New("invalid document configuration")
	}
	config.Documents = slices.Clone(config.Documents)
	config.RetiredDocuments = slices.Clone(config.RetiredDocuments)
	config.DurableDocuments = slices.Clone(config.DurableDocuments)
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
	for _, name := range config.DurableDocuments {
		if _, ok := allowed[name]; !ok {
			return Config{}, nil, errors.New("invalid document configuration")
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
