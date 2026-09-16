package server

import (
	"context"
	"crypto/rand"
	"log/slog"
	"net/http"
	"time"

	"github.com/MikeO7/kinosail/packages/webassets"

	"github.com/MikeO7/kinosail-player/internal/configuration"
	"github.com/MikeO7/kinosail-player/internal/database"
	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
	markerlogic "github.com/MikeO7/kinosail/packages/markers"
	sharedmetadata "github.com/MikeO7/kinosail/packages/metadata"
	"github.com/MikeO7/kinosail/packages/remoteaccess"
	"github.com/MikeO7/kinosail/packages/serverdiscovery"
	"github.com/MikeO7/kinosail/packages/trustedhttps"
	"github.com/MikeO7/kinosail/packages/wireguard"
	"github.com/MikeO7/kinosail/packages/workload"
)

const appMobileMore = `{{if or .NavigationMore .Owner .CanLogout}}<details class="nav-more"><summary {{range .NavigationMore}}{{if .Active}}class="active"{{end}}{{end}}>More</summary><div class="nav-more-menu">{{if .CanLogout}}<section class="nav-more-section nav-more-account" role="group" aria-labelledby="nav-more-profile"><span id="nav-more-profile" class="nav-more-heading">{{t "Profile"}}</span><a class="profile-chip" href="/account">{{.ViewerName}}</a><form action="/logout" method="post"><button class="quiet">{{t "Sign out"}}</button></form></section>{{end}}<section class="nav-more-section nav-more-actions" aria-labelledby="nav-more-actions"><span id="nav-more-actions" class="nav-more-heading">{{t "Actions"}}</span><button hidden class="quiet" type="button" data-install>{{t "Install"}}</button><a href="/quick-connect">{{t "Quick Connect"}}</a>{{if .Owner}}<form action="/scan" method="post"><button class="quiet">{{t "Sync"}}</button></form><a class="nav-supporter" href="/supporter">Supporter</a><a href="/settings">{{t "Settings"}}{{if .UpdateAvailable}} · Update{{end}}</a><a class="nav-edit-menu" href="/settings#navigation">Edit navigation</a>{{end}}</section>{{if .NavigationMore}}<section class="nav-more-section nav-more-library" aria-labelledby="nav-more-library"><span id="nav-more-library" class="nav-more-heading">{{t "Library"}}</span>{{if .CommandMenu}}<button class="quiet" type="button" data-command-open aria-keyshortcuts="Control+K Meta+K">{{t "Browse library"}} <kbd data-command-shortcut>⌘K</kbd></button>{{else}}<a href="/">{{t "Browse library"}}</a>{{end}}{{range .NavigationMore}}<a{{if .Active}} class="active" aria-current="page"{{end}} href="{{.Href}}">{{.Name}}</a>{{end}}</section>{{end}}</div></details>{{end}}`

const appHeader = `<header class="app-header"><a class="brand-lockup" href="/"><img class="brand-icon" src="/static/icon.svg?v=8" alt=""><h1>{{.ServerName}}</h1></a><form class="search" action="/" role="search"><label for="library-search">{{t "Search library"}}</label><input id="library-search" type="search" enterkeyhint="search" autocomplete="off" autocapitalize="none" spellcheck="false" aria-label="{{t "Search all libraries"}}" name="q" value="{{.Query}}" placeholder="{{t "Search all libraries"}}" hx-get="/" hx-target="#main" hx-select="#main" hx-swap="outerHTML" hx-push-url="true" hx-trigger="input changed delay:250ms, search" aria-keyshortcuts="/"><input type="hidden" name="sort" value="{{.Sort}}"><input type="hidden" name="view" value="all"></form><nav aria-label="Main navigation" data-mobile-tabs data-nav-profile="{{.ViewerID}}">{{range .NavigationPrimary}}<a {{if .Active}}class="active" aria-current="page"{{end}} href="{{.Href}}">{{.Name}}</a>{{end}}{{range .NavigationMore}}<a class="nav-main-overflow{{if .Active}} active{{end}}"{{if .Active}} aria-current="page"{{end}} href="{{.Href}}">{{.Name}}</a>{{end}}{{if .Owner}}<a class="nav-main-supporter" href="/supporter">Supporter</a><a class="nav-main-edit" href="/settings#navigation">Edit navigation</a>{{end}}` + appMobileMore + `</nav><div class="header-actions"><details class="header-compact-menu"><summary>{{t "Actions"}}</summary><div class="header-compact-panel">{{if .CommandMenu}}<button class="quiet command-open" type="button" data-command-open aria-keyshortcuts="Control+K Meta+K">{{t "Actions"}} <kbd data-command-shortcut>⌘K</kbd></button>{{else}}<a class="header-link command-open" href="/">{{t "Browse library"}}</a>{{end}}<button hidden class="quiet" type="button" data-install>{{t "Install"}}</button><div class="header-utility-links{{if .Owner}} owner-utilities{{end}}"><a class="header-link" href="/quick-connect">{{t "Quick Connect"}}</a>{{if .Owner}}<form action="/scan" method="post"><button class="quiet">{{t "Sync"}}</button></form><a class="header-link" href="/settings">{{t "Settings"}}{{if .UpdateAvailable}} · Update{{end}}</a>{{end}}</div>{{if .CanLogout}}<div class="header-account" role="group" aria-label="{{t "Profile"}}"><a class="header-link profile-chip" href="/account">{{.ViewerName}}</a><form action="/logout" method="post"><button class="quiet">{{t "Sign out"}}</button></form></div>{{end}}</div></details></div></header>`

