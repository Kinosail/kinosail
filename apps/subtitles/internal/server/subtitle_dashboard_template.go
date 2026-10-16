package server

import _ "embed"

//go:embed subtitle_dashboard.html
var subtitleDashboardHTML string

//go:embed subtitle_dashboard_item.html
var subtitleDashboardItemHTML string

var subtitleDashboardView = newLazyLocalizedTemplate("subtitle-dashboard", func() string {
	return subtitleDashboardHTML + subtitleDashboardItemHTML + `{{define "subtitle-navigation"}}` + subtitleAppHeader + `{{end}}`
})
