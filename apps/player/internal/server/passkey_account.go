package server

import (
	"errors"
	"net/http"
)

type accountPage struct {
	Passkeys                                       []passkeySummary
	HasPasskeys                                    bool
	Next, AccountURL                               string
	MFAEnabled, OIDC, SAML, OIDCLinked, SAMLLinked bool
}

func (auth *passkeyAuth) account(writer http.ResponseWriter, request *http.Request) {
	page := accountView
	viewer := currentViewer(request)
	passkeys := auth.profiles.passkeyInventory(viewer.ID)
	stored, _ := auth.profiles.byID(viewer.ID)
	data := accountPage{
		Passkeys: passkeys, HasPasskeys: len(passkeys) > 0, Next: "/", AccountURL: auth.origin + "/account",
		MFAEnabled: stored.TOTPSecret != "", OIDC: auth.oidc, SAML: auth.saml, OIDCLinked: stored.OIDCSubject != "", SAMLLinked: stored.SAMLSubject != "",
	}
	if request.URL.Query().Get("setup") == "1" {
		page = passkeyPromptView
	} else if request.URL.Query().Get("mfa") == "required" {
		page = mfaRequiredView
	} else if offer, next, err := passkeyOffer(request); err != nil {
		localizedError(writer, request, err.Error(), http.StatusBadRequest)
		return
	} else if offer {
		page, data.Next = passkeyOfferView, next
	}
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = page.Execute(writer, request, data)
}

func (auth *passkeyAuth) listAPI(writer http.ResponseWriter, request *http.Request) {
	passkeys := auth.profiles.passkeyInventory(currentViewer(request).ID)
	writeJSON(writer, map[string]any{"configured": len(passkeys) > 0, "passkeys": passkeys}, http.StatusOK)
}

func passkeyOffer(request *http.Request) (bool, string, error) {
	values, present := request.URL.Query()["passkey"]
	if !present {
		return false, "/", nil
	}
	if len(values) != 1 || values[0] != "offer" {
		return false, "", errors.New("passkey offer is invalid")
	}
	next := request.URL.Query()["next"]
	if len(next) > 1 || len(next) == 1 && (next[0] == "" || len(next[0]) > 2048 || safeLoginReturn(next[0]) != next[0]) {
		return false, "", errors.New("passkey offer return is invalid")
	}
	if len(next) == 1 {
		return true, next[0], nil
	}
	return true, "/", nil
}