var home = homeTemplateSource()

func homeTemplateSource() string {
	return `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover"><meta name="theme-color" content="#0b0d0b"><meta name="htmx-config" content='{"includeIndicatorStyles":false}'><link rel="manifest" href="/manifest.webmanifest"><link rel="icon" href="/static/icon.svg?v=8"><link rel="apple-touch-icon" href="/static/apple-touch-icon.png?v=8">
<title>{{.ServerName}} · Kinosail Player</title><script src="/static/theme.js?v=electric-1"></script><link rel="stylesheet" href="/static/app.css?v=electric-1"><script defer src="/static/htmx.min.js"></script><script defer src="/static/main.kinosail.bundle.js?v=12"></script></head><body class="library-page"><a class="skip" href="#main">Skip to content</a>
` + appHeader + `
<aside hidden class="install-help" data-install-help role="dialog" aria-labelledby="install-title"><span class="eyebrow">Keep Kinosail Player close</span><h2 id="install-title">Install Kinosail Player</h2><p>On iPhone or iPad, tap Share, choose Add to Home Screen, turn on Open as Web App, then tap Add.</p><button type="button" data-install-close>Got it</button></aside>
<dialog class="command-menu" data-command-menu role="dialog" aria-labelledby="command-title"><header><div><span class="eyebrow">{{t "Actions"}}</span><h2 id="command-title">{{t "Browse library"}}</h2></div><form method="dialog"><button class="quiet" aria-label="{{t "Cancel"}}">Esc</button></form></header><label class="command-search">{{t "Search library"}}<input data-command-search autocomplete="off" placeholder="{{t "Search all libraries"}}"></label><div class="command-results" aria-label="{{t "Actions"}}"><a href="/" data-command data-command-label="{{t "Home"}}">{{t "Home"}}</a><a href="/?view=movies" data-command data-command-label="{{t "Movies"}}">{{t "Movies"}}</a><a href="/?view=shows" data-command data-command-label="{{t "Shows"}}">{{t "Shows"}}</a><a href="/?view=list" data-command data-command-label="{{t "My List"}}">{{t "My List"}}</a><a href="/offline-downloads" data-command data-command-label="{{t "Offline downloads"}}">{{t "Offline downloads"}}</a>{{if .Owner}}<a href="/settings" data-command data-command-label="{{t "Settings"}}">{{t "Settings"}}</a><form action="/scan" method="post"><button data-command data-command-label="{{t "Sync"}}">{{t "Sync"}}</button></form>{{end}}</div><p><kbd>/</kbd> {{t "Search library"}}</p></dialog>
<main id="main" class="library-shell last-light"><header class="library-masthead{{if or .Query (ne .View "all")}} browse-masthead{{end}}"><h2>{{if .Query}}Search results{{else if eq .View "all"}}Your evening.{{else if eq .View "list"}}My List{{else if eq .View "movies"}}Movies{{else if eq .View "shows"}}Shows{{else if eq .View "collections"}}Collections{{else if eq .View "playlists"}}Playlists{{else if eq .View "music"}}Music{{else if eq .View "audiobooks"}}Audiobooks{{else if eq .View "books"}}Books{{else if eq .View "photos"}}Photos{{else if eq .View "history"}}Playback history{{else}}Unwatched{{end}}</h2>{{if .Query}}<p class="result-summary" role="status" aria-live="polite">{{.Total}} results for “{{.Query}}”. {{if ne .View "all"}}Search scope: {{.View}}.{{end}} <a href="{{.SearchClear}}" data-search-clear>Clear search</a>{{if ne .View "all"}} · <a href="/?q={{urlquery .Query}}&amp;view=all">Search everything</a>{{end}}</p>{{else}}<p>{{if eq .View "all"}}Stored here. Streamed directly.{{else if eq .View "list"}}Titles you have saved for later.{{else if eq .View "collections"}}Browse titles grouped into collections.{{else if eq .View "playlists"}}Create a playlist or let a smart playlist update for you.{{else}}Browse, filter, and play directly from this Server.{{end}}</p>{{end}}</header>
{{if and (eq .View "all") (not .Query)}}
<nav class="home-sections" aria-label="Home sections"><a href="/" aria-current="page">For you</a><a href="/?view=list">My List</a></nav>
{{$id := ""}}{{$title := ""}}{{$href := ""}}{{$meta := ""}}{{$hasArt := false}}{{$label := "View details"}}{{$details := ""}}
{{if .Continue}}{{$item := index .Continue 0}}{{$id = $item.ID}}{{$title = $item.Title}}{{$href = printf "/watch/%s" $item.ID}}{{$meta = $item.Resume}}{{$label = "Resume"}}{{$details = printf "/item/%s" $item.ID}}{{$hasArt = or (ne $item.Backdrop "") (ne $item.ShowBackdrop "") (ne (index .HomeArtwork $item.ID) "")}}
{{else if .Recent}}{{$item := index .Recent 0}}{{$id = $item.ArtworkID}}{{$title = $item.Title}}{{$href = $item.Href}}{{if $item.PlayHref}}{{$href = $item.PlayHref}}{{$details = $item.Href}}{{$label = "Play"}}{{end}}{{$meta = $item.Meta}}{{$hasArt = ne $item.ArtworkID ""}}{{end}}
{{if and $title $hasArt}}<section class="home-feature" data-palette-id="{{$id}}" aria-label="Featured title"><img {{if $hasArt}}src="/backdrop/{{$id}}"{{else}}hidden{{end}} alt="" width="1600" height="900" fetchpriority="high"><div class="home-feature-copy"><h2>{{$title}}</h2><p data-feature-meta>{{$meta}}</p><div class="watch-progress" data-watch-progress="{{$id}}" hidden><span data-watch-remaining></span><progress max="100" value="0" aria-label="Watch progress"></progress></div><div class="home-feature-actions"><a class="button" data-feature-action href="{{$href}}">{{if $details}}{{icon "play"}}{{end}}<span data-feature-label>{{t $label}}</span></a>{{if $details}}<a class="button quiet" href="{{$details}}">{{t "View details"}}</a>{{end}}</div></div></section>{{end}}

{{if .Continue}}<section class="home-shelf continue-shelf"><header><h2>Continue watching</h2><a href="/?view=history">See all</a></header><div class="grid rail resume-grid">{{range $position, $item := .Continue}}{{if lt $position 4}}<article class="card resume-card"><a class="resume-link" href="/watch/{{$item.ID}}" data-feature-id="{{$item.ID}}" data-feature-title="{{$item.Title}}" data-feature-meta="{{$item.Resume}}" data-feature-label="{{t "Resume"}}" data-feature-art="{{index $.HomeArtwork $item.ID}}">{{with (index $.HomeArtwork $item.ID)}}<img class="poster" src="/backdrop/{{$item.ID}}" alt="" width="400" height="225" decoding="async" {{if lt $position 2}}fetchpriority="high"{{else}}loading="lazy"{{end}}>{{else}}<div class="poster">{{icon "play"}}</div>{{end}}<h3>{{with (index $.HomeShowTitle $item.ID)}}{{.}} · {{end}}{{$item.Title}}</h3><small>{{$item.Resume}}</small><div class="watch-progress" data-watch-progress="{{$item.ID}}" hidden><span data-watch-remaining aria-hidden="true"></span><progress max="100" value="0" aria-label="Watch progress"></progress></div><span class="resume-action" aria-label="{{t "Resume"}}">{{icon "play"}}</span></a><form action="/continue-watching/{{$item.ID}}/remove" method="post"><button class="quiet" aria-label="{{t "Remove"}} {{$item.Title}} {{t "from Continue Watching"}}">Remove</button></form></article>{{end}}{{end}}</div></section>{{end}}
{{if .List}}<section class="home-shelf"><header><h2>My List</h2><a href="/?view=list">See all</a></header><div class="grid rail">{{range $position, $item := .List}}<a class="card" href="{{if eq $item.Kind "book"}}/book/{{$item.ID}}{{else if eq $item.Kind "video"}}/item/{{$item.ID}}{{else}}/watch/{{$item.ID}}{{end}}">{{with (index $.HomeArtwork $item.ID)}}<img class="poster" src="/art/{{.}}" alt="" width="400" height="600" decoding="async" {{if lt $position 2}}fetchpriority="high"{{else}}loading="lazy"{{end}}>{{else}}<div class="poster">{{icon "star"}}</div>{{end}}<h2>{{with (index $.HomeShowTitle $item.ID)}}{{.}} · {{end}}{{$item.Title}}</h2></a>{{end}}</div></section>{{end}}
{{if .Recent}}<section class="home-shelf"><header><h2>Recently added</h2></header><div class="grid rail recent-grid">{{range .Recent}}{{template "homeShelfCard" .}}{{end}}</div></section>{{end}}
{{if .Played}}<section class="home-shelf"><header><h2>Recently played</h2><a href="/?view=history">See all</a></header><div class="grid rail recent-grid">{{range .Played}}{{template "homeShelfCard" .}}{{end}}</div></section>{{end}}
<section class="destination-browser" aria-labelledby="browse-heading"><header><h2 id="browse-heading">Browse libraries</h2></header><div class="destination-grid">{{range .Destinations}}<section class="destination-group"><h3>{{.Name}}</h3>{{range .Destinations}}<a class="destination-card" href="{{.Href}}"><span class="destination-icon">{{icon .Icon}}</span><span><strong>{{.Name}}</strong><small>{{.Description}}</small></span><small>{{.CountLabel}}</small></a>{{end}}</section>{{else}}<div class="empty"><h2>Your Server is ready.</h2><p>Add a library folder in Settings to get started.</p>{{if .Owner}}<a class="button" href="/settings#libraries">Open Settings</a>{{end}}</div>{{end}}</div></section><p class="infinite-status" role="status" aria-live="polite" data-library-status></p>
{{else if eq .View "collections"}}<section class="curation-browser" aria-label="Collections"><div class="curation-groups"><section class="curation-group" aria-labelledby="custom-collections-heading"><header><div><h3 id="custom-collections-heading">Custom collections</h3><p>Created and curated on this Server.</p></div>{{if .Owner}}<form action="/collections" method="post"><input aria-label="New Collection name" name="name" placeholder="New Collection" maxlength="64" required><button>Create</button></form>{{end}}</header><div class="curation-grid">{{range .CustomCollectionCards}}<a class="curation-card" href="{{.Path}}"><span class="curation-poster">{{range $position, $preview := .Preview}}{{if or $preview.Artwork $preview.ShowArtwork}}<img src="/art/{{$preview.ID}}" alt="" width="400" height="600" decoding="async" {{if lt $position 2}}fetchpriority="high"{{else}}loading="lazy"{{end}}>{{else}}<span>{{icon "collection"}}</span>{{end}}{{end}}</span><strong>{{.Name}}</strong><small>{{.ItemLabel}}</small></a>{{else}}<p class="curation-empty">No custom collections yet.</p>{{end}}</div></section><section class="curation-group" aria-labelledby="default-collections-heading"><header><div><h3 id="default-collections-heading">Default collections</h3><p>Found in your media metadata.</p></div></header><div class="curation-grid">{{range .DefaultCollectionCards}}<a class="curation-card" href="{{.Path}}"><span class="curation-poster">{{range $position, $preview := .Preview}}{{if or $preview.Artwork $preview.ShowArtwork}}<img src="/art/{{$preview.ID}}" alt="" width="400" height="600" decoding="async" {{if lt $position 2}}fetchpriority="high"{{else}}loading="lazy"{{end}}>{{else}}<span>{{icon "collection"}}</span>{{end}}{{end}}</span><strong>{{.Name}}</strong><small>{{.ItemLabel}}</small></a>{{else}}<p class="curation-empty">No default collections found in your media metadata.</p>{{end}}</div></section></div></section>
{{else if eq .View "playlists"}}<section class="curation-browser" aria-label="Playlists"><header><form action="/playlists" method="post"><input aria-label="New playlist name" name="name" placeholder="New playlist" maxlength="64" required><button>Create</button></form><details><summary>New smart playlist</summary><form action="/smart-playlists" method="post"><input aria-label="Smart playlist name" name="name" placeholder="Smart playlist" maxlength="64" required><input aria-label="Match text" name="query" placeholder="Title, artist, genre"><label>Media <select name="kind"><option value="">Everything</option><option value="video">Video</option><option value="audio">Music and audiobooks</option><option value="book">Books and comics</option><option value="photo">Photos</option></select></label><label>Sort <select name="sort"><option value="title">Title</option><option value="added">Newest</option><option value="year">Year</option></select></label><button>Create smart playlist</button></form></details></header><div class="curation-grid">{{range .PlaylistCards}}<a class="curation-card" href="/playlist/{{.Name}}"><span class="curation-poster">{{range $position, $preview := .Preview}}{{if or $preview.Artwork $preview.ShowArtwork}}<img src="/art/{{$preview.ID}}" alt="" width="400" height="600" decoding="async" {{if lt $position 2}}fetchpriority="high"{{else}}loading="lazy"{{end}}>{{else}}<span>{{icon "playlist"}}</span>{{end}}{{end}}</span><strong>{{.Name}}</strong><small>{{.Mode}} · {{.ItemCount}} items</small>{{if .Rule}}<small>{{.Rule}}</small>{{end}}</a>{{else}}<div class="empty"><h2>No playlists yet.</h2><p>Build a queue by hand or let a smart playlist update itself.</p></div>{{end}}</div></section>
{{else}}<form class="browse-toolbar" action="/"><span><strong>{{.Total}}</strong> items</span><input type="hidden" name="view" value="{{.View}}">{{if .Query}}<input type="hidden" name="q" value="{{.Query}}">{{end}}<label>Sort <select name="sort"><option value="title" {{if eq .Sort "title"}}selected{{end}}>Title</option><option value="added" {{if eq .Sort "added"}}selected{{end}}>Newest</option><option value="year" {{if eq .Sort "year"}}selected{{end}}>Year</option></select></label><button class="quiet">Apply</button></form>{{if gt (len .Letters) 1}}<div class="title-jump" data-title-jump data-letter-count="{{len .Letters}}"><button class="quiet title-jump-open" type="button" data-title-jump-open aria-haspopup="dialog" aria-controls="title-jump-dialog">Jump to title{{if .Letter}}: {{.Letter}}{{end}}</button><nav class="letter-jump" aria-label="Jump to title" data-title-jump-index>{{template "letterLinks" .}}</nav><dialog id="title-jump-dialog" class="title-jump-dialog" data-title-jump-dialog aria-labelledby="title-jump-title"><header><h2 id="title-jump-title">Jump to title</h2><form method="dialog"><button class="quiet" aria-label="Close title jump">Close</button></form></header><nav class="title-jump-grid" aria-label="Choose a title letter">{{template "letterLinks" .}}</nav></dialog><output class="title-jump-preview" data-title-jump-preview aria-hidden="true" hidden></output></div>{{end}}{{template "libraryResults" .}}{{end}}
{{if .TMDB}}<footer><a href="https://www.themoviedb.org">TMDB</a> · This product uses the TMDB API but is not endorsed or certified by TMDB.</footer>{{end}}</main>{{languagePicker}}<script src="/static/supporter.js?v=11"></script></body></html>
{{define "homeShelfCard"}}<a class="card recent-card{{if .Stacked}} stacked{{end}}" href="{{.Href}}" data-feature-id="{{.ArtworkID}}" data-feature-title="{{.Title}}" data-feature-meta="{{.Meta}}" data-feature-art="{{.ArtworkID}}"{{if .Stacked}} aria-label="{{.Title}}, {{.Count}} {{.StackLabel}}; open show"{{end}}>{{if .Stacked}}<span class="poster recent-stack" aria-hidden="true"><span class="recent-stack-layer"></span><span class="recent-stack-layer"></span>{{if .ArtworkID}}<img src="/art/{{.ArtworkID}}" alt="" width="400" height="600" decoding="async" {{if .Priority}}fetchpriority="high"{{else}}loading="lazy"{{end}}>{{else}}<span class="recent-stack-placeholder">{{icon .PlaceholderIcon}}</span>{{end}}<span class="recent-stack-count">{{.Count}}</span></span>{{else}}{{if .ArtworkID}}<img class="poster" src="/art/{{.ArtworkID}}" alt="" width="400" height="600" decoding="async" {{if .Priority}}fetchpriority="high"{{else}}loading="lazy"{{end}}>{{else}}<div class="poster">{{icon .PlaceholderIcon}}</div>{{end}}{{end}}<h2>{{.Title}}</h2><small>{{.Meta}}</small></a>{{end}}
{{define "letterLinks"}}{{range .Letters}}<a href="{{.Href}}" data-title-letter="{{.Label}}" data-letter-count="{{.Count}}" aria-label="{{.Label}}, {{.Count}} {{if eq .Count 1}}title{{else}}titles{{end}}{{if .Current}}, selected; activate to show all titles{{end}}" hx-get="{{.Href}}" hx-target="#main" hx-select="#main" hx-swap="outerHTML show:top" hx-push-url="true"{{if .Current}} aria-current="true"{{end}}>{{.Label}}</a>{{end}}{{end}}
{{define "libraryResults"}}<section id="library">{{if .Shows}}<div class="library-group" data-library-group="shows">{{if ne .View "shows"}}<h2>Shows</h2>{{end}}<div class="grid">{{range $position, $show := .Shows}}<article class="card show-card"><a class="show-details" href="/show/{{$show.ID}}">{{if $show.ArtworkID}}<img class="poster" src="/art/{{$show.ArtworkID}}" alt="" width="400" height="600" decoding="async" {{if lt $position 2}}fetchpriority="high"{{else}}loading="lazy"{{end}}>{{else}}<div class="poster">{{icon "play"}}</div>{{end}}<h2>{{$show.Title}}</h2></a>{{with $show.Play}}<a class="show-play" href="{{.Stream}}" aria-label="{{.Label}} · {{$show.Title}} · {{.Title}}">{{icon "play"}}<span>{{.Label}}</span></a>{{end}}</article>{{end}}</div></div>{{end}}
{{if .Items}}<div class="library-group" data-library-group="movies">{{if ne .View "movies"}}<h2>Movies</h2>{{end}}<div class="grid">{{range $position, $item := .Items}}<a class="card" href="/item/{{$item.ID}}">{{if $item.Artwork}}<img class="poster" src="/art/{{$item.ID}}" alt="" width="400" height="600" decoding="async" {{if lt $position 2}}fetchpriority="high"{{else}}loading="lazy"{{end}}>{{else}}<div class="poster">{{icon "play"}}</div>{{end}}<h2>{{$item.Title}}{{if $item.Year}} · {{$item.Year}}{{end}}</h2></a>{{end}}</div></div>{{end}}
{{if .Albums}}<div class="library-group" data-library-group="albums"><h2>Albums</h2><div class="grid">{{range $position, $album := .Albums}}<a class="card" href="/album/{{$album.ID}}">{{if $album.ArtworkID}}<img class="poster" src="/art/{{$album.ArtworkID}}" alt="" width="400" height="600" decoding="async" {{if lt $position 2}}fetchpriority="high"{{else}}loading="lazy"{{end}}>{{else}}<div class="poster">{{icon "music"}}</div>{{end}}<h2>{{if $album.Artist}}{{$album.Artist}} · {{end}}{{$album.Title}}</h2></a>{{end}}</div></div>{{end}}
{{if .Music}}<div class="library-group" data-library-group="music"><h2>Music</h2><div class="grid">{{range $position, $item := .Music}}<a class="card" href="/watch/{{$item.ID}}">{{if $item.Artwork}}<img class="poster" src="/art/{{$item.ID}}" alt="" width="400" height="600" decoding="async" {{if lt $position 2}}fetchpriority="high"{{else}}loading="lazy"{{end}}>{{else}}<div class="poster">{{icon "music"}}</div>{{end}}<h2>{{if $item.Artist}}{{$item.Artist}} · {{end}}{{$item.Title}}</h2></a>{{end}}</div></div>{{end}}
{{if .Audiobooks}}<div class="library-group" data-library-group="audiobooks">{{if ne .View "audiobooks"}}<h2>Audiobooks</h2>{{end}}<div class="grid">{{range $position, $item := .Audiobooks}}<a class="card" href="/watch/{{$item.ID}}">{{if $item.Artwork}}<img class="poster" src="/art/{{$item.ID}}" alt="" width="400" height="600" decoding="async" {{if lt $position 2}}fetchpriority="high"{{else}}loading="lazy"{{end}}>{{else}}<div class="poster">{{icon "audiobook"}}</div>{{end}}<h2>{{if $item.Artist}}{{$item.Artist}} · {{end}}{{$item.Title}}</h2></a>{{end}}</div></div>{{end}}
{{if .Books}}<div class="library-group" data-library-group="books">{{if ne .View "books"}}<h2>Books</h2>{{end}}<div class="grid">{{range $position, $item := .Books}}<a class="card" href="/book/{{$item.ID}}">{{if $item.Artwork}}<img class="poster" src="/art/{{$item.ID}}" alt="" width="400" height="600" decoding="async" {{if lt $position 2}}fetchpriority="high"{{else}}loading="lazy"{{end}}>{{else}}<div class="poster">{{icon "book"}}</div>{{end}}<h2>{{$item.Title}}</h2></a>{{end}}</div></div>{{end}}
{{if .Photos}}<div class="library-group" data-library-group="photos">{{if ne .View "photos"}}<h2>Photos</h2>{{end}}<div class="grid">{{range $position, $item := .Photos}}<a class="card" href="/watch/{{$item.ID}}"><img class="poster photo" src="/art/{{$item.ID}}" alt="" width="400" height="300" decoding="async" {{if lt $position 2}}fetchpriority="high"{{else}}loading="lazy"{{end}}><h2>{{$item.Title}}</h2></a>{{end}}</div></div>{{end}}
{{if and (not .Shows) (not .Items) (not .Albums) (not .Music) (not .Audiobooks) (not .Books) (not .Photos)}}<div class="empty"><h2>{{if .Query}}No matching titles.{{else if eq .View "list"}}My List is empty.{{else if eq .View "history"}}Nothing played yet.{{else if eq .View "unwatched"}}Everything here is watched.{{else}}No media here yet.{{end}}</h2><p>{{if .Query}}Try another title, person, or genre.{{else if eq .View "list"}}Add a title from its detail page to keep it here.{{else if eq .View "history"}}Start a title and its progress will appear here.{{else if eq .View "unwatched"}}Browse Movies or Shows to replay something.{{else}}Add a library folder in Settings to get started.{{end}}</p>{{if eq .View "list"}}<a class="mode" href="/">Browse library</a>{{end}}</div>{{end}}</section>{{if or .Previous .Next}}<nav class="pagination" data-library-pagination aria-label="Library pages">{{if .Previous}}<a class="mode" href="{{.Previous}}">Previous</a>{{end}}{{if .Next}}<a class="mode" href="{{.Next}}" data-library-next>Load more</a>{{end}}</nav>{{end}}<p class="infinite-status" role="status" aria-live="polite" data-library-status></p>{{end}}`
}

