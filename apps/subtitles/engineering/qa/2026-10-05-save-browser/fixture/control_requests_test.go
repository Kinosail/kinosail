package main

import (
 "bytes"
 "encoding/json"
 "errors"
 "net/http"
 "os"
 "testing"
)

func bootstrapControl(t *testing.T, mode string, prepared bool) (*fixture, *http.Client, string, []byte, http.Header) {
 t.Helper()
 private := os.Getenv("R06_SAVE_PRIVATE_ROOT")
 root, err := os.MkdirTemp(private, "r06-control-")
 if err != nil { t.Fatal("private fixture directory unavailable") }
 f, err := newFixture(root, mode)
 if err != nil { t.Fatal("real Server fixture construction failed") }
 t.Cleanup(func() { f.controlStop(t) })
 client, csrf, err := enrollOwner(t, f)
 if err != nil { t.Fatal("actual TLS Owner/MFA/CSRF prerequisite failed") }
 t.Cleanup(client.CloseIdleConnections)
 id, err := controlItem(t, client, f)
 if err != nil { t.Fatal("public fictional item prerequisite failed") }
 base := "/api/v1/subtitle-library/"+id
 input := previewControl(t, client, f, base, csrf)
 headers := http.Header{"X-Kinosail-CSRF": {csrf}}
 if prepared { prepareControl(t, client, f, id, headers) }
 return f, client, base+"/apply", input, headers
}

func previewControl(t *testing.T, client *http.Client, f *fixture, base, csrf string) []byte {
 t.Helper()
 var before struct{ Fingerprint string }
 if controlJSON(t, client, f, http.MethodGet, base+"/inspect?language=en", nil, nil, &before) != http.StatusOK {
  t.Fatal("public inspection prerequisite failed")
 }
 input, err := json.Marshal(map[string]any{
  "language": "en", "fingerprint": before.Fingerprint, "text": savedSRT,
  "role": "translation", "automaticSync": false, "removeCredits": false, "mergeRepeated": false,
 })
 if err != nil { t.Fatal("fictional input encoding failed") }
 headers := http.Header{"X-Kinosail-CSRF": {csrf}}
 if controlJSON(t, client, f, http.MethodPost, base+"/preview", input, headers, nil) != http.StatusOK {
  t.Fatal("real public preview prerequisite failed")
 }
 return input
}

func prepareControl(t *testing.T, client *http.Client, f *fixture, id string, headers http.Header) {
 t.Helper()
 preparation, err := json.Marshal(map[string]string{"action": "apply", "item": id})
 if err != nil { t.Fatal("fictional preparation encoding failed") }
 var receipt struct{ ID string }
 status := controlJSON(t, client, f, http.MethodPost, "/api/v1/subtitle-operations", preparation, headers, &receipt)
 if status != http.StatusCreated || !operationID.MatchString(receipt.ID) {
  t.Fatal("real public preparation prerequisite failed")
 }
 headers.Set("X-Kinosail-Operation", receipt.ID)
}

func controlItem(t *testing.T, client *http.Client, f *fixture) (string, error) {
 var data struct{ Items []struct{ ID, Title string } }
 if controlJSON(t, client, f, http.MethodGet, "/api/v1/subtitle-library?view=library", nil, nil, &data) != http.StatusOK {
  return "", errors.New("fictional item unavailable")
 }
 if len(data.Items) != 1 { return "", errors.New("fictional item unavailable") }
 id := data.Items[0].ID
 if !itemID.MatchString(id) { return "", errors.New("fictional item unavailable") }
 return id, nil
}

func controlJSON(t *testing.T, client *http.Client, f *fixture, method, path string, body []byte, headers http.Header, value any) (status int) {
 diagnostic := &controlDiagnostic{stage: controlRoute(path), decoded: value == nil}
 defer func() { diagnostic.emit(t, status) }()
 request, err := f.privateRequest(t.Context(), method, path, bytes.NewReader(body))
 if err != nil { return 0 }
 if headers != nil { request.Header = headers.Clone() }
 request.Header.Set("Origin", f.origin)
 request.Header.Set("User-Agent", "R06-private-control")
 if body != nil { request.Header.Set("Content-Type", "application/json") }
 response, err := doPrivate(client, request)
 diagnostic.responseStatus, diagnostic.transport = controlResponseStatus(response), err == nil
 if err != nil { return 0 }
 data, err := readPrivateResponse(response)
 diagnostic.read, diagnostic.bounded = err == nil, len(data) <= privateResponseLimit
 if err != nil || len(data) > privateResponseLimit { return 0 }
 if value != nil && json.Unmarshal(data, value) != nil { return 0 }
 diagnostic.decoded = true
 return response.StatusCode
}
