package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/auditjournal"
)

var subtitleAuditActions = map[string]string{
	"/onboarding/subtitles/language":         "settings.subtitles.updated",
	"/onboarding/subtitles/scans":            "settings.scans.updated",
	"/onboarding/subtitles/libraries":        "library.added",
	"/onboarding/subtitles/libraries/remove": "library.removed",
	"/onboarding/tmdb":                       "",
}

func auditAction(request *http.Request) (string, string) {
	return auditjournal.Classify(request, subtitleAuditActions)
}

func auditTarget(request *http.Request) string { return auditjournal.Target(request) }

func auditDetails(request *http.Request) map[string]string { return auditjournal.Details(request) }

func truncate(value string, limit int) string { return auditjournal.Truncate(value, limit) }
