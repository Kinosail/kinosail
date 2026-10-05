package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"reflect"
	"context"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	flag.Parse()
	if *fixtureServe { os.Exit(runFixture()) }
	os.Exit(m.Run())
}

func TestSaveHeadersLegacyActualCompletion(t *testing.T) { checkRealFault(t, "headers", false) }
func TestSaveBodyLegacyActualCompletion(t *testing.T) { checkRealFault(t, "body", false) }
func TestSaveHeadersReceiptActualCompletion(t *testing.T) { checkRealFault(t, "headers", true) }
func TestSaveBodyReceiptActualCompletion(t *testing.T) { checkRealFault(t, "body", true) }

func checkRealFault(t *testing.T, mode string, prepared bool) {
	t.Helper()
	private := os.Getenv("R06_SAVE_PRIVATE_ROOT")
	root, err := os.MkdirTemp(private, "r06-control-")
	if err != nil { t.Fatal("private fixture directory unavailable") }
	f, err := newFixture(root, mode)
	if err != nil { t.Fatal("real Server fixture construction failed") }
	defer f.stop()
	client, csrf, err := enrollOwner(t, f)
	if err != nil { t.Fatal("actual TLS Owner/MFA/CSRF prerequisite failed") }
	id, err := controlItem(client, f.origin)
	if err != nil { t.Fatal("public fictional item prerequisite failed") }
	base := "/api/v1/subtitle-library/" + id
	var before struct { Fingerprint string }
	if controlJSON(client, f.origin, http.MethodGet, base+"/inspect?language=en", nil, nil, &before) != 200 {
		t.Fatal("public inspection prerequisite failed")
	}
	input, _ := json.Marshal(map[string]any{
		"language":"en", "fingerprint":before.Fingerprint, "text":savedSRT,
		"role":"translation", "automaticSync":false, "removeCredits":false, "mergeRepeated":false,
	})
	headers := http.Header{"X-Kinosail-CSRF":{csrf}}
	if controlJSON(client, f.origin, http.MethodPost, base+"/preview", input, headers, nil) != 200 {
		t.Fatal("real public preview prerequisite failed")
	}
	if prepared {
		preparation, _ := json.Marshal(map[string]string{"action":"apply", "item":id})
		var receipt struct { ID string }
		if controlJSON(client, f.origin, http.MethodPost, "/api/v1/subtitle-operations",
			preparation, headers, &receipt) != 201 || !operationID.MatchString(receipt.ID) {
			t.Fatal("real public preparation prerequisite failed")
		}
		headers.Set("X-Kinosail-Operation", receipt.ID)
	}
	response := make(chan *http.Response, 1)
	failed := make(chan bool, 1)
	requestDone := make(chan struct{})
	var bodyReadDone chan struct{}
	requestContext, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer func() {
		cancel()
		f.stop()
		select { case <-requestDone: case <-time.After(3*time.Second): t.Error("owned request did not settle") }
		if bodyReadDone != nil { select { case <-bodyReadDone: case <-time.After(3*time.Second): t.Error("owned body read did not settle") } }
		select { case abandoned := <-response: abandoned.Body.Close(); default: }
	}()
	go func() {
		defer close(requestDone)
		request, requestErr := http.NewRequestWithContext(requestContext, http.MethodPost, f.origin+base+"/apply", bytes.NewReader(input))
		if requestErr != nil { failed <- true; return }
		request.Header = headers.Clone()
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", f.origin)
		request.Header.Set("User-Agent", "R06-private-control")
		result, requestErr := client.Do(request)
		if requestErr != nil { failed <- true; return }
		response <- result
	}()
	select {
	case <-f.eligible:
	case <-failed: t.Fatal("Save transport prerequisite failed")
	case <-time.After(10*time.Second): t.Fatal("actual Save/History/receipt witness missing")
	}
	snapshot := f.snapshot()
	if !snapshot.ActualSaved || !snapshot.HistoryOnce || !snapshot.RecoveryMatches ||
		!snapshot.InspectionMatches || !snapshot.HoldEligible || snapshot.SaveAttempts != 1 {
		t.Fatal("actual persisted Save/History witness is not eligible")
	}
	expected := 200
	if prepared { expected = 202 }
	var result *http.Response
	if mode == "headers" {
		select { case <-response: t.Fatal("held headers escaped before release"); case <-failed: t.Fatal("held request failed"); case <-time.After(200*time.Millisecond): }
	} else {
		select {
		case result = <-response:
		case <-failed: t.Fatal("actual response headers unavailable")
		case <-time.After(2*time.Second): t.Fatal("body fault also withheld headers")
		}
		bodyDone := make(chan bool, 1)
		bodyReadDone = make(chan struct{})
		bodyReadStarted := make(chan struct{})
		go func() {
			defer close(bodyReadDone)
			close(bodyReadStarted)
			data, readErr := io.ReadAll(io.LimitReader(result.Body, 65537))
			result.Body.Close()
			bodyDone <- readErr == nil && bytes.Equal(data, f.actualBody())
		}()
		<-bodyReadStarted
		select { case <-bodyDone: t.Fatal("held body escaped before release"); case <-time.After(200*time.Millisecond): }
		f.release()
		select {
		case exact := <-bodyDone: if !exact { t.Fatal("released actual body changed") }
		case <-time.After(3*time.Second): t.Fatal("released body did not settle")
		}
	}
	if mode == "headers" {
		f.release()
		select {
		case result = <-response:
		case <-failed: t.Fatal("released response failed")
		case <-time.After(3*time.Second): t.Fatal("released response did not settle")
		}
		data, readErr := io.ReadAll(io.LimitReader(result.Body, 65537))
		result.Body.Close()
		if readErr != nil || !bytes.Equal(data, f.actualBody()) { t.Fatal("released actual body changed") }
	}
	if result.StatusCode != expected || !equalApplicationHeaders(result.Header, f.actualHeaders()) {
		t.Fatal("released actual response status or headers changed")
	}
	settlement := time.Now().Add(2*time.Second)
	snapshot = f.snapshot()
	for snapshot.ActiveHolds != 0 && time.Now().Before(settlement) {
		time.Sleep(10*time.Millisecond)
		snapshot = f.snapshot()
	}
	if snapshot.SaveAttempts != 1 || !snapshot.ActualSaved || !snapshot.HistoryOnce {
		t.Fatal("release replayed or changed the actual Save")
	}
	if snapshot.ActiveHolds != 0 || !snapshot.ResponseBodyWritten || snapshot.ClientCancelled || snapshot.BoundaryFailed {
		t.Fatal("actual released response did not settle as a complete body write")
	}
}

