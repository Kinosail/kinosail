package viewing

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestPlexSectionsAreBoundedAndOrdered(t *testing.T) { //nolint:cyclop,gocognit // The barrier verifies bounded overlap and source order.
	entered := make(chan struct{}, 4)
	release := make(chan struct{})
	var active, maximum atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/library/sections" {
			_, _ = writer.Write([]byte(`{"MediaContainer":{"Directory":[{"Key":"1","Type":"movie"},{"Key":"2","Type":"movie"},{"Key":"3","Type":"movie"},{"Key":"4","Type":"movie"}]}}`))
			return
		}
		current := active.Add(1)
		for previous := maximum.Load(); current > previous; previous = maximum.Load() {
			if maximum.CompareAndSwap(previous, current) {
				break
			}
		}
		defer active.Add(-1)
		entered <- struct{}{}
		select {
		case <-release:
		case <-request.Context().Done():
			return
		}
		key := strings.Split(request.URL.Path, "/")[3]
		writePlexSectionPage(writer, key)
	}))
	defer server.Close()
	done := make(chan struct {
		items []Activity
		err   error
	}, 1)
	go func() {
		items, err := Fetch(t.Context(), server.Client(), Input{Source: "plex", URL: server.URL, Token: "token"}, false)
		done <- struct {
			items []Activity
			err   error
		}{items, err}
	}()
	for range 3 {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			close(release)
			t.Fatal("Plex sections did not overlap")
		}
	}
	close(release)
	outcome := <-done
	items, err := outcome.items, outcome.err
	if err != nil || len(items) != 4 || maximum.Load() != 3 {
		t.Fatalf("Plex sections = %#v, maximum %d, %v", items, maximum.Load(), err)
	}
	for index, item := range items {
		if item.SourceID != strconv.Itoa(index+1) {
			t.Fatalf("section order = %#v", items)
		}
	}
}

func TestPlexSectionsRejectInvalidDiscoveryBeforeChildRequests(t *testing.T) {
	var childRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/library/sections" {
			_, _ = writer.Write([]byte(`{"MediaContainer":{"Directory":[{"Key":"1","Type":"movie"},{"Key":"","Type":"movie"}]}}`))
			return
		}
		childRequests.Add(1)
		_, _ = writer.Write([]byte(`{"MediaContainer":{"TotalSize":0,"Metadata":[]}}`))
	}))
	defer server.Close()
	if _, err := Fetch(t.Context(), server.Client(), Input{Source: "plex", URL: server.URL, Token: "token"}, false); err == nil || childRequests.Load() != 0 {
		t.Fatalf("invalid section made %d child requests: %v", childRequests.Load(), err)
	}
}

func TestPlexSectionsStopOnCancellation(t *testing.T) {
	entered := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/library/sections" {
			_, _ = writer.Write([]byte(`{"MediaContainer":{"Directory":[{"Key":"1","Type":"movie"},{"Key":"2","Type":"movie"}]}}`))
			return
		}
		select {
		case entered <- struct{}{}:
		default:
		}
		<-request.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		_, err := Fetch(ctx, server.Client(), Input{Source: "plex", URL: server.URL, Token: "token"}, false)
		done <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("Plex section did not start")
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Plex import = %v", err)
	}
}

func BenchmarkPlexSections(b *testing.B) { //nolint:gocognit // The benchmark compares complete serial and concurrent section fetches.
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/library/sections" {
			_, _ = writer.Write([]byte(`{"MediaContainer":{"Directory":[{"Key":"1","Type":"movie"},{"Key":"2","Type":"movie"},{"Key":"3","Type":"movie"},{"Key":"4","Type":"movie"}]}}`))
			return
		}
		time.Sleep(20 * time.Millisecond)
		key := strings.Split(request.URL.Path, "/")[3]
		writePlexSectionPage(writer, key)
	}))
	defer server.Close()
	input := Input{Source: "plex", URL: server.URL, Token: "token"}
	client := server.Client()
	for _, mode := range []string{"serial", "parallel"} {
		b.Run(mode, func(b *testing.B) {
			for b.Loop() {
				var activities []Activity
				var err error
				if mode == "serial" {
					for _, key := range []string{"1", "2", "3", "4"} {
						activities, err = fetchPlexSection(b.Context(), client, input, key, "movie", activities)
						if err != nil {
							break
						}
					}
				} else {
					activities, err = Fetch(b.Context(), client, input, false)
				}
				if err != nil || len(activities) != 4 {
					b.Fatalf("Plex import = %d activities, %v", len(activities), err)
				}
			}
		})
	}
}

func writePlexSectionPage(writer http.ResponseWriter, key string) {
	_ = json.NewEncoder(writer).Encode(map[string]any{"MediaContainer": map[string]any{"TotalSize": 1, "Metadata": []map[string]any{{"RatingKey": key, "Type": "movie", "Title": "Movie", "ViewCount": 1}}}})
}
