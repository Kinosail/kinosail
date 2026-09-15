package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

const showHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>{{.Title}} · Kinosail Subtitles</title><script src="/static/theme.js?v=electric-1"></script><link rel="stylesheet" href="/static/app.css?v=electric-1"></head><body class="detail-page"><main class="detail-shell last-light" data-palette-id="{{.BackdropID}}"><a class="back" href="/?view=shows">{{icon "back"}} Shows</a><section class="media-hero{{if .BackdropID}} has-media-backdrop{{end}}">{{if .BackdropID}}<img class="media-backdrop" src="/backdrop/{{.BackdropID}}" alt="" fetchpriority="high">{{end}}{{if .ArtworkID}}<img class="hero-poster" src="/art/{{.ArtworkID}}" alt="" loading="eager">{{else}}<div class="hero-poster art">{{icon "play"}}</div>{{end}}<div class="media-hero-copy"><span class="eyebrow">Show</span><h1>{{.Title}}</h1><p class="meta-line">{{if .Year}}{{.Year}}{{end}}{{if .Genres}} · {{.Genres}}{{end}}{{if .Studio}} · {{.Studio}}{{end}}</p>{{if .Plot}}<p>{{.Plot}}</p>{{end}}{{if .Next}}<div class="watch-progress" data-watch-progress="{{.Next.ID}}" hidden><span data-watch-remaining></span><progress max="100" value="0" aria-label="Watch progress"></progress></div><a class="button hero-action" href="{{.Next.Stream}}">{{.Next.Label}} · {{.Next.Title}}</a>{{end}}</div></section><section class="detail-section"><header><span class="eyebrow">Episodes</span><h2>{{.EpisodeCount}} available</h2></header>{{range .SeasonGroups}}<section class="season-group"><h3>Season {{.Number}}</h3><div class="episode-list">{{range .Episodes}}<a class="episode" href="/watch/{{.ID}}"><span class="play">{{icon "play"}}</span><span><strong>{{.Title}}</strong>{{if .Plot}}<small>{{.Plot}}</small>{{end}}</span>{{if .Watched}}<span class="status" aria-label="Watched">{{icon "check"}} Watched</span>{{end}}</a>{{end}}</div></section>{{end}}</section></main></body></html>`

var showView = newLocalizedTemplate("show", showHTML)

type episodeData struct {
	library.Item
	Watched bool
}

type seasonPageData struct {
	Number   int
	Episodes []episodeData
}

type showPageData struct {
	library.Show
	SeasonGroups []seasonPageData
	Next         *showPlayAction
	EpisodeCount int
	BackdropID   string
}

type showPlayAction = catalog.PlayAction

type showCard struct {
	library.Show
	Play *showPlayAction
}

func showPlay(request *http.Request, progress *progressStore, show library.Show) *showPlayAction {
	return catalog.ShowPlay(show, func(id string) playbackState { return progress.Get(request, id) })
}

func showCards(request *http.Request, progress *progressStore, shows []library.Show) []showCard {
	result := make([]showCard, len(shows))
	for index, show := range shows {
		result[index] = showCard{show, showPlay(request, progress, show)}
	}
	return result
}

func browseShow(index *libraryIndex, progress *progressStore) http.HandlerFunc { //nolint:gocognit,cyclop // One projection preserves episode order, seasons, and next-up choice.
	return func(writer http.ResponseWriter, request *http.Request) {
		items, _ := visibleLibrary(request, index)
		_, shows := library.Organize(items)
		for _, show := range shows {
			if show.ID == request.PathValue("id") {
				data := showPageData{Show: show, Next: showPlay(request, progress, show), EpisodeCount: len(show.Episodes), BackdropID: showBackdropID(show)}
				for _, episode := range show.Episodes {
					projected := episodeData{episode, progress.Watched(request, episode.ID)}
					if len(data.SeasonGroups) == 0 || data.SeasonGroups[len(data.SeasonGroups)-1].Number != episode.Season {
						data.SeasonGroups = append(data.SeasonGroups, seasonPageData{Number: episode.Season})
					}
					position := len(data.SeasonGroups) - 1
					data.SeasonGroups[position].Episodes = append(data.SeasonGroups[position].Episodes, projected)
				}
				writer.Header().Set("Content-Type", "text/html; charset=utf-8")
				if err := showView.Execute(writer, request, data); err != nil {
					localizedError(writer, request, err.Error(), http.StatusInternalServerError)
				}
				return
			}
		}
		localizedNotFound(writer, request)
	}
}