func equalApplicationHeaders(received, actual http.Header) bool {
	for name, values := range actual {
		if !reflect.DeepEqual(received.Values(name), values) { return false }
	}
	for name := range received {
		if _, exists := actual[name]; exists { continue }
		switch http.CanonicalHeaderKey(name) {
		case "Date", "Content-Length", "Transfer-Encoding", "Connection":
		default: return false
		}
	}
	return true
}

func controlItem(client *http.Client, origin string) (string, error) {
	var data struct { Items []struct { ID, Title string } }
	if controlJSON(client, origin, http.MethodGet, "/api/v1/subtitle-library?view=library", nil, nil, &data) != 200 ||
		len(data.Items) != 1 || !itemID.MatchString(data.Items[0].ID) {
		return "", errors.New("fictional item unavailable")
	}
	return data.Items[0].ID, nil
}

func controlJSON(client *http.Client, origin, method, path string, body []byte, headers http.Header, value any) int {
	request, err := http.NewRequest(method, origin+path, bytes.NewReader(body))
	if err != nil { return 0 }
	if headers != nil { request.Header = headers.Clone() }
	request.Header.Set("Origin", origin)
	request.Header.Set("User-Agent", "R06-private-control")
	if body != nil { request.Header.Set("Content-Type", "application/json") }
	response, err := client.Do(request)
	if err != nil { return 0 }
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 65537))
	if err != nil || len(data) > 65536 { return 0 }
	if value != nil && json.Unmarshal(data, value) != nil { return 0 }
	return response.StatusCode
}
