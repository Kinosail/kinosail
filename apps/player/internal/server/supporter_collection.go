package server

import (
	"net/http"

	supporterengine "github.com/MikeO7/kinosail/packages/supporter"
)

type supporterCollectionView struct {
	Badges  []supporterengine.ViewerBadge `json:"badges"`
	Display string                        `json:"display"`
}

func (program *supporterProgram) collection() supporterCollectionView {
	status := program.status()
	result := supporterCollectionView{Badges: []supporterengine.ViewerBadge{}, Display: program.settings.supporterDisplay()}
	for _, badge := range []*supporterBadgeStatus{status.PatronOrder, status.Monthly, status.Yearly, status.LivingStandard} {
		if badge != nil {
			result.Badges = append(result.Badges, supporterengine.ViewerBadge{Edition: badge.Edition, Family: badge.Family, Tier: badge.Tier, Name: badge.Name, Rank: badge.Rank, Active: badge.Active, Archived: badge.Expired})
		}
	}
	return result
}

func registerSupporterCollection(mux *http.ServeMux, program *supporterProgram) {
	mux.HandleFunc("GET /api/v1/supporter/collection", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" {
			apiError(w, errSupporterDisplay, http.StatusBadRequest)
			return
		}
		w.Header().Set("Cache-Control", "private, no-store")
		writeJSON(w, program.collection(), http.StatusOK)
	})
	mux.HandleFunc("GET /supporter/collection", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, no-store")
		_ = supporterCollectionTemplate.Execute(w, r, program.collection())
	})
}

var supporterCollectionTemplate = newLocalizedTemplate("supporter-collection", `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Supporter collection · Kinosail</title><link rel="stylesheet" href="/static/app.css?v=electric-1"></head><body class="supporter-page"><main class="supporter-shell"><a href="/">Back to library</a><h1>Our supporter collection</h1><p>Thank you for making Kinosail possible. All three editions can be collected together.</p><div class="supporter-owned-grid">{{range .Badges}}<article class="owned-badge"><img width="200" height="200" src="/static/supporter/badges/{{if .Edition}}{{.Edition}}{{else}}{{.Family}}{{end}}-{{.Rank}}.svg" alt=""><h2>{{.Name}}</h2><p>{{if .Edition}}{{.Edition}}{{else}}Earlier support{{end}}{{if .Archived}} · Past support{{else}} · Collected{{end}}</p></article>{{else}}<p>No badges collected yet. The Server Owner can add supporter keys.</p>{{end}}</div><p>Kinosail stays complete and free for everyone.</p></main></body></html>`)
