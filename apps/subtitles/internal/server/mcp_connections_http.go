package server

import (
	"errors"
	"net/http"

	"github.com/MikeO7/kinosail/packages/mcpgateway"
)

var agentConnectionsView = newCSRFTemplate("agent-connections", `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Agent connections · Kinosail Subtitles</title><script src="/static/theme.js?v=electric-1"></script><link rel="stylesheet" href="/static/app.css?v=electric-1"></head><body class="settings-page"><a class="skip" href="#main">Skip to content</a><main id="main" class="settings-shell"><a class="back" href="/settings">{{icon "back"}} Settings</a><header class="settings-intro"><span class="eyebrow">Owner controls</span><h1>Connect Codex and other AI agents</h1><p>Choose the connection that fits where your agent runs. Connections stay direct to this Server, and Library Content stays on it.</p></header><div class="settings-flow"><section class="wide"><h2>Recommended: host connection</h2><p>No certificate or browser approval is required. This uses the SSH and Docker access already held by a Server administrator.</p><p>On the Server host, run <code>{{.StdioCommand}}</code></p><p>From another device with an SSH host alias, run <code>{{.StdioSSHCommand}}</code> and replace <code>SERVER</code> with that alias.</p>{{if gt (len .Owners) 1}}<p>This Server has multiple Owners. Append one Owner Profile ID to <code>kinosail mcp-stdio</code>:</p>{{range .Owners}}<p><strong>{{.Name}}</strong> · <code>{{.ID}}</code></p>{{end}}{{end}}<p>This host connection has full Owner access and is recorded in activity as <strong>Host MCP</strong>. Remove its Codex entry or revoke the underlying SSH/Docker access to stop it.</p></section><section class="wide"><h2>HTTPS / OAuth alternative</h2>{{if eq .Mode "built-in"}}<p>Use this when an agent cannot use the Server host. It supports revocable read, write, and management grants through browser approval.</p>{{if .CertificateAvailable}}<p>A locally generated HTTPS certificate may prompt each client to trust the <a href="/api/v1/agent-connections/certificate">Kinosail local CA certificate</a>. A publicly trusted certificate avoids that prompt.</p>{{else}}<p>This option requires an HTTPS certificate already trusted by the client.</p>{{end}}<ol><li>Run <code>{{index .HTTPCommands 0}}</code></li><li>Run <code>{{index .HTTPCommands 1}}</code>, then approve the browser prompt.</li></ol>{{else}}<p>This Server uses an external OAuth provider. Register the agent there, then connect it to <code>{{.Resource}}</code>.</p>{{end}}</section><section class="wide"><h2>Active HTTPS / OAuth connections</h2>{{range .Connections}}<div class="profile-row"><div><strong>{{.ClientName}}</strong><p>{{.ProfileName}} · {{range $i, $scope := .Scopes}}{{if $i}}, {{end}}{{$scope}}{{end}} · expires {{.Expires}}{{if .LastUsed}} · last used {{.LastUsed}}{{end}}</p></div><form action="/settings/agent-connections/revoke" method="post"><button class="danger" name="id" value="{{.ID}}">Revoke {{.ClientName}}</button></form></div>{{else}}<p>No HTTPS / OAuth agents are connected.</p>{{end}}</section><section><h2>HTTPS connection address</h2><p><code>{{.Resource}}</code></p><p>Read access is the default. Write access is optional. Server administration requires an Owner and a recent strong sign-in.</p></section></div></main></body></html>`)

func (connections *mcpConnections) projection() mcpgateway.ConnectionProjection {
	return connections.ApplicationProjection(mcpProfileState(connections.profiles), connections.dataDir)
}

func apiAgentConnections(connections *mcpConnections) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		projection := connections.projection()
		writeJSON(writer, projection.APIDocument(), http.StatusOK)
	}
}

func apiRevokeAgentConnection(connections *mcpConnections) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if err := connections.Revoke(request.PathValue("id")); err != nil {
			apiError(writer, err, http.StatusNotFound)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}
}

func showAgentConnections(connections *mcpConnections, _ string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := executeCSRFTemplate(agentConnectionsView, writer, request, connections.projection()); err != nil {
			localizedError(writer, request, "agent connections view failed", http.StatusInternalServerError)
		}
	}
}

func registerAgentConnections(mux *http.ServeMux, auth *authentication, connections *mcpConnections, dataDir string) {
	mux.Handle("GET /settings/agent-connections", auth.owner(showAgentConnections(connections, dataDir)))
	mux.Handle("POST /settings/agent-connections/revoke", auth.owner(webRevokeAgentConnection(connections)))
}

func webRevokeAgentConnection(connections *mcpConnections) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		request.Body = http.MaxBytesReader(writer, request.Body, 16<<10)
		if !formEncoded(request) || request.URL.RawQuery != "" || request.ParseForm() != nil || !onlyFormKeys(request.PostForm, "id") {
			localizedError(writer, request, "invalid connection", http.StatusBadRequest)
			return
		}
		id, ok := oneValue(request.PostForm, "id", 128)
		if !ok {
			localizedError(writer, request, "invalid connection", http.StatusBadRequest)
			return
		}
		if err := connections.Revoke(id); err != nil {
			localizedError(writer, request, err.Error(), http.StatusNotFound)
			return
		}
		http.Redirect(writer, request, "/settings/agent-connections", http.StatusSeeOther)
	}
}

func apiAgentConnectionCertificate(connections *mcpConnections) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		certificate := mcpgateway.LocalCACertificate(connections.dataDir)
		if certificate == nil {
			apiError(writer, errors.New("local CA certificate is unavailable; configure a certificate trusted by this device"), http.StatusNotFound)
			return
		}
		writer.Header().Set("Content-Type", "application/x-pem-file")
		writer.Header().Set("Content-Disposition", `attachment; filename="kinosail-local-ca.pem"`)
		writer.Header().Set("Cache-Control", "private, no-store")
		_, _ = writer.Write(certificate)
	}
}
