package server

import (
	"io"
	"mime"
	"net/http"
	"time"

	supporterengine "github.com/MikeO7/kinosail/packages/supporter"
)

type supporterGrantView struct {
	ID, Eyebrow, FamilyTitle, Intro, PurchaseLabel, SupportURL, SupportedSince, ExpiresAt string
	Status                                                                                supporterGrantStatus
	Tiers                                                                                 []supporterTierView
	CurrentBadge                                                                          supporterTierView
	Available                                                                             bool
}

type supporterPageData struct {
	Status                             supporterStatus
	LivingStandard                     supporterGrantView
	PatronOrder                        supporterGrantView
	Error                              string
	Masterwork                         supporterTierView
	MasterworkActive                   bool
	HasAny, HasAnyFleet, HasMasterwork bool
}

func supporterDate(value string) string {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return ""
	}
	return parsed.UTC().Format("Jan 2, 2006")
}

func supporterGrantPage(status supporterGrantStatus, collectedLevel int, id, eyebrow, intro, purchaseLabel, supportURL string) supporterGrantView {
	familyTitle := "Patron Orders"
	if id == livingStandardFamily {
		familyTitle = "Living Standards"
	}
	return supporterGrantView{
		ID: id, Eyebrow: eyebrow, FamilyTitle: familyTitle, Intro: intro, PurchaseLabel: purchaseLabel, SupportURL: supportURL, Status: status,
		Tiers: supporterBadgeTiers(id, status, collectedLevel), CurrentBadge: supporterBadge(id, status.Rank),
		SupportedSince: supporterDate(status.SupportedSince), ExpiresAt: supporterDate(status.ExpiresAt),
		Available: status.Active || status.Archived,
	}
}

func (program *supporterProgram) pageData(message string) supporterPageData {
	status := program.status()
	living := supporterGrantPage(status.LivingStandard, status.BadgeCase.LivingLevel, livingStandardFamily, "Monthly support · Recommended", "Living Standards are complete badges with signal geometry, service marks, and an honest active-through date.", "Choose monthly support", status.SupportURL)
	patron := supporterGrantPage(status.PatronOrder, status.BadgeCase.PatronLevel, patronOrderFamily, "One-time support · Permanent", "Patron Orders are permanent enamel badges. Each level has its own complete silhouette and caption emblem.", "Choose one-time support", status.SupportURL)
	masterwork := supporterBadge(livingStandardFamily, status.BadgeCase.MasterworkLevel)
	if status.BadgeCase.MasterworkEarned {
		masterwork.BadgeName = perfectSyncNames[status.BadgeCase.MasterworkLevel-1]
	}
	return supporterPageData{
		Status: status, LivingStandard: living, PatronOrder: patron, Masterwork: masterwork, MasterworkActive: status.BadgeCase.MasterworkActive, Error: message,
		HasAny:        living.Available || patron.Available,
		HasAnyFleet:   status.LivingStandard.Collection != nil || status.PatronOrder.Collection != nil,
		HasMasterwork: status.BadgeCase.MasterworkEarned,
	}
}

func registerSupporter(mux *http.ServeMux, auth *authentication, program *supporterProgram) {
	mux.Handle("GET /supporter", auth.owner(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_ = supporterView.Execute(writer, request, program.pageData(""))
	})))
	mux.Handle("POST /supporter/activate", auth.owner(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		request.Body = http.MaxBytesReader(writer, request.Body, 1024)
		if !formEncoded(request) || request.URL.RawQuery != "" || request.ParseForm() != nil || !onlyFormKeys(request.PostForm, "key", "recognitionName") {
			writeSupporterFormError(writer, request, program, "The supporter details could not be read.")
			return
		}
		key, keyOK := oneValue(request.PostForm, "key", 128)
		name, nameOK := optionalValue(request.PostForm, "recognitionName", 80)
		if !keyOK || !nameOK {
			writeSupporterFormError(writer, request, program, "The supporter details could not be read.")
			return
		}
		if err := program.activate(request.Context(), key, name); err != nil {
			writeSupporterFormError(writer, request, program, err.Error())
			return
		}
		http.Redirect(writer, request, "/supporter", http.StatusSeeOther)
	})))
}

func writeSupporterFormError(writer http.ResponseWriter, request *http.Request, program *supporterProgram, message string) {
	writer.WriteHeader(http.StatusBadRequest)
	_ = supporterView.Execute(writer, request, program.pageData(message))
}

func registerSupporterAPI(mux *http.ServeMux, auth *authentication, program *supporterProgram) {
	mux.Handle("GET /api/v1/supporter", auth.owner(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, program.status(), http.StatusOK)
	})))
	mux.Handle("POST /api/v1/supporter/activate", auth.owner(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		mediaType, _, contentTypeErr := mime.ParseMediaType(request.Header.Get("Content-Type"))
		if contentTypeErr != nil || mediaType != "application/json" || request.URL.RawQuery != "" {
			apiError(writer, errInvalidSupporterRequest, http.StatusBadRequest)
			return
		}
		key, name, ok := readSupporterActivationJSON(writer, request)
		if !ok {
			return
		}
		if err := program.activate(request.Context(), key, name); err != nil {
			apiError(writer, err, http.StatusBadRequest)
			return
		}
		writeJSON(writer, program.status(), http.StatusOK)
	})))
	mux.Handle("GET /api/v1/supporter/certificates/{family}", auth.owner(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.RawQuery != "" {
			apiError(writer, errInvalidSupporterRequest, http.StatusBadRequest)
			return
		}
		program.writeShareCertificate(writer, request.PathValue("family"))
	})))
}

var errInvalidSupporterRequest = &supporterRequestError{}

type supporterRequestError struct{}

func (*supporterRequestError) Error() string { return "invalid supporter request" }

func readSupporterActivationJSON(writer http.ResponseWriter, request *http.Request) (string, string, bool) {
	request.Body = http.MaxBytesReader(writer, request.Body, 1024)
	record, err := io.ReadAll(io.LimitReader(request.Body, 1025))
	input, inputErr := supporterengine.ParseActivationJSON(record)
	if err != nil || len(record) > 1024 || inputErr != nil {
		apiError(writer, errInvalidSupporterRequest, http.StatusBadRequest)
		return "", "", false
	}
	name := ""
	if input.RecognitionName != nil {
		name = *input.RecognitionName
	}
	return input.Key, name, true
}
