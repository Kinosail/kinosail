package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"
)

const r16InitialText = "1\n00:00:01,000 --> 00:00:02,000\nR16 original line.\n"
const r16ReplacementText = "1\n00:00:01,000 --> 00:00:02,000\nR16 corrected line.\n\n"
const r16ReplayText = "1\n00:00:01,000 --> 00:00:02,000\nR16 replay witness.\n\n"

type r16Authority uint8

const (
	r16Owner r16Authority = iota
	r16Anonymous
	r16MissingCSRF
	r16WrongCSRF
)

func r16Request(t *testing.T, f r16PublicFixture, method, route string, body []byte, authority r16Authority) r16Response {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	response := f.Request(ctx, method, route, body, authority)
	if !response.Complete {
		t.Fatal("public freshness response body did not complete and close")
	}
	return response
}

func r16DecodeResponse(t *testing.T, response r16Response, target any) {
	t.Helper()
	if response.Status != http.StatusOK || len(response.Body) > 65536 {
		t.Fatal("public freshness JSON response was not bounded and successful")
	}
	decoder := json.NewDecoder(bytes.NewReader(response.Body))
	if decoder.Decode(target) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		t.Fatal("public freshness JSON response was malformed")
	}
}

func r16ReadCounts(t *testing.T, f r16PublicFixture) r16Counts {
	t.Helper()
	var counts struct {
		Total *int `json:"total"`
		Ready *int `json:"ready"`
		Wanted *int `json:"wanted"`
		Pending *int `json:"pending"`
		Unavailable *int `json:"unavailable"`
	}
	r16DecodeResponse(t, r16Request(t, f, http.MethodGet, "/api/v1/subtitle-library?view=summary", nil, r16Owner), &counts)
	for _, value := range []*int{counts.Total, counts.Ready, counts.Wanted, counts.Pending, counts.Unavailable} {
		if value == nil || *value < 0 {
			t.Fatal("actual summary omitted or invalidated a coverage total")
		}
	}
	return r16Counts{*counts.Total, *counts.Ready, *counts.Wanted, *counts.Pending, *counts.Unavailable}
}

func r16ReadRow(t *testing.T, f r16PublicFixture) r16Row {
	t.Helper()
	var data struct { Items []r16Row `json:"items"` }
	r16DecodeResponse(t, r16Request(t, f, http.MethodGet, "/api/v1/subtitle-library?view=library", nil, r16Owner), &data)
	if len(data.Items) != 1 {
		t.Fatal("actual catalogue did not expose exactly one fictional subtitle row")
	}
	return data.Items[0]
}

func r16HistoryRows(t *testing.T, f r16PublicFixture) []r16History {
	t.Helper()
	var data struct { History []r16History `json:"history"` }
	r16DecodeResponse(t, r16Request(t, f, http.MethodGet, "/api/v1/subtitle-library?view=history", nil, r16Owner), &data)
	if data.History == nil {
		t.Fatal("actual History projection omitted its array")
	}
	return data.History
}

func r16AssertHistoryEmpty(t *testing.T, f r16PublicFixture) {
	t.Helper()
	if len(r16HistoryRows(t, f)) != 0 {
		t.Fatal("actual History effect count did not match admitted work")
	}
}

func r16AssertManualHistory(t *testing.T, f r16PublicFixture, count int) {
	t.Helper()
	rows := r16HistoryRows(t, f)
	if len(rows) != count { t.Fatal("actual manual History count did not match admitted Saves") }
	for _, row := range rows {
		if row.ID != f.ItemID() || row.Action != "updated" || row.Language != "en" || row.Source != "external" || row.Reason != "manual" || !row.Available {
			t.Fatal("actual manual History did not match the fictional item and Save")
		}
		if _, err := time.Parse(time.RFC3339, row.Changed); err != nil {
			t.Fatal("actual manual History did not expose a valid completion time")
		}
	}
}

func r16AssertReadyCounts(t *testing.T, counts r16Counts) {
	t.Helper()
	if counts != (r16Counts{Total: 1, Ready: 1}) { t.Fatal("fictional sidecar was not exactly ready before the public control") }
}

func r16AssertCountsEqual(t *testing.T, before, after r16Counts) {
	t.Helper()
	if before != after { t.Fatal("public control did not retain all five coverage totals") }
}

func r16AssertInitialRow(t *testing.T, row r16Row, id string) {
	t.Helper()
	if row.ID != id || !row.Ready || row.Source != "" || row.Installed != "" || row.Restorable || row.Frozen {
		t.Fatal("fictional row did not begin ready without a ledger installation or recovery")
	}
}

func r16AssertSavedRow(t *testing.T, row r16Row, id string) {
	t.Helper()
	if row.ID != id || !row.Ready || row.Source != "external" || !row.Frozen || !row.Restorable || row.Installed == "" {
		t.Fatal("real Save did not update source installation and recovery projection")
	}
}

func r16AssertFiles(t *testing.T, files r16Files, current, recovery string, exists bool) {
	t.Helper()
	if !files.CurrentExists || string(files.Current) != current || string(files.Recovery) != recovery || files.RecoveryExists != exists {
		t.Fatal("actual owned sidecar or recovery bytes did not match completed work")
	}
}

