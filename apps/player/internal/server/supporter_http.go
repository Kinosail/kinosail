package server

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"time"

	"github.com/MikeO7/kinosail/packages/httpguard"
	supporterengine "github.com/MikeO7/kinosail/packages/supporter"
)

type supporterBadgeArt struct {
	Family, FamilyName, Tier, Name, Title string
	Rank, ServiceCount                    int
	Active, Expired                       bool
}

type supporterLevelView struct {
	supporterBadgeArt
	Detail, State string
}

type supporterOwnedBadgeView struct {
	supporterBadgeArt
	RecognitionName, Since, Until, CertificateURL string
	FleetEdition                                  string
	Founding, CompleteFleet                       bool
	FleetAppCount                                 int
	ServiceMarks                                  []int
}

type supporterPageData struct {
	Status                     supporterStatus
	Living, Patron             *supporterOwnedBadgeView
	LivingLevels, PatronLevels []supporterLevelView
	Masterwork                 *supporterBadgeArt
	HasBadges                  bool
	Display                    string
	Error                      string
}

func supporterDate(value string) string {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return ""
	}
	return parsed.UTC().Format("Jan 2, 2006")
}

func supporterOwnedBadge(status *supporterBadgeStatus) *supporterOwnedBadgeView {
	if status == nil {
		return nil
	}
	familyName, certificateURL, title := "Patron Order", "/api/v1/supporter/certificates/patron-order.svg", patronBadgeTitles[status.Rank-1]
	if status.Family == livingStandardFamily {
		familyName, certificateURL, title = "Living Standard", "/api/v1/supporter/certificates/living-standard.svg", livingBadgeTitles[status.Rank-1]
	}
	view := &supporterOwnedBadgeView{
		supporterBadgeArt: supporterBadgeArt{Family: status.Family, FamilyName: familyName, Tier: status.Tier, Name: status.Name, Title: title, Rank: status.Rank, ServiceCount: len(status.ServiceMarks), Active: status.Active, Expired: status.Expired},
		RecognitionName:   status.RecognitionName, Since: supporterDate(status.SupportedSince), Until: supporterDate(status.ExpiresAt), CertificateURL: certificateURL,
		Founding: status.Founding, CompleteFleet: status.Collection != nil && status.Collection.ID == "complete-fleet", ServiceMarks: status.ServiceMarks,
	}
	if status.Collection != nil {
		view.FleetEdition, view.FleetAppCount = status.Collection.Edition, len(status.Collection.AppIDs)
	}
	return view
}

var (
	patronBadgeTitles  = []string{"First Token", "Crew Coin", "Navigator Compass", "Patron Shield", "Steward Medal", "Lighthouse Crest", "Commodore Order", "Admiral Star", "North Star Sovereign", "Legacy Seal"}
	livingBadgeTitles  = []string{"Wake Light", "Crew Signal", "Navigator Pulse", "Helmsman Orbit", "Beacon Watch", "Lighthouse Standard", "Commodore Vanguard", "Admiral Array", "North Star Crown", "Legacy Constellation"}
	masterworkTitles   = []string{"Wakebound Crest", "Twin Current", "Navigator Union", "Helmsman's Accord", "Beacon Pact", "Lighthouse Ascendant", "Commodore Full Sail", "Admiral Full Sail", "North Star Dominion", "Sovereign Full Sail"}
	patronBadgeDetails = []string{"A quiet enamel sail, marking the beginning.", "A stronger edge for a place in the crew.", "A compass frame for the journey ahead.", "A shield around the Kinosail sail.", "A ribboned mark of lasting stewardship.", "A raised crest with a guiding star.", "Laurel leaves and a command ribbon.", "A broad star with full admiralty detail.", "A crowned polar star above the sail.", "The complete sail, laurel, and constellation."}
	livingBadgeDetails = patronBadgeDetails
)

func supporterLevels(family string, owned *supporterBadgeStatus, collectedLevel int) []supporterLevelView {
	titles, details, familyName := patronBadgeTitles, patronBadgeDetails, "Patron Order"
	if family == livingStandardFamily {
		titles, details, familyName = livingBadgeTitles, livingBadgeDetails, "Living Standard"
	}
	levels := make([]supporterLevelView, len(supporterTiers))
	for index, tier := range supporterTiers {
		state := "Available"
		active, expired, serviceCount := false, false, 0
		if collectedLevel >= index+1 {
			if owned != nil && owned.Rank == index+1 {
				active, expired, serviceCount = owned.Active, owned.Expired, len(owned.ServiceMarks)
			}
			switch {
			case expired:
				state = "Archived"
			case owned == nil || owned.Rank != index+1:
				state = "Collected"
			default:
				state = "Your badge"
			}
		}
		levels[index] = supporterLevelView{supporterBadgeArt: supporterBadgeArt{Family: family, FamilyName: familyName, Tier: tier, Name: supporterName(tier), Title: titles[index], Rank: index + 1, ServiceCount: serviceCount, Active: active, Expired: expired}, Detail: details[index], State: state}
	}
	return levels
}