var homeView = newLocalizedTemplate("home", homeWithPlaylistImport)

// Config describes the installation-owned paths used by the HTTP interface.
type Config struct {
	Lifecycle           context.Context
	MediaDir            string
	DataDir             string
	CacheDir            string
	FFmpeg              string
	FFprobe             string
	ProbeHardware       bool
	HardwareDevices     []string
	HardwareOS          string
	HardwareArch        string
	FPCalc              string
	RequireAuth         bool
	AuthURL             string
	WireGuardDir        string
	WireGuardEndpoint   string
	ScanInterval        time.Duration
	Metadata            MetadataConfig
	TMDBToken           string
	TMDBURL             string
	TMDBImageURL        string
	DLNAURL             string
	WatchRoomTTL        time.Duration
	QuickConnectTTL     time.Duration
	OIDC                OIDCConfig
	SAML                SAMLConfig
	MCP                 MCPConfig
	ProxyToken          string
	Notifications       NotificationConfig
	BackupDir           string
	BackupKey           string
	BackupInterval      time.Duration
	BackupRetention     int
	MaintenanceInterval time.Duration
	TranscodeCacheLimit int64
	Configuration       configuration.Snapshot
	SCIM                SCIMConfig
	InternetAccess      *remoteaccess.Manager
	TrustedHTTPS        *trustedhttps.Manager
	TrustedHTTPSCheck   func(context.Context, trustedhttps.Config) error
	Supporter           SupporterConfig
}

