package downloads

import (
	"net/http"
	"os"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestManifestAPIRequiresVisibleReadyRevision(t *testing.T) {
	manager, job := readyRepairJob(t)
	job.ItemID = "item"
	manager.jobs[job.ID] = job
	mux := http.NewServeMux()
	RegisterAPI(mux, manager, testAccess{library.Item{ID: "item", Kind: "video"}})
	path := "/api/v1/downloads/" + job.ID + "/manifest"
	for _, tc := range []struct {
		suffix string
		allow  bool
		status int
	}{{"", true, 200}, {"?extra=1", true, 404}, {"", false, 404}} {
		response := requestAPI(t, mux, http.MethodGet, path+tc.suffix, "", tc.allow)
		if response.Code != tc.status {
			t.Fatalf("manifest = %d %s", response.Code, response.Body.String())
		}
	}
	job.State = "preparing"
	manager.jobs[job.ID] = job
	response := requestAPI(t, mux, http.MethodGet, path, "", true)
	if response.Code != 503 || response.Header().Get("Retry-After") != "15" {
		t.Fatalf("preparing = %d %v", response.Code, response.Header())
	}
	job.State = "failed"
	manager.jobs[job.ID] = job
	if response := requestAPI(t, mux, http.MethodGet, path, "", true); response.Code != 404 {
		t.Fatal("failed manifest exposed")
	}
	job.State = "ready"
	job.ItemID = "hidden"
	manager.jobs[job.ID] = job
	if response := requestAPI(t, mux, http.MethodGet, path, "", true); response.Code != 404 {
		t.Fatal("hidden item's manifest exposed")
	}
}

func TestManifestAPIReportsQueueBackpressure(t *testing.T) {
	manager, job := readyRepairJob(t)
	job.ItemID = "item"
	manager.jobs[job.ID] = job
	manager.manifestWork = make(map[string]*manifestFlight)
	for index := range QueueCapacity {
		manager.manifestWork[string(rune(index))] = &manifestFlight{}
	}
	mux := http.NewServeMux()
	RegisterAPI(mux, manager, testAccess{library.Item{ID: "item", Kind: "video"}})
	response := requestAPI(t, mux, http.MethodGet, "/api/v1/downloads/"+job.ID+"/manifest", "", true)
	if response.Code != 503 || response.Header().Get("Retry-After") != "15" {
		t.Fatalf("full queue = %d %v", response.Code, response.Header())
	}
	if _, err := os.Stat(job.File + ".manifest"); !os.IsNotExist(err) {
		t.Fatal("full queue wrote manifest")
	}
}