func r16AssertFilesEqual(t *testing.T, before, after r16Files) {
	t.Helper()
	if before.CurrentExists != after.CurrentExists || !bytes.Equal(before.Current, after.Current) || !bytes.Equal(before.Recovery, after.Recovery) || before.RecoveryExists != after.RecoveryExists {
		t.Fatal("no-write control changed actual sidecar or recovery bytes")
	}
}

func r16ReadProvider(t *testing.T, f r16PublicFixture) r16Provider {
	t.Helper()
	var data struct { Providers []r16Provider `json:"providers"` }
	r16DecodeResponse(t, r16Request(t, f, http.MethodGet, "/api/v1/subtitle-library?view=summary", nil, r16Owner), &data)
	for _, provider := range data.Providers { if provider.Name == "SubDL" { return provider } }
	t.Fatal("actual owned provider health projection was missing")
	return r16Provider{}
}

func r16AssertConnectedProvider(t *testing.T, provider r16Provider) {
	t.Helper()
	if !provider.Configured || !provider.Tested || provider.State != "connected" || provider.LastSuccess == "" {
		t.Fatal("actual provider health did not become connected")
	}
	if provider.Remaining == nil || *provider.Remaining != 17 {
		t.Fatal("actual provider quota did not match the owned response")
	}
}

func r16Save(t *testing.T, f r16PublicFixture, text string) {
	t.Helper()
	route := "/api/v1/subtitle-library/" + url.PathEscape(f.ItemID())
	var current struct { Fingerprint string `json:"fingerprint"` }
	r16DecodeResponse(t, r16Request(t, f, http.MethodGet, route+"/inspect?language=en", nil, r16Owner), &current)
	if len(current.Fingerprint) != 64 { t.Fatal("actual current subtitle fingerprint was missing") }
	body, err := json.Marshal(struct {
		Language string `json:"language"`
		Fingerprint string `json:"fingerprint"`
		Text string `json:"text"`
		AutomaticSync bool `json:"automaticSync"`
	}{"en", current.Fingerprint, text, false})
	if err != nil { t.Fatal("fictional manual Save JSON could not be encoded") }
	var preview struct { Proposed *struct { Cues []struct { Text string `json:"text"` } `json:"cues"` } `json:"proposed"` }
	r16DecodeResponse(t, r16Request(t, f, http.MethodPost, route+"/preview", body, r16Owner), &preview)
	if preview.Proposed == nil || len(preview.Proposed.Cues) != 1 {
		t.Fatal("real manual Save preview was not eligible")
	}
	if preview.Proposed.Cues[0].Text != r16Dialogue(text) {
		t.Fatal("real manual Save preview did not contain the proposed fictional text")
	}
	var saved struct { Fingerprint string `json:"fingerprint"` }
	r16DecodeResponse(t, r16Request(t, f, http.MethodPost, route+"/apply", body, r16Owner), &saved)
	if saved.Fingerprint == current.Fingerprint { t.Fatal("actual manual Save did not change its current fingerprint") }
}

func r16OpenStream(t *testing.T, f r16PublicFixture, cursor string) r16EventStream {
	t.Helper()
	stream := f.OpenEvents(t.Context(), cursor, 5*time.Second)
	if stream.Status() != http.StatusOK || stream.Headers().Get("Content-Type") != "text/event-stream" || stream.Headers().Get("Cache-Control") != "no-store" {
		t.Fatal("actual authenticated event stream was not admitted")
	}
	return stream
}

func r16NextSubtitleEvent(t *testing.T, stream r16EventStream) r16Event {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	for range 32 {
		frame, err := stream.Next(ctx)
		if err != nil { t.Fatal("actual subtitle event did not arrive within the owned bound") }
		if frame.Type != "subtitles.updated" { continue }
		if len(frame.Data) > 1024 { t.Fatal("actual subtitle event exceeded its safe envelope bound") }
		var event r16Event
		decoder := json.NewDecoder(bytes.NewReader(frame.Data))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&event) != nil || decoder.Decode(&struct{}{}) != io.EOF { t.Fatal("actual subtitle event envelope was malformed") }
		if frame.ID != strconv.FormatUint(event.ID, 10) { t.Fatal("actual SSE cursor did not match its event envelope") }
		return event
	}
	t.Fatal("actual subtitle event was not found in the bounded stream")
	return r16Event{}
}

func r16AssertSubtitleEvent(t *testing.T, event r16Event) {
	t.Helper()
	if event.ID == 0 || event.Type != "subtitles.updated" || event.Resource != "/api/v1/subtitle-library" || event.At.IsZero() {
		t.Fatal("actual subtitle event contained invalid public metadata")
	}
}

func r16SettleStream(t *testing.T, stream r16EventStream) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 5*time.Second)
	defer cancel()
	if !stream.StopAndJoin(ctx) { t.Error("owned event stream did not close and join") }
}

func r16SettleFixture(t *testing.T, f r16PublicFixture) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 5*time.Second)
	defer cancel()
	if !f.StopAndJoin(ctx) { t.Error("owned freshness fixture did not settle and close") }
}