// New returns Kinosail's HTTP interface.
func New(config Config) http.Handler {
	return newApplication(config)
}

func newApplication(config Config) http.Handler { //nolint:funlen,cyclop,gocognit // The composition root validates defaults and wires every adapter to shared application services.
	lifecycle, managedLifecycle := prepareApplicationConfig(&config)
	stateDB, err := database.OpenContext(config.Lifecycle, config.DataDir, managedLifecycle)
	if err != nil {
		return unavailableApplication(config, "application state is unavailable")
	}
	closeApplicationDatabase(config.Lifecycle, stateDB, managedLifecycle)
	settings, updates, unavailable := initializeApplicationSettings(config, stateDB, managedLifecycle)
	if unavailable != "" {
		return unavailableApplication(config, unavailable)
	}
	supporter := newSupporterProgram(settings, config.Supporter)
	workloads := workload.New(workload.HeavyCapacity())
	metadata := newMetadataStore(config.Metadata, config.DataDir, config.CacheDir, stateDB)
	if metadata.err != nil {
		return unavailableApplication(config, "application state is unavailable")
	}
	probe := newMediaProbe(config.FFprobe)
	probe.ffmpeg, probe.cacheDir = config.FFmpeg, config.CacheDir
	probe.chapters = newChapterProvider(config.Metadata.ChaptersURL)
	markers := markerlogic.NewAnalyzer(markerlogic.Config{DataDir: config.DataDir, CacheDir: config.CacheDir, FFmpeg: config.FFmpeg, Fingerprint: config.FPCalc, Database: stateDB, Acquire: func(ctx context.Context) (func(), error) { return workloads.Acquire(ctx, workload.Background) }})
	probe.markers = markers
	legacyMetadata := sharedmetadata.NewTMDB(sharedmetadata.TMDBConfig{Token: config.TMDBToken, URL: config.TMDBURL, ImageURL: config.TMDBImageURL, CacheDir: config.CacheDir})
	index := &libraryIndex{Index: catalog.NewIndex(config.Lifecycle, settings.roots(), config.CacheDir, config.ScanInterval, func(ctx context.Context) (func(), error) {
		return workloads.Acquire(ctx, workload.Background)
	})}
	index.SetDecorator(func(items []library.Item) []library.Item {
		return metadata.apply(legacyMetadata.Enrich(config.Lifecycle, probe.decorate(config.Lifecycle, items)))
	})
	index.SetFrequency(settings.scanFrequency())
	if managedLifecycle {
		index.Schedule(config.Lifecycle)
		index.Watch(config.Lifecycle)
	}
	progress := newProgressStore(config.DataDir, stateDB)
	lists := newListStore(config.DataDir, stateDB)
	if progress.Err() != nil || lists.err != nil {
		return unavailableApplication(config, "application state is unavailable")
	}
	auth := newAuthentication(config.Lifecycle, config.DataDir, config.RequireAuth, config.AuthURL, settings, config.Notifications, stateDB, config.Configuration.Duration("logging.audit_retention"), config.Configuration.Duration("logging.playback_retention"))
	if config.InternetAccess != nil && config.InternetAccess.Status().Mode == "https" {
		// Keep LAN administration local. Passkey ceremonies still require their exact origin.
		auth.passkeys.redirectPages = false
	}
	if !auth.audit.Healthy() {
		return unavailableApplication(config, "activity journal is unavailable")
	}
	if auth.profiles.err != nil {
		if config.InternetAccess != nil {
			_ = config.InternetAccess.Kill()
		}
	}
	viewingImports := newViewingImportManager(lifecycle, config.DataDir, index, progress, lists, auth.profiles)
	homeAssistant, err := newHomeAssistant(settings, auth.profiles, index, progress, lists, lifecycle, config.AuthURL, rand.Reader)
	if err != nil {
		return unavailableApplication(config, "application state is unavailable")
	}
	agentConnections := newMCPConnections(config.AuthURL, config.DataDir, auth.profiles, stateDB)
	agentConnections.Configure(config.MCP)
	progress.SetAuditor(auth.audit)
	auth.audit.SetTitle(func(id string) string {
		item, _ := index.Find(id)
		return item.Title
	})
	auth.audit.SetSnapshot(settings.auditSnapshot)
	management := newOwnerAccess(config, auth)
	verifiedDirect := wireguard.OpenOptional(config.WireGuardDir, config.WireGuardEndpoint)
	hls := newHLS(config.Lifecycle, config.CacheDir, config.FFmpeg, index, probe, settings, workloads)
	frames := newTrickplay(config.CacheDir, config.FFmpeg, index)
	backups := newBackupManager(lifecycle, config.DataDir, config.BackupDir, config.BackupKey, config.BackupInterval, config.BackupRetention, workloads)
	maintenance := newMaintenanceManager(lifecycle, hls, backups, metadata, markers, config.MaintenanceInterval, config.TranscodeCacheLimit)
	markers.SetProbe(markerProbe(probe))
	if managedLifecycle {
		probe.scheduleSubtitles(config.Lifecycle, index, settings, workloads, maintenance)
		markers.Schedule(config.Lifecycle, index.AddAnalyzer, markerProbe(probe))
	}
	downloads := newDownloadManager(config.Lifecycle, config.CacheDir, config.FFmpeg, settings, workloads, probe)
	events := wireLiveEvents(index, downloads, homeAssistant)
	if downloads.Err() != nil {
		return unavailableApplication(config, "download cache is unavailable")
	}
	shares := newMediaShares(config.DataDir, stateDB, index)
	experience := newMediaExperienceStore(config.DataDir, stateDB)
	if experience.err != nil {
		return unavailableApplication(config, "media preferences are unavailable")
	}
	mux, rooms := http.NewServeMux(), newWatchRooms(config.WatchRoomTTL, index, auth)
	mux.HandleFunc("GET /static/htmx.min.js", serveScript(htmx))
	mux.HandleFunc("GET /static/hls.min.js", serveScript(hlsJS))
	mux.HandleFunc("GET /static/player.js", serveScript(playerJS))
	mux.HandleFunc("GET /static/downloads.js", serveScript(downloadsJS))
	mux.HandleFunc("GET /static/main.kinosail.bundle.js", serveScript(mainBundle))
	mux.HandleFunc("GET /static/pwa.js", serveScript(pwaJS))
	mux.HandleFunc("GET /static/supporter.js", serveScript(supporterJS))
	mux.HandleFunc("GET /static/supporter.css", serveSupporterStyle)
	mux.HandleFunc("GET /static/supporter/badges/{file...}", serveSupporterBadge)
	mux.HandleFunc("GET /static/theme.js", serveScript(themeJS))
	mux.HandleFunc("GET /static/quick-connect.js", serveScript(quickConnectJS))
	mux.HandleFunc("GET /static/connect.js", serveScript(connectJS))
	mux.HandleFunc("GET /static/app.css", serveStyle)
	mux.HandleFunc("GET /manifest.webmanifest", serveAsset(manifest, "application/manifest+json"))
	mux.HandleFunc("GET /static/manrope.woff2", serveAsset(webassets.Manrope, "font/woff2"))
	mux.HandleFunc("GET /static/icon.svg", serveAsset(icon, "image/svg+xml"))
	mux.HandleFunc("GET /static/icon-192.png", serveAsset(icon192, "image/png"))
	mux.HandleFunc("GET /static/icon-512.png", serveAsset(icon512, "image/png"))
	mux.HandleFunc("GET /static/icon-maskable-512.png", serveAsset(iconMaskable512, "image/png"))
	mux.HandleFunc("GET /static/apple-touch-icon.png", serveAsset(appleTouchIcon, "image/png"))
	mux.HandleFunc("GET /static/cinema-backdrop.jpg", serveAsset(cinemaBackdrop, "image/jpeg"))
	mux.HandleFunc("GET /service-worker.js", serveServiceWorker)
	mux.HandleFunc("GET /offline", serveOffline)
	mux.HandleFunc("GET /favicon.ico", serveAsset(icon, "image/svg+xml"))
	registerMediaShares(mux, shares, auth)
	quickConnect := registerIdentity(mux, auth, config.QuickConnectTTL, config.OIDC, config.SAML, config.SCIM)
	registerSettings(mux, settings, updates, index, progress, auth, verifiedDirect, config.InternetAccess, config.TrustedHTTPS, quickConnect, shares, hls, metadata, markers, backups, maintenance, viewingImports, homeAssistant, events, rooms, config.AuthURL)
	registerOwnerAccess(mux, auth, management, config.AuthURL)
	registerSupporter(mux, auth, supporter)
	registerAgentConnections(mux, auth, agentConnections, config.DataDir)
	viewingImports.register(mux, auth)
	mux.HandleFunc("GET /healthz", health)
	mux.HandleFunc("GET /language", showLanguage)
	mux.HandleFunc("POST /language", saveWebLanguage)
	registerPlayer(mux, index, progress, settings, lists, hls, probe, metadata, rooms, auth)
	registerMarkerAdmin(mux, auth, index, probe, markers)
	frames.register(mux)
	metadata.register(mux, auth, index)
	registerCollections(mux, index, lists, auth)
	if managedLifecycle {
		sharedmetadata.Schedule(config.Lifecycle, metadata.available(), index.AddAnalyzer, func(ctx context.Context) error { return metadata.refreshMissing(ctx, index) })
	}
	registerAPI(mux, apiServices{index, progress, lists, auth, settings, hls, probe, metadata, rooms, backups, downloads, verifiedDirect, maintenance, viewingImports, agentConnections, config.InternetAccess, config.TrustedHTTPS, shares, quickConnect, supporter, updates, homeAssistant, config.AuthURL, events})
	registerMediaExperience(mux, experience, index, progress)
	registerCasting(mux, auth, index, probe, hls, settings, config.AuthURL)
	mcpAdapter := registerMCPWithConnections(mux, config.MCP, auth, apiRouting(mux), agentConnections)
	startApplicationMCPHost(config, managedLifecycle, mcpAdapter, auth.profiles)
	if managedLifecycle {
		if err := serverdiscovery.Start(config.Lifecycle, settings.serverName(), config.AuthURL); err != nil {
			slog.Warn("Player local discovery unavailable", "error", err)
		}
	}
	registerJellyfin(mux, settings, index, progress, lists, auth, quickConnect, probe, hls, downloads)
	registerBrowsers(mux, index, progress, lists)
	registerFiles(mux, index, probe)
	downloads.registerWeb(mux, index)
	mux.Handle("POST /scan", auth.owner(catalog.RescanHandler(index.Index, localizedError)))
	mux.HandleFunc("GET /{$}", showHome(index, progress, lists, settings, updates, legacyMetadata.Active() || metadata.configured()))
	handler := withDLNA(maintenance.track(apiRouting(mux)), mux, auth, config, settings, updates, index)
	if management != nil && managedLifecycle {
		management.Attach(config.Lifecycle, handler)
	}
	return handler
}

