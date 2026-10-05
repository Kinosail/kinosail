package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"reflect"
	"strings"
)

var (
	restoreItemID = regexp.MustCompile(`^[a-f0-9]{16}$`)
	restoreOperationID = regexp.MustCompile(`^[a-f0-9]{64}$`)
	restoreMutationPath = regexp.MustCompile(`^/api/v1/subtitle-library/([a-f0-9]{16})/restore$`)
)

func (target *restoreTarget) seedFixedRoutes() {
	for _, path := range []string{"/setup", "/login", "/account", "/account/mfa/enable", "/?view=library",
		"/api/v1/subtitle-library?view=library", "/api/v1/subtitle-library?view=history",
		"/api/v1/subtitle-operations", "/__r06_restore/witness", "/__r06_restore/release"} {
		target.addRouteLocked(path)
	}
}
func (target *restoreTarget) addRouteLocked(path string) {
	route, query, _ := strings.Cut(path, "?")
	target.routes[path] = url.URL{Scheme: "https", Host: target.authority, Path: route, RawQuery: query}
}
func (target *restoreTarget) registerCatalog(response *restoreCapture) bool {
	var catalog struct { Items []struct { ID string } }
	if response.status != http.StatusOK || response.overflow || response.body.Len() > privateResponseLimit ||
		json.Unmarshal(response.body.Bytes(), &catalog) != nil { return false }
	if len(catalog.Items) != 1 { return false }
	return target.registerItem(catalog.Items[0].ID)
}
func (target *restoreTarget) registerItem(id string) bool {
	if !restoreItemID.MatchString(id) { return false }
	target.mu.Lock()
	defer target.mu.Unlock()
	if target.item != "" && target.item != id { return false }
	target.item = id
	base := "/api/v1/subtitle-library/" + id
	for _, suffix := range []string{"/inspect?language=en", "/export?language=en&format=srt", "/preview", "/apply", "/restore"} {
		target.addRouteLocked(base + suffix)
	}
	return true
}
func (target *restoreTarget) registerReceipt(receipt restorePublicReceipt) bool {
	target.mu.Lock()
	defer target.mu.Unlock()
	if target.item == "" || receipt.Item != target.item || receipt.Action != "restore" || receipt.State != "prepared" { return false }
	if !restoreOperationID.MatchString(receipt.ID) || target.receipt != "" && target.receipt != receipt.ID { return false }
	target.receipt = receipt.ID
	target.addRouteLocked("/api/v1/subtitle-operations/" + receipt.ID)
	return true
}
func (target *restoreTarget) receiptID() string {
	target.mu.Lock(); defer target.mu.Unlock()
	return target.receipt
}
func (f *restoreRig) inspectPath() string { return "/api/v1/subtitle-library/" + f.item + "/inspect?language=en" }
func (f *restoreRig) exportPath() string { return "/api/v1/subtitle-library/" + f.item + "/export?language=en&format=srt" }

func equalRestoreApplicationHeaders(received, actual http.Header) bool {
	for name, values := range actual { if !reflect.DeepEqual(received.Values(name), values) { return false } }
	for name := range received { if _, exists := actual[name]; !exists && !restoreTransportHeader(name) { return false } }
	return true
}
func restoreTransportHeader(name string) bool {
	switch http.CanonicalHeaderKey(name) {
	case "Date", "Content-Length", "Transfer-Encoding", "Connection": return true
	default: return false
	}
}