func (program *supporterProgram) pageData(message string) supporterPageData {
	status := program.status()
	var masterwork *supporterBadgeArt
	if status.BadgeCase.MasterworkEarned {
		level := status.BadgeCase.MasterworkLevel
		masterwork = &supporterBadgeArt{Family: "masterwork", FamilyName: "Full Sail", Tier: supporterTiers[level-1], Name: supporterName(supporterTiers[level-1]), Title: masterworkTitles[level-1], Rank: level, Active: status.BadgeCase.MasterworkActive, Expired: !status.BadgeCase.MasterworkActive}
	}
	return supporterPageData{Status: status, Living: supporterOwnedBadge(status.LivingStandard), Patron: supporterOwnedBadge(status.PatronOrder), Masterwork: masterwork, LivingLevels: supporterLevels(livingStandardFamily, status.LivingStandard, status.BadgeCase.LivingLevel), PatronLevels: supporterLevels(patronOrderFamily, status.PatronOrder, status.BadgeCase.PatronLevel), HasBadges: status.PatronOrder != nil || status.LivingStandard != nil, Display: program.settings.supporterDisplay(), Error: message}
}

var supporterView = newLocalizedTemplate("supporter", supporterHTML)

func registerSupporter(mux *http.ServeMux, auth *authentication, program *supporterProgram) {
	registerSupporterDisplay(mux, auth, program)
	mux.Handle("GET /supporter", auth.owner(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_ = supporterView.Execute(writer, request, program.pageData(""))
	})))
	mux.Handle("POST /supporter/activate", auth.owner(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		request.Body = http.MaxBytesReader(writer, request.Body, 2048)
		if !httpguard.FormEncoded(request) || request.URL.RawQuery != "" || request.ParseForm() != nil || !httpguard.OnlyFormKeys(request.PostForm, "key", "recognitionName") {
			writer.WriteHeader(http.StatusBadRequest)
			_ = supporterView.Execute(writer, request, program.pageData("The supporter key could not be read."))
			return
		}
		key, keyOK := httpguard.RequiredValue(request.PostForm, "key", 128)
		name, nameOK := httpguard.OptionalValue(request.PostForm, "recognitionName", 80)
		if !keyOK || !nameOK {
			writer.WriteHeader(http.StatusBadRequest)
			_ = supporterView.Execute(writer, request, program.pageData("The supporter details could not be read."))
			return
		}
		if err := program.activate(request.Context(), key, name); err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			_ = supporterView.Execute(writer, request, program.pageData(err.Error()))
			return
		}
		http.Redirect(writer, request, "/supporter#thank-you", http.StatusSeeOther)
	})))
}

func registerSupporterAPI(mux *http.ServeMux, auth *authentication, program *supporterProgram) {
	mux.Handle("GET /api/v1/supporter", auth.owner(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writeJSON(writer, program.status(), http.StatusOK) })))
	mux.Handle("GET /api/v1/supporter/certificate.svg", auth.owner(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.RawQuery != "" {
			apiError(writer, errInvalidSupporterCertificateRequest, http.StatusBadRequest)
			return
		}
		program.writeCurrentCertificate(writer)
	})))
	certificate := func(family string) http.Handler {
		return auth.owner(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.URL.RawQuery != "" {
				apiError(writer, errInvalidSupporterCertificateRequest, http.StatusBadRequest)
				return
			}
			program.writeCertificate(writer, family)
		}))
	}
	mux.Handle("GET /api/v1/supporter/certificates/patron-order.svg", certificate(patronOrderFamily))
	mux.Handle("GET /api/v1/supporter/certificates/living-standard.svg", certificate(livingStandardFamily))
	mux.Handle("POST /api/v1/supporter/activate", auth.owner(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		key, recognitionName, ok := readSupporterActivation(writer, request)
		if !ok {
			return
		}
		if err := program.activate(request.Context(), key, recognitionName); err != nil {
			apiError(writer, err, http.StatusBadRequest)
			return
		}
		writeJSON(writer, program.status(), http.StatusOK)
	})))
}

var errInvalidSupporterCertificateRequest = errors.New("supporter certificate request must not contain parameters")

func readSupporterActivation(writer http.ResponseWriter, request *http.Request) (string, string, bool) {
	mediaType, _, mediaErr := mime.ParseMediaType(request.Header.Get("Content-Type"))
	request.Body = http.MaxBytesReader(writer, request.Body, 2048)
	data, readErr := io.ReadAll(io.LimitReader(request.Body, 2049))
	input, inputErr := supporterengine.ParseActivationJSON(data)
	if mediaErr != nil || mediaType != "application/json" || request.URL.RawQuery != "" || readErr != nil || len(data) > 2048 || inputErr != nil {
		apiError(writer, errors.New("invalid supporter activation request"), http.StatusBadRequest)
		return "", "", false
	}
	name := ""
	if input.RecognitionName != nil {
		name = *input.RecognitionName
	}
	return input.Key, name, true
}
