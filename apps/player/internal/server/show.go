package server

import (
	"net/http"
	"strings"

	"github.com/MikeO7/kinosail/packages/catalogapi"
	"github.com/MikeO7/kinosail/packages/library"
)

const showHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>{{.Title}} · Kinosail</title><script src="/static/theme.js?v=electric-1"></script><link rel="stylesheet" href="/static/app.css?v=skeleton-3"></head><body class="detail-page show-detail"><main class="detail-shell last-light" data-palette-id="{{.BackdropID}}"><a class="back" href="/?view=shows">{{icon "back"}} Shows</a><section class="media-hero{{if .HasBackdrop}} has-media-backdrop{{end}}">{{if .HasBackdrop}}<img class="media-backdrop" src="/backdrop/{{.BackdropID}}" alt="" fetchpriority="high">{{end}}{{if .ArtworkID}}<img class="hero-poster" src="/art/{{.ArtworkID}}" alt="" loading="eager">{{else}}<div class="hero-poster art">{{icon "play"}}</div>{{end}}<div class="media-hero-copy"><span class="eyebrow">Show</span><h1>{{.Title}}</h1><p class="meta-line">{{if .Year}}{{.Year}}{{end}}{{if .Genres}} · {{.Genres}}{{end}}{{if .Studio}} · {{.Studio}}{{end}}</p>{{if .Plot}}<p>{{.Plot}}</p>{{end}}{{if .Next}}<div class="watch-progress" data-watch-progress="{{.Next.ID}}" hidden><span data-watch-remaining></span><progress max="100" value="0" aria-label="Watch progress"></progress></div><div class="hero-actions"><a class="button hero-action" href="/watch/{{.Next.ID}}">{{if .Next.Watched}}{{t "Play again"}}{{else}}{{t "Play next"}}{{end}} · {{.Next.DisplayTitle}}</a><a class="button quiet season-jump" href="#seasons">Jump to seasons</a></div>{{end}}</div></section><section id="seasons" class="detail-section"><header><span class="eyebrow">{{t "Episodes"}}</span><h2>{{.EpisodeCount}} available</h2></header><nav class="season-index" aria-label="Jump to season"><span class="season-index-label">Jump to</span>{{range .SeasonGroups}}<a href="#season-{{.Number}}">Season {{.Number}}</a>{{end}}</nav>{{range .SeasonGroups}}<section id="season-{{.Number}}" class="season-group"><header class="season-heading"><h3>Season {{.Number}}</h3><p>{{len .Episodes}} episode{{if ne (len .Episodes) 1}}s{{end}}{{if .WatchedCount}} · {{.WatchedCount}} watched{{end}}</p></header><div class="season-reel" data-season-reel>{{with .Preview}}<a class="episode-preview" href="/watch/{{.ID}}" data-preview-link><span class="episode-preview-art{{if not .Artwork}} art{{end}}" data-preview-art>{{if .Artwork}}<img src="/art/{{.ID}}?variant=episode" alt="" data-preview-image>{{else}}<img hidden alt="" data-preview-image>{{end}}</span><span class="eyebrow" data-preview-action>{{if .Watched}}{{t "Play again"}}{{else if .Next}}{{t "Play next"}}{{else}}{{t "Play episode"}}{{end}}</span><strong class="episode-title" data-preview-title>{{.DisplayTitle}}</strong><small class="episode-subtitle" data-preview-plot {{if not .Plot}}hidden{{end}}>{{.Plot}}</small></a>{{end}}<div class="episode-ledger">{{range .Episodes}}<a class="episode{{if .Next}} is-next{{end}}" href="/watch/{{.ID}}" aria-label="{{if .Watched}}Play again episode {{.Episode}}, {{.DisplayTitle}}, watched{{else if .Next}}Play episode {{.Episode}}, {{.DisplayTitle}}, next episode{{else}}Play episode {{.Episode}}, {{.DisplayTitle}}{{end}}" data-episode-row data-episode-title="{{.DisplayTitle}}" data-episode-plot="{{.Plot}}" data-episode-action="{{if .Watched}}{{t "Play again"}}{{else if .Next}}{{t "Play next"}}{{else}}{{t "Play episode"}}{{end}}" data-episode-art="{{if .Artwork}}/art/{{.ID}}?variant=episode{{end}}"><span class="episode-still" aria-hidden="true">{{if .Artwork}}<img src="/art/{{.ID}}?variant=episode" alt="" loading="lazy" width="160" height="90">{{else}}{{icon "play"}}{{end}}</span><span class="episode-index" aria-hidden="true">{{if lt .Episode 10}}0{{end}}{{.Episode}}</span><span class="episode-copy"><strong class="episode-title">{{.DisplayTitle}}</strong>{{if .Plot}}<small class="episode-subtitle">{{.Plot}}</small>{{end}}</span>{{if .Next}}<span class="episode-state">{{t "Next"}}</span>{{else if .Watched}}<span class="episode-state">{{icon "check"}} {{t "Watched"}}</span>{{end}}</a>{{end}}</div></div></section>{{end}}</section>{{if .People}}<section class="cast-section"><header><h2>Cast</h2></header><div class="grid rail">{{range .People}}<a class="card" href="/actor?name={{urlquery .Name}}">{{if .Image}}<img class="poster" src="{{.Image}}" alt="" loading="lazy">{{else}}<div class="poster art" aria-hidden="true">{{icon "play"}}</div>{{end}}<h3>{{.Name}}</h3>{{if .Role}}<small>{{.Role}}</small>{{end}}</a>{{end}}</div></section>{{end}}</main></body></html>`

var showView = newLocalizedTemplate("show", showHTML)

type episodeData struct {
	library.Item
	DisplayTitle string
	Watched      bool
	Next         bool
}

type seasonPageData struct {
	Number       int
	Episodes     []episodeData
	Preview      *episodeData
	WatchedCount int
}

type showPageData struct {
	library.Show
	SeasonGroups []seasonPageData
	Next         *episodeData
	EpisodeCount int
	BackdropID   string
	HasBackdrop  bool
	People       []catalogapi.ClientPerson
}

type showPlayAction struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Label  string `json:"label"`
	Stream string `json:"stream"`
}

type showCard struct {
	library.Show
	Play *showPlayAction
}

func showPlay(request *http.Request, progress *progressStore, show library.Show) *showPlayAction {
	for _, episode := range show.Episodes {
		state := progress.Get(request, episode.ID)
		if !state.Watched {
			label := "Play next"
			if state.Seconds > 0 {
				label = "Resume"
			}
			return &showPlayAction{episode.ID, episode.Title, label, "/watch/" + episode.ID}
		}
	}
	if len(show.Episodes) == 0 {
		return nil
	}
	episode := show.Episodes[0]
	return &showPlayAction{episode.ID, episode.Title, "Play again", "/watch/" + episode.ID}
}

func showCards(request *http.Request, progress *progressStore, shows []library.Show) []showCard {
	result := make([]showCard, len(shows))
	for index, show := range shows {
		result[index] = showCard{show, showPlay(request, progress, show)}
	}
	return result
}

func browseShow(index *libraryIndex, progress *progressStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		items, _ := visibleLibrary(request, index)
		_, shows := library.Organize(items)
		for _, show := range shows {
			if show.ID != request.PathValue("id") {
				continue
			}
			writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			if err := showView.Execute(writer, request, projectShowPage(request, progress, show)); err != nil {
				localizedError(writer, request, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		localizedNotFound(writer, request)
	}
}

func projectShowPage(request *http.Request, progress *progressStore, show library.Show) showPageData {
	data := showPageData{Show: show, EpisodeCount: len(show.Episodes), BackdropID: showBackdropID(show), HasBackdrop: show.Backdrop != "", People: catalogapi.ShowCast(show)}
	for _, episode := range show.Episodes {
		projected := episodeData{Item: episode, DisplayTitle: episodeDisplayTitle(episode), Watched: progress.Watched(request, episode.ID)}
		if data.Next == nil && !projected.Watched {
			projected.Next = true
			data.Next = &projected
		}
		appendSeasonEpisode(&data, projected)
	}
	if data.Next == nil && len(show.Episodes) != 0 {
		data.Next = &episodeData{Item: show.Episodes[0], DisplayTitle: episodeDisplayTitle(show.Episodes[0]), Watched: true}
	}
	return data
}

func appendSeasonEpisode(data *showPageData, episode episodeData) {
	if len(data.SeasonGroups) == 0 || data.SeasonGroups[len(data.SeasonGroups)-1].Number != episode.Season {
		data.SeasonGroups = append(data.SeasonGroups, seasonPageData{Number: episode.Season})
	}
	season := &data.SeasonGroups[len(data.SeasonGroups)-1]
	season.Episodes = append(season.Episodes, episode)
	if episode.Watched {
		season.WatchedCount++
	}
	if season.Preview == nil || season.Preview.Watched && !episode.Watched {
		preview := episode
		season.Preview = &preview
	}
}

func episodeDisplayTitle(item library.Item) string {
	if _, title, found := strings.Cut(item.Title, " · "); found {
		return title
	}
	return item.Title
}
