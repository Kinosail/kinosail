package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"testing"
)

func bootstrapControl(t *testing.T, target *ownedTarget, mode string, prepared bool) (*fixture, *http.Client, string, []byte, http.Header) {
	t.Helper()
	private := os.Getenv("R06_SAVE_PRIVATE_ROOT")
	root, err := os.MkdirTemp(private, "r06-control-")
	if err != nil {
		t.Fatal("private fixture directory unavailable")
	}
	f, err := newFixture(target, root, mode)
	if err != nil {
		t.Fatal("real Server fixture construction failed")
	}
	t.Cleanup(func() { f.controlStop(t) })
	client, csrf, err := enrollOwner(t, target, f)
	if err != nil {
		t.Fatal("actual TLS Owner/MFA/CSRF prerequisite failed")
	}
	t.Cleanup(client.CloseIdleConnections)
	id, err := controlItem(t, client, target, f)
	if err != nil {
		t.Fatal("public fictional item prerequisite failed")
	}
	base := "/api/v1/subtitle-library/" + id
	input := previewControl(t, client, target, f, base, csrf)
	headers := http.Header{"X-Kinosail-CSRF": {csrf}}
	if prepared {
		prepareControl(t, client, target, f, id, headers)
	}
	return f, client, base + "/apply", input, headers
}

func previewControl(t *testing.T, client *http.Client, target *ownedTarget, f *fixture, base, csrf string) []byte {
	t.Helper()
	var before struct{ Fingerprint string }
	if controlJSON(t, client, target, f, http.MethodGet, base+"/inspect?language=en", nil, nil, &before) != http.StatusOK {
		t.Fatal("public inspection prerequisite failed")
	}
	input, err := json.Marshal(map[string]any{
		"language": "en", "fingerprint": before.Fingerprint, "text": savedSRT,
		"role": "translation", "automaticSync": false, "removeCredits": false, "mergeRepeated": false,
	})
	if err != nil {
		t.Fatal("fictional input encoding failed")
	}
	headers := http.Header{"X-Kinosail-CSRF": {csrf}}
	if controlJSON(t, client, target, f, http.MethodPost, base+"/preview", input, headers, nil) != http.StatusOK {
		t.Fatal("real public preview prerequisite failed")
	}
	return input
}

func prepareControl(t *testing.T, client *http.Client, target *ownedTarget, f *fixture, id string, headers http.Header) {
	t.Helper()
	preparation, err := json.Marshal(map[string]string{"action": "apply", "item": id})
	if err != nil {
		t.Fatal("fictional preparation encoding failed")
	}
	var receipt struct{ ID string }
	status := controlJSON(t, client, target, f, http.MethodPost, "/api/v1/subtitle-operations", preparation, headers, &receipt)
	if status != http.StatusCreated || !operationID.MatchString(receipt.ID) {
		t.Fatal("real public preparation prerequisite failed")
	}
	headers.Set("X-Kinosail-Operation", receipt.ID)
}

func controlItem(t *testing.T, client *http.Client, target *ownedTarget, f *fixture) (string, error) {
	var data struct{ Items []struct{ ID, Title string } }
	if controlJSON(t, client, target, f, http.MethodGet, "/api/v1/subtitle-library?view=library", nil, nil, &data) != http.StatusOK {
		return "", errors.New("fictional item unavailable")
	}
	if len(data.Items) != 1 {
		return "", errors.New("fictional item unavailable")
	}
	id := data.Items[0].ID
	if !itemID.MatchString(id) || !target.ownsItem(id) {
		return "", errors.New("fictional item unavailable")
	}
	return id, nil
}

func controlJSON(t *testing.T, client *http.Client, target *ownedTarget, f *fixture, method, path string, body []byte, headers http.Header, value any) (status int) {
	diagnostic := &controlDiagnostic{stage: controlRoute(path), decoded: value == nil}
	defer func() { diagnostic.emit(t, status) }()
	endpoint, err := target.endpoint(path)
	if err != nil {
		return 0
	}
	request, err := http.NewRequestWithContext(t.Context(), method, endpoint, bytes.NewReader(body))
	if err != nil {
		return 0
	}
	if headers != nil {
		request.Header = headers.Clone()
	}
	request.Header.Set("Origin", target.origin)
	request.Header.Set("User-Agent", "R06-private-control")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if !target.admittedRequest(request) {
		return 0
	}
	response, err := client.Do(request)
	diagnostic.responseStatus, diagnostic.transport = controlResponseStatus(response), err == nil
	if err != nil {
		f.closeFailedPrivateResponse(response)
		return 0
	}
	return decodeControlResponse(response, value, diagnostic)
}

func decodeControlResponse(response *http.Response, value any, diagnostic *controlDiagnostic) int {
	data, err := readPrivateResponse(response)
	diagnostic.read, diagnostic.bounded = err == nil, len(data) <= privateResponseLimit
	if err != nil || len(data) > privateResponseLimit {
		return 0
	}
	if value != nil && json.Unmarshal(data, value) != nil {
		return 0
	}
	diagnostic.decoded = true
	return response.StatusCode
}
