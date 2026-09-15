package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/mcpgateway"
)

var mcpApprovalView = newCSRFTemplate("mcp-approval", `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Connect {{.ClientName}} · Kinosail Subtitles</title><link rel="stylesheet" href="/static/app.css?v=electric-1"></head><body class="settings-page"><a class="skip" href="#main">Skip to content</a><main id="main" class="settings-shell"><a class="back" href="/settings/agent-connections">{{icon "back"}} Agent connections</a><header class="settings-intro"><span class="eyebrow">Private connection</span><h1>Allow {{.ClientName}} to connect?</h1><p>Kinosail will share only the capabilities you approve for {{.ProfileName}}. Media bytes stay on this Server.</p></header><form method="post" action="/oauth/authorize"><input type="hidden" name="request" value="{{.RequestID}}"><fieldset><legend>Access</legend><label><input type="checkbox" name="scopes" value="kinosail.read" checked> Browse library and viewing context</label>{{if .Write}}<label><input type="checkbox" name="scopes" value="kinosail.write"> Update personal library state</label>{{end}}{{if .Manage}}<label><input type="checkbox" name="scopes" value="kinosail.manage"> Administer this Server (requires recent sign-in)</label>{{end}}</fieldset><button name="decision" value="allow">Allow connection</button><button class="quiet" name="decision" value="deny">Cancel</button></form></main></body></html>`)

type mcpApprovalData = mcpgateway.Approval

func mcpApproval(writer http.ResponseWriter, request *http.Request, approval mcpgateway.Approval) error {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	return executeCSRFTemplate(mcpApprovalView, writer, request, approval)
}
