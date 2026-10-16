package server_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	appbackup "github.com/MikeO7/kinosail-subtitles/internal/backup"
	"github.com/MikeO7/kinosail-subtitles/internal/database"
	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

type subtitleInventory struct {
	Items []struct {
		ID     string `json:"id"`
		Frozen bool   `json:"frozen"`
	} `json:"items"`
}

func TestBackupAndRestoreKeepSubtitleLedgerAndFrozenChoices(t *testing.T) {
	t.Parallel()
	media, source, restored := t.TempDir(), t.TempDir(), t.TempDir()
	target := filepath.Join(media, "Arrival.en.srt")
	writeTestFile(t, filepath.Join(media, "Arrival.mp4"), "video")
	writeTestFile(t, target, "1\n00:00:01,000 --> 00:00:02,000\nCurrent\n")
	writeTestFile(t, target+".kinosail.bak", "1\n00:00:01,000 --> 00:00:02,000\nPrevious\n")
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: source, CacheDir: t.TempDir()})
	id := frozenSubtitleInventory(t, handler, false).Items[0].ID
	if response := requestJSON(t, handler, http.MethodPost, "/api/v1/subtitle-library/"+id+"/restore", `{}`); response.Code != http.StatusNoContent {
		t.Fatalf("restore previous = %d %q", response.Code, response.Body.String())
	}
	if response := requestJSON(t, handler, http.MethodPost, "/api/v1/subtitle-library/"+id+"/replacement", `{"replaceable":false}`); response.Code != http.StatusNoContent {
		t.Fatalf("keep subtitle = %d %q", response.Code, response.Body.String())
	}
	frozenSubtitleInventory(t, handler, true)
	const key = "correct horse battery staple"
	var archive bytes.Buffer
	if handled, err := appbackup.Command([]string{"backup"}, nil, &archive, source, key); !handled || err != nil {
		t.Fatalf("backup = %t, %v", handled, err)
	}
	if handled, err := appbackup.Command([]string{"restore"}, bytes.NewReader(archive.Bytes()), io.Discard, restored, key); !handled || err != nil {
		t.Fatalf("restore = %t, %v", handled, err)
	}
	restarted := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: restored, CacheDir: t.TempDir()})
	if inventory := frozenSubtitleInventory(t, restarted, true); inventory.Items[0].ID != id {
		t.Fatalf("restored inventory item = %q, want %q", inventory.Items[0].ID, id)
	}
	history := requestApp(t, restarted, http.MethodGet, "/api/v1/subtitle-library?view=history", "")
	if history.Code != http.StatusOK || !strings.Contains(history.Body.String(), `"action":"restored"`) {
		t.Fatalf("restored history = %d %q", history.Code, history.Body.String())
	}
	store, err := database.Open(restored, false)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.SaveEntry("subtitle_records.json", "after-restore:en", map[string]any{"source": "external"}); err != nil {
		t.Fatalf("restored ledger records are not in entry storage: %v", err)
	}
}

func frozenSubtitleInventory(t *testing.T, handler http.Handler, frozen bool) subtitleInventory {
	t.Helper()
	var inventory subtitleInventory
	response := requestApp(t, handler, http.MethodGet, "/api/v1/subtitle-library?view=library", "")
	if json.Unmarshal(response.Body.Bytes(), &inventory) != nil || len(inventory.Items) != 1 || inventory.Items[0].Frozen != frozen {
		t.Fatalf("inventory frozen=%t = %d %q", frozen, response.Code, response.Body.String())
	}
	return inventory
}
