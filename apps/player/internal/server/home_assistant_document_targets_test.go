package server_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestHomeAssistantDocumentTargetsRemainIndependentThroughPublicApp(t *testing.T) {
	client := newDocumentTargetClient(t)
	first, second := client.claim(t, ""), client.claim(t, "")
	if first.ID == second.ID || first.Claim == second.Claim {
		t.Fatal("R18 separate document claims shared target identity or authority")
	}
	client.poll(t, first.ID, []string{first.Claim}, false)
	client.poll(t, second.ID, []string{second.Claim}, false)
	snapshot := client.snapshot(t)
	if len(snapshot) != 2 || snapshot[first.ID].Position != 1 || snapshot[second.ID].Position != 1 {
		t.Fatal("R18 independent document states did not publish two exact targets")
	}
	client.queueSeek(t, first.ID)
	client.poll(t, second.ID, []string{second.Claim}, false)
	client.poll(t, first.ID, []string{first.Claim}, true)
	client.poll(t, first.ID, []string{first.Claim}, false)
	client.poll(t, second.ID, []string{second.Claim}, false)
}

func TestHomeAssistantDocumentSiblingCannotOverwriteReleaseOrDrain(t *testing.T) {
	client := newDocumentTargetClient(t)
	claim := client.claim(t, "r18-sibling-fixture")
	path := documentTargetsPath + "/" + claim.ID
	client.poll(t, claim.ID, []string{claim.Claim}, false)
	client.queueSeek(t, claim.ID)
	before := client.snapshot(t)
	conflict := client.call(t, http.MethodPost, documentTargetsPath+"/claims", `{"id":"r18-sibling-fixture"}`, nil)
	requireDocumentTargetStatus(t, conflict, http.StatusConflict)
	requireDocumentTargetSnapshot(t, client.snapshot(t), before)
	headers := [][]string{nil, {"wrong-document-claim-000000000"}, {claim.Claim, claim.Claim}, {strings.Repeat("a", 65)}}
	for _, values := range headers {
		rejected := client.call(t, http.MethodPut, path, documentTargetBody(9), values)
		requireDocumentTargetStatus(t, rejected, http.StatusForbidden)
		requireDocumentTargetSnapshot(t, client.snapshot(t), before)
		released := client.call(t, http.MethodPost, path+"/release", `{}`, values)
		requireDocumentTargetStatus(t, released, http.StatusForbidden)
		requireDocumentTargetSnapshot(t, client.snapshot(t), before)
	}
	client.poll(t, claim.ID, []string{claim.Claim}, true)
	client.poll(t, claim.ID, []string{claim.Claim}, false)
}

func TestHomeAssistantDocumentReleaseKeepsCandidateAndRejectsRetiredClaim(t *testing.T) {
	client := newDocumentTargetClient(t)
	first := client.claim(t, "r18-reload-fixture")
	path := documentTargetsPath + "/" + first.ID
	client.poll(t, first.ID, []string{first.Claim}, false)
	release := client.call(t, http.MethodPost, path+"/release", `{}`, []string{first.Claim})
	requireDocumentTargetStatus(t, release, http.StatusNoContent)
	if len(client.snapshot(t)) != 0 {
		t.Fatal("R18 successful document release left a published target")
	}
	second := client.claim(t, first.ID)
	if second.ID != first.ID || second.Claim == first.Claim {
		t.Fatal("R18 reclaim changed automation identity or reused retired authority")
	}
	client.poll(t, second.ID, []string{second.Claim}, false)
	client.queueSeek(t, second.ID)
	before := client.snapshot(t)
	requireDocumentTargetStatus(t, client.call(t, http.MethodPut, path, documentTargetBody(9), []string{first.Claim}), http.StatusForbidden)
	requireDocumentTargetSnapshot(t, client.snapshot(t), before)
	requireDocumentTargetStatus(t, client.call(t, http.MethodPost, path+"/release", `{}`, []string{first.Claim}), http.StatusForbidden)
	requireDocumentTargetSnapshot(t, client.snapshot(t), before)
	client.poll(t, second.ID, []string{second.Claim}, true)
	client.poll(t, second.ID, []string{second.Claim}, false)
}

func TestHomeAssistantDocumentNativeStrictContractRemainsUnchanged(t *testing.T) {
	client := newDocumentTargetClient(t)
	const id = "r18-native-fixture"
	path := documentTargetsPath + "/" + id
	client.poll(t, id, nil, false)
	client.queueSeek(t, id)
	before := client.snapshot(t)
	unknownState := strings.TrimSuffix(documentTargetBody(9), "}") + `,"claim":"unknown-field"}`
	requireDocumentTargetStatus(t, client.call(t, http.MethodPut, path, unknownState, nil), http.StatusBadRequest)
	requireDocumentTargetSnapshot(t, client.snapshot(t), before)
	unknownCommand := `{"command":"seek","position":9,"claim":"unknown-field"}`
	requireDocumentTargetStatus(t, client.call(t, http.MethodPost, path+"/commands", unknownCommand, nil), http.StatusBadRequest)
	requireDocumentTargetSnapshot(t, client.snapshot(t), before)
	client.poll(t, id, nil, true)
	client.poll(t, id, nil, false)
}
