package server

import (
	"context"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/database"
	"github.com/MikeO7/kinosail/packages/library"
	sharedmetadata "github.com/MikeO7/kinosail/packages/metadata"
)

type MetadataConfig = sharedmetadata.Config

type metadataRecord = sharedmetadata.Record

type metadataStore struct {
	mu          sync.RWMutex
	file, cache string
	config      MetadataConfig
	records     map[string]metadataRecord
	client      *http.Client
	tvmaze      *sharedmetadata.TVMazeProvider
	persist     func(string, any) error
	err         error
}

func newMetadataStore(config MetadataConfig, dataDir, cache string, databases ...*database.Store) *metadataStore {
	stateDB := configuredDatabase(databases)
	config = sharedmetadata.NormalizeConfig(config)
	store := &metadataStore{cache: cache, config: config, records: make(map[string]metadataRecord), client: sharedmetadata.IntegrationHTTPClient(15 * time.Second), tvmaze: sharedmetadata.NewTVMazeProvider(config.TVMazeURL, ApplicationVersion), persist: statePersistence(stateDB)}
	if dataDir != "" {
		store.file = filepath.Join(dataDir, "metadata.json")
		_, store.err = loadState(stateDB, store.file, &store.records)
		if store.err == nil {
			store.err = sharedmetadata.ValidateRecords(store.records)
		}
	}
	return store
}

func (store *metadataStore) configured() bool {
	return sharedmetadata.Configured(store.cache, store.config.Token, store.config.URL, store.config.ImageURL)
}

func (store *metadataStore) available() bool { return store.configured() || store.tvmaze != nil }

func (store *metadataStore) label() string {
	if store.configured() {
		return "Licensed credential configured"
	}
	return "Not configured"
}

func (store *metadataStore) apply(items []library.Item) []library.Item {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return sharedmetadata.ApplyRecords(items, store.records, store.currentArtwork)
}

func (store *metadataStore) setAll(updates map[string]metadataRecord) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	records := sharedmetadata.MergeRecords(store.records, updates)
	if store.file != "" {
		if err := store.persist(store.file, records); err != nil {
			return err
		}
	}
	store.records = records
	return nil
}

func (store *metadataStore) tvMetadata(ctx context.Context, item library.Item, id int, name, date string, record *metadataRecord, poster *string) {
	sharedmetadata.TMDBDetails{BaseURL: store.config.URL, Fetch: store.getJSON}.TVMetadata(ctx, item, id, name, date, record, poster)
}

func (store *metadataStore) movieCollection(ctx context.Context, id int) string {
	return (sharedmetadata.TMDBDetails{BaseURL: store.config.URL, Fetch: store.getJSON}).MovieCollection(ctx, id)
}

func (store *metadataStore) getJSON(ctx context.Context, endpoint string, target any) error {
	return sharedmetadata.GetProviderJSON(ctx, store.client, endpoint, store.config.Token, target)
}

func (store *metadataStore) downloadImage(ctx context.Context, path, target string) error {
	return sharedmetadata.DownloadProviderImage(ctx, store.client, store.config.ImageURL, path, target)
}

func (store *metadataStore) artworkPath(id string) string {
	return filepath.Join(store.cache, "metadata", id+".jpg")
}
