package productapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"unicode"

	"github.com/MikeO7/kinosail/packages/apihttp"
	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/metadata"
)

const maximumItemID = 128

// MetadataIndex is the app-owned Library port used by metadata mutations.
type MetadataIndex interface {
	Find(string) (library.Item, bool)
	Refresh(context.Context) error
}

// MetadataStore is the app-owned persistence and provider port used by metadata mutations.
type MetadataStore interface {
	Record(string) metadata.Record
	Save(string, metadata.Record) error
	Fetch(context.Context, library.Item) error
}

// MetadataPatch contains the canonical owner-editable bulk fields.
type MetadataPatch struct {
	Title   *string `json:"title"`
	Year    *string `json:"year"`
	Plot    *string `json:"plot"`
	Rating  *string `json:"rating"`
	Tagline *string `json:"tagline"`
	Genres  *string `json:"genres"`
}

// BulkMetadataStore applies one validated bulk edit transaction.
type BulkMetadataStore interface {
	ApplyMetadata(context.Context, []string, MetadataPatch) (int, error)
}

// BulkMetadataFunc adapts app persistence without moving HTTP behavior back into the app.
type BulkMetadataFunc func(context.Context, []string, MetadataPatch) (int, error)

// ApplyMetadata implements BulkMetadataStore.
func (apply BulkMetadataFunc) ApplyMetadata(ctx context.Context, ids []string, patch MetadataPatch) (int, error) {
	return apply(ctx, ids, patch)
}

type metadataInput struct{ Title, Year, Plot, Rating, Tagline, Genres string }

// EditMetadata validates and applies one owner-authored metadata record.
func EditMetadata(index MetadataIndex, store MetadataStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		id := request.PathValue("id")
		if !validItemID(id) || request.URL.RawQuery != "" || !jsonRequest(request) {
			apihttp.Error(writer, errors.New("metadata is invalid"), http.StatusBadRequest)
			return
		}
		var input metadataInput
		err := decodeObjectRequest(writer, request, &input)
		if err != nil {
			apihttp.Error(writer, err, http.StatusBadRequest)
			return
		}
		item, found := index.Find(id)
		if !found {
			apihttp.NotFound(writer)
			return
		}
		if !boundedInput(input) {
			apihttp.Error(writer, errors.New("metadata is invalid"), http.StatusBadRequest)
			return
		}
		record := store.Record(item.ID)
		record.Title, record.Year = strings.TrimSpace(input.Title), strings.TrimSpace(input.Year)
		record.Plot, record.Rating = strings.TrimSpace(input.Plot), strings.TrimSpace(input.Rating)
		record.Tagline, record.Genres = strings.TrimSpace(input.Tagline), strings.TrimSpace(input.Genres)
		record.Owner = true
		if !metadata.Valid(record) {
			apihttp.Error(writer, errors.New("metadata is invalid"), http.StatusBadRequest)
			return
		}
		if store.Save(item.ID, record) != nil || index.Refresh(request.Context()) != nil {
			apihttp.Error(writer, errors.New("metadata could not be saved"), http.StatusInternalServerError)
			return
		}
		apihttp.WriteJSON(writer, map[string]string{"id": item.ID, "title": record.Title}, http.StatusOK)
	}
}

// BulkEditMetadata validates and applies one bulk owner metadata transaction.
func BulkEditMetadata(store BulkMetadataStore, invalid ...error) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.RawQuery != "" || !jsonRequest(request) {
			apihttp.Error(writer, errors.New("invalid JSON request"), http.StatusBadRequest)
			return
		}
		var input struct {
			IDs []string `json:"ids"`
			MetadataPatch
		}
		if err := decodeObjectRequest(writer, request, &input); err != nil {
			apihttp.Error(writer, err, http.StatusBadRequest)
			return
		}
		updated, err := store.ApplyMetadata(request.Context(), input.IDs, input.MetadataPatch)
		if err != nil {
			status := http.StatusInternalServerError
			for _, candidate := range invalid {
				if errors.Is(err, candidate) {
					status = http.StatusBadRequest
				}
			}
			apihttp.Error(writer, err, status)
			return
		}
		apihttp.WriteJSON(writer, map[string]int{"updated": updated}, http.StatusOK)
	}
}

// RefreshMetadata validates and refreshes one video metadata record.
func RefreshMetadata(index MetadataIndex, store MetadataStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		id := request.PathValue("id")
		if !validItemID(id) || !httpguard.EmptyMutationRequest(writer, request) {
			apihttp.Error(writer, errors.New("metadata request is invalid"), http.StatusBadRequest)
			return
		}
		item, found := index.Find(id)
		if !found || item.Kind != "video" {
			apihttp.NotFound(writer)
			return
		}
		if store.Fetch(request.Context(), item) != nil || index.Refresh(request.Context()) != nil {
			apihttp.Error(writer, errors.New("metadata provider unavailable"), http.StatusBadGateway)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}
}

func jsonRequest(request *http.Request) bool {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	return err == nil && mediaType == "application/json"
}

func decodeObjectRequest(writer http.ResponseWriter, request *http.Request, target any) error {
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	data, err := io.ReadAll(request.Body)
	if err != nil {
		return errors.New("invalid JSON request")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return errors.New("invalid JSON request")
	}
	if err := decodeUniqueObjectFields(decoder); err != nil {
		return err
	}
	_, _ = decoder.Token() // More returning false at object scope guarantees the closing delimiter.
	if err = decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request must contain one JSON object")
	}
	decoder = json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(target); err != nil {
		return errors.New("invalid JSON request")
	}
	return nil
}

func decodeUniqueObjectFields(decoder *json.Decoder) error {
	seen := make(map[string]bool)
	for decoder.More() {
		name, err := decoder.Token()
		key, ok := name.(string)
		identity := strings.ToLower(key)
		var value json.RawMessage
		if err != nil || !ok || seen[identity] || decoder.Decode(&value) != nil {
			return errors.New("invalid JSON request")
		}
		seen[identity] = true
	}
	return nil
}

func validItemID(id string) bool {
	return id != "" && len(id) <= maximumItemID && !strings.ContainsAny(id, `/\`) && strings.IndexFunc(id, unicode.IsControl) < 0
}

func boundedInput(input metadataInput) bool {
	return len(input.Title) <= 200 && len(input.Year) <= 4 && len(input.Plot) <= 5000 && len(input.Rating) <= 32 && len(input.Tagline) <= 300 && len(input.Genres) <= 500
}
