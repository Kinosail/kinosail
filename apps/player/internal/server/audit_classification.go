package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/auditjournal"
)

func auditAction(request *http.Request) (string, string) {
	return auditjournal.Classify(request, nil)
}

func auditTarget(request *http.Request) string { return auditjournal.Target(request) }

func auditDetails(request *http.Request) map[string]string { return auditjournal.Details(request) }

func truncate(value string, limit int) string { return auditjournal.Truncate(value, limit) }
