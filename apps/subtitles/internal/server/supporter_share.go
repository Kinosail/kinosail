package server

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"unicode/utf8"
)

type supporterShareData struct {
	Family, State, BadgeName, LevelName, RecognitionName, PrimaryColor string
	SupportedSince, ExpiresAt, Outline, Ornament, ServiceMarks         string
	Collection                                                         *supporterCollectionStatus
	FitRecognition                                                     bool
}

const supporterCertificateSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1200 630" role="img" aria-labelledby="title description"><title id="title">{{.LevelName}} {{.Family}} certificate for Kinosail Subtitles</title><desc id="description">A privacy-safe supporter certificate with a Kinosail Subtitles caption badge.</desc><rect width="1200" height="630" fill="#090a08"/><path d="M0 525Q310 430 610 535T1200 490V630H0Z" fill="#12170e"/><g transform="translate(70 72) scale(1.9)" fill="#11150a" stroke="{{.PrimaryColor}}" stroke-width="5" stroke-linejoin="round"><path d="{{.Outline}}"/><path d="{{.Ornament}}" fill="none" stroke="#d6b96d" stroke-width="3"/><path d="M72 78h96v72h-52l-26 18v-18H72Z" fill="#090a08" stroke="#f6f8ef" stroke-width="5"/><path d="M92 104h56M92 126h42" fill="none" stroke="{{.PrimaryColor}}" stroke-width="7" stroke-linecap="round"/></g><g font-family="Inter,ui-sans-serif,-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif"><text x="560" y="106" fill="{{.PrimaryColor}}" font-size="18" font-weight="800" letter-spacing="4">KINOSAIL SUBTITLES / {{.State}}</text><text x="560" y="190" fill="#f6f8ef" font-size="58" font-weight="800">{{.BadgeName}}</text><text x="560" y="238" fill="#9ca391" font-size="25">Level {{.LevelName}} / {{.Family}}</text>{{if .RecognitionName}}<text x="560" y="310" fill="#fff4bd" font-size="30" font-weight="700"{{if .FitRecognition}} textLength="570" lengthAdjust="spacingAndGlyphs"{{end}}>{{.RecognitionName}}</text>{{end}}<text x="560" y="388" fill="#9ca391" font-size="18" font-weight="700" letter-spacing="2">SUPPORTED SINCE</text><text x="560" y="424" fill="#f6f8ef" font-size="25">{{.SupportedSince}}</text>{{if .ExpiresAt}}<text x="830" y="388" fill="#9ca391" font-size="18" font-weight="700" letter-spacing="2">{{if eq .State "ARCHIVED"}}ACTIVE THROUGH{{else}}CURRENT THROUGH{{end}}</text><text x="830" y="424" fill="#f6f8ef" font-size="25">{{.ExpiresAt}}</text>{{end}}{{if .ServiceMarks}}<text x="560" y="480" fill="{{.PrimaryColor}}" font-size="18" font-weight="800" letter-spacing="2">LIVING SERVICE / {{.ServiceMarks}}</text>{{end}}{{if .Collection}}<text x="560" y="520" fill="#d6b96d" font-size="20" font-weight="800">COMPLETE FLEET / {{.Collection.Edition}}</text>{{end}}<text x="560" y="585" fill="#9ca391" font-size="16">No payment, activation, device, or private installation data is included.</text></g></svg>`

var supporterCertificateView = template.Must(template.New("supporter-certificate").Parse(supporterCertificateSVG))

func (program *supporterProgram) writeShareCertificate(writer http.ResponseWriter, family string) {
	certificate, status, available := program.certificateForFamily(family)
	if !available {
		apiNotFound(writer)
		return
	}
	badge := supporterBadge(family, status.Rank)
	state := "PERMANENT"
	primaryColor := "#d6b96d"
	if family == livingStandardFamily {
		state = "ACTIVE"
		primaryColor = "#c8f169"
		if status.Archived {
			state = "ARCHIVED"
		}
	}
	data := supporterShareData{
		Family: strings.ToUpper(status.Title), State: state, BadgeName: badge.BadgeName,
		LevelName: fmt.Sprintf("%d / %s", status.Rank, status.Name), RecognitionName: certificate.RecognitionName,
		FitRecognition: fitSupporterRecognition(certificate.RecognitionName),
		PrimaryColor:   primaryColor,
		SupportedSince: supporterDate(certificate.SupportedSince), ExpiresAt: supporterDate(certificate.ExpiresAt),
		Outline: badge.Outline, Ornament: badge.Ornament, ServiceMarks: supporterServiceMarkText(status.ServiceMarks), Collection: status.Collection,
	}
	filename := fmt.Sprintf("kinosail-subtitles-%s-%s.svg", strings.ReplaceAll(status.Tier, "_", "-"), family)
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	writer.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	writer.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	if err := supporterCertificateView.Execute(writer, data); err != nil {
		http.Error(writer, "certificate unavailable", http.StatusInternalServerError)
	}
}

func fitSupporterRecognition(value string) bool {
	return utf8.RuneCountInString(value) > 19
}

func supporterServiceMarkText(marks []int) string {
	values := make([]string, len(marks))
	for index, mark := range marks {
		values[index] = fmt.Sprintf("%dM", mark)
	}
	return strings.Join(values, " · ")
}
