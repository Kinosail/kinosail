package server

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"net/http"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MikeO7/kinosail-player/internal/database"
	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/library"
	sharedmetadata "github.com/MikeO7/kinosail/packages/metadata"
)

type MetadataConfig = sharedmetadata.Config

type metadataRecord = sharedmetadata.Record

const (
	metadataDocument    = "metadata.json"
	metadataRecordLimit = 100_000
)

type metadataStore struct {
	mu          sync.RWMutex
	file, cache string
	config      MetadataConfig
	active      atomic.Pointer[MetadataConfig]
	records     map[string]metadataRecord
	client      *http.Client
	tvmaze      *sharedmetadata.TVMazeProvider
	persist     func(string, any) error
	stateDB     *database.Store
	err         error
}

var errMetadataRecordLimit = errors.New("metadata record limit reached")

func newMetadataStore(config MetadataConfig, dataDir, cache string, databases ...*database.Store) *metadataStore {
	stateDB := configuredDatabase(databases)
	config = sharedmetadata.NormalizeConfig(config)
	store := &metadataStore{cache: cache, config: config, records: make(map[string]metadataRecord), client: localIntegrationHTTPClient(15 * time.Second), tvmaze: sharedmetadata.NewTVMazeProvider(config.TVMazeURL, ApplicationVersion), persist: statePersistence(stateDB)}
	store.active.Store(&config)
	store.load(dataDir, stateDB)
	return store
}

func (store *metadataStore) load(dataDir string, stateDB *database.Store) {
	if dataDir == "" {
		return
	}
	store.file = filepath.Join(dataDir, metadataDocument)
	_, store.err = loadState(stateDB, store.file, &store.records)
	if store.err == nil {
		store.err = sharedmetadata.ValidateRecords(store.records)
	}
	if store.records == nil {
		store.records = make(map[string]metadataRecord)
	}
	if store.err == nil && stateDB != nil {
		if store.err = stateDB.EnableEntries(metadataDocument); store.err == nil {
			store.stateDB = stateDB
		}
	}
}

func (store *metadataStore) configured() bool {
	config := store.config
	if current := store.active.Load(); current != nil {
		config = *current
	}
	return sharedmetadata.Configured(store.cache, config.Token, config.URL, config.ImageURL)
}

func (store *metadataStore) providerSnapshot() *metadataStore {
	config := store.config
	if current := store.active.Load(); current != nil {
		config = *current
	}
	return &metadataStore{cache: store.cache, config: config, client: store.client, tvmaze: store.tvmaze}
}

func (store *metadataStore) configureTMDB(token, url, imageURL string) {
	config := *store.active.Load()
	config.Token, config.URL, config.ImageURL = token, url, imageURL
	config = sharedmetadata.NormalizeConfig(config)
	store.active.Store(&config)
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

func (store *metadataStore) set(id string, record metadataRecord) error {
	return store.setAll(map[string]metadataRecord{id: record})
}

func (store *metadataStore) setAll(updates map[string]metadataRecord) error {
	if err := sharedmetadata.ValidateRecords(updates); err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.stateDB != nil {
		return store.saveEntries(updates)
	}
	records := sharedmetadata.MergeRecords(store.records, updates)
	if len(records) > metadataRecordLimit {
		return errMetadataRecordLimit
	}
	if store.file != "" {
		if err := store.persist(store.file, records); err != nil {
			return err
		}
	}
	store.records = records
	return nil
}

func (store *metadataStore) saveEntries(updates map[string]metadataRecord) error {
	cloned := make(map[string]metadataRecord, len(updates))
	values := make(map[string]any, len(updates))
	added := 0
	for id, record := range updates {
		if _, exists := store.records[id]; !exists {
			added++
		}
		cloned[id] = sharedmetadata.Clone(record)
		values[id] = cloned[id]
	}
	if len(store.records)+added > metadataRecordLimit {
		return errMetadataRecordLimit
	}
	if err := store.stateDB.SaveEntries(metadataDocument, values, nil); err != nil {
		return err
	}
	maps.Copy(store.records, cloned)
	return nil
}

func (store *metadataStore) editHandler(index *libraryIndex) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		request.Body = http.MaxBytesReader(writer, request.Body, 8<<10)
		if !httpguard.FormEncoded(request) || request.URL.RawQuery != "" || request.ParseForm() != nil || !httpguard.OnlyFormKeys(request.PostForm, "title", "year", "plot", "rating", "tagline", "genres") {
			localizedError(writer, request, "metadata is invalid", http.StatusBadRequest)
			return
		}
		item, found := index.Find(request.PathValue("id"))
		store.mu.RLock()
		record := store.records[item.ID]
		store.mu.RUnlock()
		record, valid := sharedmetadata.RecordFromForm(request.PostForm, record)
		if !found || !valid {
			localizedError(writer, request, "metadata is invalid", http.StatusBadRequest)
			return
		}
		if err := store.set(item.ID, record); err != nil || index.Refresh(request.Context()) != nil {
			localizedError(writer, request, "metadata could not be saved", http.StatusInternalServerError)
			return
		}
		http.Redirect(writer, request, "/watch/"+item.ID, http.StatusSeeOther)
	}
}

func (store *metadataStore) refreshHandler(index *libraryIndex) http.HandlerFunc {
	return sharedmetadata.RefreshHandler(index, store.fetch, localizedError, localizedNotFound)
}

func (store *metadataStore) fetch(ctx context.Context, item library.Item) error {
	record, err := store.providerSnapshot().fetchRecord(ctx, item)
	if err != nil {
		return err
	}
	return store.set(item.ID, record)
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

func (store *metadataStore) downloadImage(ctx context.Context, path, target string, backdrop bool) error {
	if !backdrop {
		return sharedmetadata.DownloadProviderImage(ctx, store.client, store.config.ImageURL, path, target)
	}
	err := sharedmetadata.DownloadProviderBackdrop(ctx, store.client, store.config.ImageURL, path, target)
	if err != nil && ctx.Err() == nil && store.config.ImageURL == sharedmetadata.NormalizeConfig(MetadataConfig{}).ImageURL {
		failureClass := "response-or-transport"
		var validation *sharedmetadata.BackdropValidationError
		if errors.As(err, &validation) {
			failureClass = validation.FailureClass()
		}
		slog.WarnContext(ctx, "Metadata backdrop rejected", "request_id", requestActivityID(ctx),
			"operation", "metadata.backdrop", "failure_class", failureClass, "outcome", "cache-not-replaced")
	}
	return err
}

func (store *metadataStore) artworkPath(id string) string {
	return filepath.Join(store.cache, "metadata", id+".jpg")
}