func withDLNA(app http.Handler, appMux *http.ServeMux, auth *authentication, config Config, settings *settingsStore, updates *updateChecker, index *libraryIndex) http.Handler {
	root := http.NewServeMux()
	registerDLNA(root, config.Lifecycle, config.DLNAURL, settings, index)
	root.Handle("/", app)
	pattern := func(request *http.Request) string {
		_, matched := root.Handler(request)
		if matched == "/" {
			_, matched = appMux.Handler(request)
		}
		return matched
	}
	return trustedProxy(config.ProxyToken, allowedHost(config.AuthURL, config.Configuration.Strings("tls.hosts"), observeRequests(auth.audit, pattern, tripwirePublic(auth.audit, publicRequestLimits(security(localized(jellyfinCompatibility(settings, homeAssistantGate(settings, auth.protect(withApplicationShell(settings, updates, pattern, root), pattern))))))))))
}

func registerBrowsers(mux *http.ServeMux, index *libraryIndex, progress *progressStore, lists *listStore) {
	mux.HandleFunc("GET /show/{id}", browseShow(index, progress))
	mux.HandleFunc("GET /actor", browseActor(index, false))
	mux.HandleFunc("GET /api/v1/actor", browseActor(index, true))
	mux.HandleFunc("GET /album/{id}", browseAlbum(index))
	mux.HandleFunc("GET /book/{id}", browseBook(index, lists))
	registerReader(mux, index, progress)
}
