package metadata

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

type TMDBClient struct {
	token, baseURL, imageURL, cacheDir string
	http                               *http.Client
}

// TMDBConfig contains the optional legacy movie enrichment settings.
type TMDBConfig struct {
	Token, URL, ImageURL, CacheDir string
}

type TMDBMetadata struct {
	Title, Year, Plot, Genres, Director, Poster string
	Cast                                        []library.Person
	TMDBID                                      int
}

type tmdbMovie struct {
	Title       string `json:"title"`
	ReleaseDate string `json:"release_date"`
	Overview    string `json:"overview"`
	PosterPath  string `json:"poster_path"`
	Genres      []struct {
		Name string `json:"name"`
	} `json:"genres"`
	Credits struct {
		Cast []TMDBCastMember `json:"cast"`
		Crew []struct {
			Name       string `json:"name"`
			Job        string `json:"job"`
			Department string `json:"department"`
		} `json:"crew"`
	} `json:"credits"`
}

type TMDBCastMember struct {
	Name        string `json:"name"`
	Character   string `json:"character"`
	ProfilePath string `json:"profile_path"`
}

// NewTMDB returns a bounded movie metadata client.
func NewTMDB(config TMDBConfig) *TMDBClient {
	if config.Token == "" && config.CacheDir == "" {
		return nil
	}
	publicProvider := config.URL == "" && config.ImageURL == ""
	if config.URL == "" {
		config.URL = "https://api.themoviedb.org/3"
	}
	if config.ImageURL == "" {
		config.ImageURL = "https://image.tmdb.org/t/p/w500"
	}
	if !validProviderBaseURL(config.URL) || !validProviderBaseURL(config.ImageURL) {
		return nil
	}
	client := hardenedHTTPClient(20 * time.Second)
	if !publicProvider {
		client = localIntegrationHTTPClient(20 * time.Second)
	}
	return &TMDBClient{config.Token, strings.TrimRight(config.URL, "/"), strings.TrimRight(config.ImageURL, "/"), filepath.Join(config.CacheDir, "tmdb"), client}
}

func (client *TMDBClient) Enrich(ctx context.Context, items []library.Item) []library.Item { //nolint:cyclop // Cached fallback, duplicate identity, and bounded enrichment share one Library pass.
	if client == nil {
		return items
	}
	jobs := make([]int, 0)
	seen := make(map[string]bool)
	duplicate := false
	for index := range items {
		if items[index].Kind != "video" || items[index].Show != "" {
			continue
		}
		jobs = append(jobs, index)
		duplicate = duplicate || seen[items[index].ID]
		seen[items[index].ID] = true
	}
	workerCount := min(len(jobs), max(1, min(3, runtime.GOMAXPROCS(0)-1)))
	if duplicate || workerCount < 2 {
		for _, index := range jobs {
			client.enrichOne(ctx, &items[index])
		}
		return items
	}
	queue := make(chan int)
	var workers sync.WaitGroup
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range queue {
				client.enrichOne(ctx, &items[index])
			}
		}()
	}
	for _, index := range jobs {
		queue <- index
	}
	close(queue)
	workers.Wait()
	return items
}

func (client *TMDBClient) enrichOne(ctx context.Context, item *library.Item) {
	cached, fresh := client.Load(item.ID)
	if fresh || client.token == "" {
		applyTMDB(item, cached)
		return
	}
	metadata, err := client.fetch(ctx, *item)
	if err != nil {
		applyTMDB(item, cached)
		slog.Warn("TMDB enrichment failed", "title", item.Title, "error", err)
		return
	}
	_ = saveJSON(client.metadataPath(item.ID), metadata)
	applyTMDB(item, metadata)
}

func metadataFor(movie tmdbMovie) TMDBMetadata {
	metadata := TMDBMetadata{Title: strings.TrimSpace(movie.Title), Plot: strings.TrimSpace(movie.Overview)}
	if len(movie.ReleaseDate) >= 4 {
		metadata.Year = movie.ReleaseDate[:4]
	}
	genres := make([]string, 0, len(movie.Genres))
	for _, genre := range movie.Genres {
		genres = append(genres, strings.TrimSpace(genre.Name))
	}
	metadata.Genres = strings.Join(genres, " · ")
	for _, person := range movie.Credits.Crew {
		if strings.EqualFold(person.Job, "Director") {
			metadata.Director = strings.TrimSpace(person.Name)
			break
		}
	}
	return metadata
}

func (client *TMDBClient) get(ctx context.Context, endpoint string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+client.token)
	request.Header.Set("Accept", "application/json")
	response, err := client.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("TMDB returned %s", response.Status)
	}
	return decodeExternalJSON(response.Body, 4<<20, target)
}

func (client *TMDBClient) download(ctx context.Context, remotePath, destination string) (string, error) {
	if remotePath == "" {
		return "", nil
	}
	if !ValidTMDBPath(remotePath) {
		return "", errors.New("TMDB image path is invalid")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.imageURL+remotePath, nil)
	if err != nil {
		return "", err
	}
	response, err := client.http.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || !strings.HasPrefix(response.Header.Get("Content-Type"), "image/") {
		return "", fmt.Errorf("TMDB image returned %s", response.Status)
	}
	extension := filepath.Ext(remotePath)
	if extension == "" {
		extension = ".jpg"
	}
	destination += extension
	return cacheImage(response.Body, destination)
}

func cacheImage(source io.Reader, destination string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return "", err
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".tmdb-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(temporary.Name())
	written, copyErr := io.Copy(temporary, io.LimitReader(source, 8<<20+1))
	closeErr := temporary.Close()
	if copyErr != nil || closeErr != nil || written > 8<<20 {
		return "", errors.New("TMDB image exceeds cache limit")
	}
	if err := os.Chmod(temporary.Name(), 0o600); err != nil {
		return "", err
	}
	return destination, os.Rename(temporary.Name(), destination)
}

func applyTMDB(item *library.Item, metadata TMDBMetadata) {
	if !item.LocalTitle && metadata.Title != "" {
		item.Title = metadata.Title
	}
	if item.Year == "" {
		item.Year = metadata.Year
	}
	if item.Plot == "" {
		item.Plot = metadata.Plot
	}
	if item.Genres == "" {
		item.Genres = metadata.Genres
	}
	if item.Director == "" {
		item.Director = metadata.Director
	}
	if item.Artwork == "" {
		item.Artwork = metadata.Poster
	}
	if len(item.Cast) == 0 {
		item.Cast = metadata.Cast
	}
	if metadata.TMDBID != 0 {
		item.ProviderIDs = map[string]string{"tmdb": strconv.Itoa(metadata.TMDBID)}
	}
}
