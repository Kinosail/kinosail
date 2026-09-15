package server

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestSupporterShareCertificatesUseEveryGallerySilhouette(t *testing.T) { //nolint:gocognit // The table verifies every public certificate silhouette.
	t.Parallel()
	if marks := supporterServiceMarkText([]int{3, 6, 12, 24, 36, 60}); marks != "3M · 6M · 12M · 24M · 36M · 60M" {
		t.Fatalf("service marks = %q", marks)
	}
	for _, family := range []string{livingStandardFamily, patronOrderFamily} {
		for _, badge := range supporterBadgeTiers(family, supporterGrantStatus{}, 0) {
			primary := "#d6b96d"
			if family == livingStandardFamily {
				primary = "#c8f169"
			}
			data := supporterShareData{
				Family: strings.ToUpper(family), State: "ACTIVE", BadgeName: badge.BadgeName,
				LevelName: fmt.Sprintf("%d / %s", badge.Rank, badge.Name), SupportedSince: "Aug 30, 2026",
				Outline: badge.Outline, Ornament: badge.Ornament, PrimaryColor: primary,
			}
			var output bytes.Buffer
			if err := supporterCertificateView.Execute(&output, data); err != nil {
				t.Fatal(err)
			}
			for _, expected := range []string{`d="` + badge.Outline + `"`, `d="` + badge.Ornament + `"`, `stroke="` + primary + `"`, badge.BadgeName} {
				if !strings.Contains(output.String(), expected) {
					t.Fatalf("%s level %d certificate omitted %q", family, badge.Rank, expected)
				}
			}
			if strings.Contains(output.String(), "ZgotmplZ") {
				t.Fatalf("%s level %d certificate sanitized its badge", family, badge.Rank)
			}
		}
	}
}

func TestSupporterShareCertificateFitsMaximumRecognitionName(t *testing.T) {
	t.Parallel()
	for _, length := range []int{20, 24, 32, 80} {
		name := strings.Repeat("W", length)
		data := supporterShareData{RecognitionName: name, FitRecognition: fitSupporterRecognition(name)}
		var output bytes.Buffer
		if err := supporterCertificateView.Execute(&output, data); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(output.String(), `textLength="570" lengthAdjust="spacingAndGlyphs">`+name+`</text>`) || strings.Contains(output.String(), "ZgotmplZ") {
			t.Fatalf("%d-character recognition name was not fitted: %q", length, output.String())
		}
	}
	if fitSupporterRecognition(strings.Repeat("W", 19)) {
		t.Fatal("19-character recognition name was fitted")
	}
}

func BenchmarkSupporterShareCertificate(b *testing.B) {
	badge := supporterBadgeTiers(livingStandardFamily, supporterGrantStatus{}, 0)[9]
	data := supporterShareData{
		Family:         "LIVING STANDARD",
		State:          "ACTIVE",
		BadgeName:      badge.BadgeName,
		LevelName:      "10 / Rosetta Crown",
		SupportedSince: "Aug 30, 2026",
		Outline:        badge.Outline,
		Ornament:       badge.Ornament,
		PrimaryColor:   "#c8f169",
		ServiceMarks:   "3M · 6M · 12M · 24M · 36M · 60M",
	}
	var output bytes.Buffer
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		output.Reset()
		if err := supporterCertificateView.Execute(&output, data); err != nil {
			b.Fatal(err)
		}
	}
}
