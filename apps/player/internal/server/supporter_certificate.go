package server

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"unicode"
)

type supporterCertificateView struct {
	Name, NameLine1, NameLine2, Family, FamilySlug, Badge, Tier, Since, State, Fleet, Edition, ServiceMarks string
	Rank, ServiceCount, NameSize                                                                            int
	BadgeArt                                                                                                template.HTML
}

var supporterCertificateSVG = template.Must(template.New("supporter-certificate").Parse(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1200 630" role="img" aria-labelledby="title description" data-family="{{.FamilySlug}}" data-rank="{{.Rank}}"><title id="title">Kinosail Player {{.Family}}, level {{.Rank}}, for {{.Name}}</title><desc id="description">A shareable Kinosail Player supporter certificate. It contains no payment, activation, installation, device, or media data.</desc><style>text{font-family:Inter,ui-sans-serif,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}.field{fill:#090a08}.frame{fill:none;stroke:#30352b;stroke-width:2}.kicker,.state,.fleet{fill:#c8f169;font-size:18px;font-weight:800;letter-spacing:5px}.state{font-size:18px;letter-spacing:.3px}.name{fill:#f6f8ef;font-weight:800;letter-spacing:-2px}.badge-name{fill:#fff4bd;font-size:38px;font-weight:760}.detail{fill:#9ca391;font-size:22px}.family{fill:#d6b96d;font-size:20px;font-weight:800;letter-spacing:3px}</style><rect class="field" width="1200" height="630"/><path class="frame" d="M32 32h1136v566H32zM50 50h1100v530H50z"/><svg x="35" y="105" width="430" height="430" viewBox="0 0 360 360">{{.BadgeArt}}</svg><text class="kicker" x="500" y="112">KINOSAIL PLAYER · SUPPORTER PASSPORT</text>{{if .NameLine2}}<text class="name" x="500" y="184" style="font-size:{{.NameSize}}px">{{.NameLine1}}</text><text class="name" x="500" y="224" style="font-size:{{.NameSize}}px">{{.NameLine2}}</text>{{else}}<text class="name" x="500" y="205" style="font-size:{{.NameSize}}px">{{.NameLine1}}</text>{{end}}<text class="family" x="500" y="254">{{.Family}} · LEVEL {{.Rank}}</text><text class="badge-name" x="500" y="307">{{.Badge}}</text><text class="detail" x="500" y="360">{{.Tier}} · Supported since {{.Since}}</text><text class="state" x="500" y="416">{{.State}}</text>{{if .ServiceCount}}<text class="detail" x="500" y="462">{{.ServiceCount}} tenure service {{if eq .ServiceCount 1}}mark{{else}}marks{{end}}</text>{{end}}{{if .Fleet}}<text class="fleet" x="500" y="510">{{.Fleet}} · {{.Edition}}</text>{{end}}<text class="detail" x="500" y="558">With gratitude, Kinosail.</text></svg>`))

func supporterCertificateArt(family string, rank int) template.HTML {
	if (family != patronOrderFamily && family != livingStandardFamily && editionName(family) == "") || rank < 1 || rank > 10 {
		return ""
	}
	art, err := supporterBadges.ReadFile(fmt.Sprintf("static/supporter/badges/%s-%d.svg", family, rank))
	if err != nil {
		return ""
	}
	// Only repository-owned, embedded SVG artwork reaches this trusted template fragment.
	content := strings.Replace(string(art), `id="title"`, `id="badge-title"`, 1)
	return template.HTML(content) //nolint:gosec // Fixed embedded artwork; no user content.
}

func renderSupporterCertificate(view supporterCertificateView) ([]byte, error) {
	view.NameLine1, view.NameLine2, view.NameSize = certificateNameLines(view.Name)
	view.BadgeArt = supporterCertificateArt(view.FamilySlug, view.Rank)
	var output bytes.Buffer
	if err := supporterCertificateSVG.Execute(&output, view); err != nil {
		return nil, err
	}
	rendered := output.String()
	if view.ServiceMarks != "" {
		label := strconv.Itoa(view.ServiceCount) + " tenure service marks"
		if view.ServiceCount == 1 {
			label = "1 tenure service mark"
		}
		rendered = strings.Replace(rendered, label, "Tenure marks · "+view.ServiceMarks, 1)
	}
	return []byte(rendered), nil
}

func certificateNameLines(value string) (string, string, int) {
	characters := []rune(value)
	if len(characters) <= 24 {
		return value, "", certificateNameSize(characters)
	}
	cut := (len(characters) + 1) / 2
	for index := cut; index >= max(12, cut-8); index-- {
		if unicode.IsSpace(characters[index]) {
			cut = index
			break
		}
	}
	first, second := characters[:cut], []rune(strings.TrimSpace(string(characters[cut:])))
	return string(first), string(second), min(38, certificateNameSize(first), certificateNameSize(second))
}

func certificateNameSize(characters []rune) int {
	width := 0.0
	for _, character := range characters {
		switch {
		case unicode.IsSpace(character):
			width += .32
		case character > unicode.MaxASCII || strings.ContainsRune("MW@%", character):
			width++
		case unicode.IsUpper(character):
			width += .72
		default:
			width += .56
		}
	}
	if width == 0 {
		return 66
	}
	return max(12, min(66, int(630/width)))
}

func (program *supporterProgram) writeCertificate(writer http.ResponseWriter, family string) {
	status := program.status()
	badge := status.PatronOrder
	familyName, badgeNames := "Patron Order", patronBadgeTitles
	if family == livingStandardFamily {
		badge, familyName, badgeNames = status.LivingStandard, "Living Standard", livingBadgeTitles
	} else if editionName(family) != "" {
		familyName = editionName(family)
		if family == "monthly" {
			badge = status.Monthly
		}
		if family == "yearly" {
			badge = status.Yearly
		}
	} else if family != patronOrderFamily {
		http.Error(writer, "supporter badge family is invalid", http.StatusBadRequest)
		return
	}
	if badge == nil {
		http.Error(writer, "supporter badge is not available", http.StatusNotFound)
		return
	}
	view := certificateView(badge, family, familyName, badgeNames)
	rendered, err := renderSupporterCertificate(view)
	if err != nil {
		http.Error(writer, "could not create supporter certificate", http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
	writer.Header().Set("Content-Disposition", `attachment; filename="kinosail-player-`+family+`-level-`+strconv.Itoa(badge.Rank)+`.svg"`)
	writer.Header().Set("Cache-Control", "private, no-store")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = writer.Write(rendered)
}

func certificateView(badge *supporterBadgeStatus, family, familyName string, badgeNames []string) supporterCertificateView {
	name := badge.RecognitionName
	if name == "" {
		name = "Kinosail Supporter"
	}
	state := "Permanent"
	if badge.ExpiresAt != "" {
		state = "Active through " + supporterDate(badge.ExpiresAt)
		if badge.Expired {
			state = "Archived · Supported through " + supporterDate(badge.ExpiresAt)
		}
	}
	view := supporterCertificateView{Name: name, Family: familyName, FamilySlug: family, Badge: badgeNames[badge.Rank-1], Tier: badge.Name, Since: supporterDate(badge.SupportedSince), State: state, Rank: badge.Rank, ServiceCount: len(badge.ServiceMarks)}
	if editionName(family) != "" {
		view.Badge = badge.Name + " " + editionName(family)
	}
	if family == livingStandardFamily && len(badge.ServiceMarks) > 0 {
		marks := make([]string, len(badge.ServiceMarks))
		for index, mark := range badge.ServiceMarks {
			marks[index] = strconv.Itoa(mark)
		}
		view.ServiceMarks = strings.Join(marks, " / ") + " months"
	}
	if badge.Collection != nil {
		view.Fleet, view.Edition = badge.Collection.Name, badge.Collection.Edition
	}
	return view
}

func (program *supporterProgram) writeCurrentCertificate(writer http.ResponseWriter) {
	program.writeCertificate(writer, currentSupporterCertificate(program.status()))
}

func currentSupporterCertificate(status supporterStatus) string {
	badges := []struct {
		family string
		badge  *supporterBadgeStatus
	}{
		{"monthly", status.Monthly}, {"yearly", status.Yearly}, {livingStandardFamily, status.LivingStandard}, {patronOrderFamily, status.PatronOrder},
	}
	for _, item := range badges {
		if item.badge != nil && item.badge.Active {
			return item.family
		}
	}
	for _, item := range badges {
		if item.badge != nil {
			return item.family
		}
	}
	return patronOrderFamily
}
