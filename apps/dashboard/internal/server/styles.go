package server

import _ "embed"

var (
	//go:embed web/static/dashboard.css
	dashboardBaseCSS []byte
	//go:embed web/static/dashboard-workspace.css
	dashboardWorkspaceCSS []byte
	dashboardCSS          = append(append([]byte(nil), dashboardBaseCSS...), dashboardWorkspaceCSS...)
)
