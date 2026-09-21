package server

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/catalogapi"
	"github.com/MikeO7/kinosail/packages/library"
	sharedplayback "github.com/MikeO7/kinosail/packages/playerweb"
)

const playerHTML = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover"><meta name="theme-color" content="#0b0d0b"><link rel="manifest" href="/manifest.webmanifest"><link rel="icon" href="/static/icon.svg?v=8"><link rel="apple-touch-icon" href="/static/apple-touch-icon.png?v=8"><script src="/static/theme.js?v=electric-1"></script><link rel="stylesheet" href="/static/app.css?v=electric-1"><script defer src="/static/main.kinosail.bundle.js?v=12"></script>{{if ne .Kind "photo"}}{{if .HLS}}<script defer src="/static/hls.min.js?v=1.7.1"></script>{{end}}<script defer src="/static/downloads.js?v=3"></script><script defer src="/static/player.js?v=34"></script>{{end}}
<title>{{.Title}} · Kinosail Player</title></head><body class="player-page{{if and (eq .Kind "video") (or .Backdrop .Artwork .ShowBackdrop .ShowArtwork)}} has-media-backdrop{{end}}" data-viewer-profile="{{.ViewerProfile}}" data-cast-connecting="{{t "Connecting…"}}" data-cast-device="{{t "Playing on device"}}" data-cast-here="{{t "Playing here"}}" data-cast-none="{{t "No device selected"}}" data-room-leading="{{t "Room connected · you lead"}}" data-room-following="{{t "Room connected · following leader"}}" data-room-disconnected="{{t "Room disconnected · reload to reconnect"}}" data-sleep-stops="{{t "Stops in {minutes} minutes"}}" data-sleep-ended="{{t "Sleep timer ended"}}">{{if and (eq .Kind "video") (or .Backdrop .Artwork .ShowBackdrop .ShowArtwork)}}<img class="media-backdrop" src="/backdrop/{{.ID}}" alt="" fetchpriority="low" decoding="async">{{end}}<main class="player-shell last-light" data-palette-id="{{.ID}}" {{if .ReplayGainTrack}}data-replaygain-track="{{.ReplayGainTrack}}"{{end}} {{if .ReplayGainAlbum}}data-replaygain-album="{{.ReplayGainAlbum}}"{{end}}><a class="back" href="/">{{icon "back"}} Library</a>
<div class="media-stage{{if eq .Kind "video"}} is-busy{{end}}">{{if eq .Kind "photo"}}<img class="viewer" src="{{.Source}}" alt="{{.Title}}">{{else if or (eq .Kind "audio") (eq .Kind "audiobook")}}<audio aria-label="{{.Title}}" controls preload="metadata" data-title="{{.Title}}" {{if .Artwork}}data-artwork="/art/{{.ID}}"{{end}} data-start="{{.Start}}" data-progress="/progress/{{.ID}}" {{if .Queue}}data-queue="{{.Queue}}"{{end}} x-webkit-airplay="allow" {{if .Room}}data-room="{{.Room}}" data-room-leader="{{.RoomLeader}}" data-room-media="{{.ID}}"{{end}} src="{{.Source}}"></audio>{{else}}<video aria-label="{{.Title}}" id="player-media" controls playsinline {{if .Resume}}data-autoplay{{else}}autoplay{{end}} preload="{{if .HLS}}auto{{else}}metadata{{end}}" {{if or .Backdrop .ShowBackdrop}}poster="/backdrop/{{.ID}}"{{else if .Artwork}}poster="/art/{{.ID}}"{{end}} data-title="{{.Title}}" {{if .Artwork}}data-artwork="/art/{{.ID}}"{{end}} data-start="{{.Start}}" data-progress="/progress/{{.ID}}{{if .PlaybackToken}}?playbackToken={{.PlaybackToken}}{{end}}" data-playback-token="{{.PlaybackToken}}" data-auto-skip="{{.AutoSkip}}" x-webkit-airplay="allow" {{if .Room}}data-room="{{.Room}}" data-room-leader="{{.RoomLeader}}" data-room-media="{{.ID}}"{{end}} {{if .Next}}data-next="/watch/{{.Next}}"{{end}} {{if .HLS}}data-hls="{{.Source}}" data-duration="{{.Duration}}" data-direct="{{.DirectSource}}" data-direct-type="{{.DirectType}}"{{else}}src="{{.Source}}{{if .Resume}}#t={{.Start}}{{end}}" {{if .AdaptiveSource}}data-adaptive="{{.AdaptiveSource}}" data-duration="{{.Duration}}" data-direct="{{.DirectSource}}" data-direct-type="{{.DirectType}}"{{end}} {{if .FallbackSource}}data-fallback="{{.FallbackSource}}"{{end}}{{end}}>{{range .Tracks}}<track {{if .Default}}default {{end}}kind="{{if .Kind}}{{.Kind}}{{else}}subtitles{{end}}" label="{{.Label}}" data-subtitle-source="{{.Source}}" {{if .Language}}srclang="{{.Language}}"{{end}}>{{end}}</video><div class="player-stage-toolbar"><strong>{{.Title}}</strong></div><div class="player-controls" data-player-controls hidden><button class="player-center-control" type="button" aria-label="Play" data-player-toggle><span aria-hidden="true" data-play-icon></span></button><button class="player-center-control seek-back" type="button" aria-label="Go back 10 seconds" data-player-back>10</button><button class="player-center-control seek-forward" type="button" aria-label="Go forward 10 seconds" data-player-forward>10</button><div class="player-control-dock"><label class="player-scrubber"><span class="sr-only">Seek</span><input type="range" min="0" max="100" value="0" step="0.1" data-player-seek></label><div class="player-control-row"><button type="button" aria-label="Play" data-player-toggle><span aria-hidden="true" data-play-icon></span></button><button type="button" aria-label="Go back 10 seconds" data-player-back><span aria-hidden="true">−10</span></button><button type="button" aria-label="Go forward 10 seconds" data-player-forward><span aria-hidden="true">+10</span></button><button type="button" aria-label="Mute" aria-pressed="false" data-player-mute><svg aria-hidden="true" viewBox="0 0 24 24"><path d="M4 9v6h4l5 4V5L8 9H4m12.5 3a4 4 0 0 0-2-3.46v6.92a4 4 0 0 0 2-3.46m-2-7v2.06a8 8 0 0 1 0 15.88V21a10 10 0 0 0 0-18Z"/></svg></button><label class="player-volume"><span class="sr-only">Volume</span><input type="range" min="0" max="1" value="1" step="0.05" data-player-volume></label><output data-player-time>0:00 / 0:00</output><span class="player-control-spacer"></span><button type="button" aria-label="Subtitles" aria-pressed="false" data-player-captions>CC</button><button type="button" aria-label="Settings" aria-controls="player-settings" aria-expanded="false" data-player-settings><svg aria-hidden="true" viewBox="0 0 24 24"><path d="M19.4 13a7.8 7.8 0 0 0 .05-1 7.8 7.8 0 0 0-.05-1l2.1-1.65-2-3.46-2.5 1a7.6 7.6 0 0 0-1.74-1L14.9 3h-4l-.4 2.89a7.6 7.6 0 0 0-1.74 1l-2.5-1-2 3.46L6.4 11a7.8 7.8 0 0 0-.05 1 7.8 7.8 0 0 0 .05 1l-2.1 1.65 2 3.46 2.5-1a7.6 7.6 0 0 0 1.74 1l.4 2.89h4l.4-2.89a7.6 7.6 0 0 0 1.74-1l2.5 1 2-3.46L19.4 13M13 15.5A3.5 3.5 0 1 1 13 8a3.5 3.5 0 0 1 0 7.5Z"/></svg></button><button type="button" aria-label="Theater" aria-controls="player-media" aria-keyshortcuts="T" aria-pressed="false" data-theater><svg aria-hidden="true" data-theater-label viewBox="0 0 24 24"><path d="M3 5h18v14H3V5m2 2v10h14V7H5Z"/></svg></button><button type="button" aria-label="Enter fullscreen" data-player-fullscreen><svg aria-hidden="true" viewBox="0 0 24 24"><path d="M4 4h6v2H6v4H4V4m10 0h6v6h-2V6h-4V4M4 14h2v4h4v2H4v-6m14 0h2v6h-6v-2h4v-4Z"/></svg></button></div></div></div><div class="player-settings" id="player-settings" hidden><header><strong>Playback settings</strong><button class="quiet" type="button" aria-label="Close playback settings" data-player-settings-close>Close</button></header>{{if or .HLS .AdaptiveSource}}<label data-quality-control hidden><span>Quality</span><select data-quality aria-label="Stream quality"><option value="auto">Auto</option></select><small role="status" aria-live="polite" data-quality-state>Auto</small></label>{{end}}{{if gt (len .Audio) 1}}<label><span>Audio</span><select data-audio-track aria-label="Audio track">{{range .Audio}}<option value="{{.Index}}" {{if eq .Index $.Plan.AudioIndex}}selected{{end}}>{{.Label}}</option>{{end}}</select><small role="status" data-audio-status></small></label>{{end}}<label><span>Subtitles</span><select data-subtitles aria-label="Subtitles"><option value="off">Off</option>{{range $index, $track := .Tracks}}<option value="{{$index}}" {{if $track.Default}}selected{{end}}>{{$track.Label}}</option>{{end}}</select><small role="status" data-subtitle-status hidden></small></label><p>Apple’s native controls remain available in iPhone fullscreen.</p></div><div class="player-buffer" role="status" aria-live="polite" data-player-status><span class="buffer-skeleton" aria-hidden="true"></span><span data-player-message>Loading video…</span><progress hidden max="100" value="0" aria-label="Video buffered" data-buffered>0%</progress></div>{{if .Markers}}<div class="marker-actions">{{range .Markers}}<button hidden type="button" data-marker="{{.Type}}" data-start="{{.Start}}" data-seek="{{.End}}" data-marker-source="{{.Source}}">Skip {{.Label}}</button>{{end}}</div>{{end}}{{end}}</div>
<p class="playback-device-status"><span role="status" aria-live="polite" data-cast-state>Available devices use a direct connection to this Server.</span></p><div class="title-block"><h1>{{.Title}}{{if .Year}} <small>{{.Year}}</small>{{end}}{{if .Rating}} <small>{{.Rating}}</small>{{end}}</h1>
{{if .Artist}}<p class="title-byline">{{.Artist}}{{if .Album}} · {{.Album}}{{end}}{{if .Track}} · Track {{.Track}}{{end}}</p>{{end}}{{if .Tagline}}<p class="title-tagline"><em>{{.Tagline}}</em></p>{{end}}{{if .Plot}}<p class="title-summary">{{.Plot}}</p>{{end}}{{if or .Genres .Director .Studio}}<p class="title-facts">{{if .Genres}}<span>{{.Genres}}</span>{{end}}{{if .Director}}<span>Directed by {{.Director}}</span>{{end}}{{if .Studio}}<span>{{.Studio}}</span>{{end}}</p>{{end}}</div>
{{if .Audiobook}}<div class="player-actions"><label>Playback speed <select data-playback-rate><option value="0.75">0.75×</option><option value="1" selected>1×</option><option value="1.25">1.25×</option><option value="1.5">1.5×</option><option value="1.75">1.75×</option><option value="2">2×</option></select></label><label>Sleep timer <select data-sleep-timer><option value="0">Off</option><option value="15">15 minutes</option><option value="30">30 minutes</option><option value="45">45 minutes</option><option value="60">60 minutes</option></select></label><span role="status" aria-live="polite" data-sleep-state></span></div>{{end}}
<div class="primary-player-actions"><div class="primary-action-row"><form action="/list/{{.ID}}" method="post"><button name="listed" value="{{if .Listed}}false{{else}}true{{end}}">{{if .Listed}}Remove from{{else}}Add to{{end}} My List</button></form>{{if ne .Kind "photo"}}<form action="/watched/{{.ID}}" method="post"><button class="quiet" name="watched" value="{{if .Watched}}false{{else}}true{{end}}">Mark {{if .Watched}}{{t "unwatched"}}{{else}}{{t "watched"}}{{end}}</button></form>{{end}}</div>{{if or .ModeURL .CanDownload}}<details class="more-player-actions"><summary>Playback &amp; downloads</summary><div>{{if .ModeURL}}<a class="mode" href="{{.ModeURL}}" aria-label="{{t .ModeLabel}}"><span>Playback</span><strong>{{t .ModeLabel}}</strong></a>{{end}}{{if .CanDownload}}<a class="mode" href="/download/{{.ID}}" aria-label="Download original"><span>Download</span><strong>Original file</strong></a>{{if .OfflineQuality}}<form action="/offline/{{.ID}}" method="post"><button class="quiet" name="quality" value="{{.OfflineQuality}}" aria-label="Prepare {{.OfflineQuality}} offline"><span>Offline</span><strong>Prepare {{.OfflineQuality}}</strong></button></form><a class="mode" href="/offline-downloads" aria-label="Offline downloads"><span>Offline</span><strong>Manage downloads</strong></a>{{end}}{{end}}</div></details>{{end}}</div>
{{if .Chapters}}<details class="chapters"><summary><span>Chapters</span><small>{{len .Chapters}}</small></summary><ol class="chapter-list">{{range .Chapters}}<li><button type="button" data-chapter data-start="{{.Start}}" data-end="{{.End}}" data-seek="{{.Start}}"><span>{{.Title}}</span><time>{{.Timestamp}}</time></button></li>{{end}}</ol></details>{{end}}

{{if or .Playlists .Collections}}<details class="curation-menu player-disclosure" data-curation-menu><summary><span>Add to playlist or collection</span><small>Choose where this title appears</small></summary><label class="curation-search">Search playlists and collections<input type="search" maxlength="200" autocomplete="off" placeholder="Search by name…" data-curation-search></label><div class="curation-options">{{if .Playlists}}<section><h2>Playlists</h2>{{range .Playlists}}<form action="/playlist/{{.Name}}/{{$.ID}}" method="post" data-curation-option><button name="included" value="{{if .Included}}false{{else}}true{{end}}" aria-label="{{if .Included}}Remove from{{else}}Add to{{end}} playlist · {{.Name}}"><span>{{.Name}}</span><small>{{if .Included}}Added{{else}}Add{{end}}</small></button></form>{{end}}</section>{{end}}{{if and .Owner .Collections}}<section><h2>Collections</h2>{{range .Collections}}<form action="/collection/{{.Name}}/{{$.ID}}" method="post" data-curation-option><button name="included" value="{{if .Included}}false{{else}}true{{end}}" aria-label="{{if .Included}}Remove from{{else}}Add to{{end}} Collection · {{.Name}}"><span>{{.Name}}</span><small>{{if .Included}}Added{{else}}Add{{end}}</small></button></form>{{end}}</section>{{end}}</div><p class="curation-empty" role="status" hidden data-curation-empty>No playlists or collections found.</p></details>{{end}}
{{if ne .Kind "photo"}}<details class="playback-tools player-disclosure"><summary><span>Watch together</span><small>Shared viewing room</small></summary><div class="player-actions"><button class="quiet" type="button" data-cast>Play on device</button></div>{{if .Room}}<div class="player-actions"><a class="mode" href="/room/{{.Room}}">Invite link</a><span role="status" aria-live="polite" data-room-state>{{if .RoomLeader}}You lead this room{{else}}Following the room leader{{end}}</span>{{if .RoomLeader}}<label>Play <select data-room-choice>{{range .RoomItems}}<option value="{{.ID}}" {{if eq .ID $.ID}}selected{{end}}>{{.Title}}</option>{{end}}</select></label>{{end}}</div>{{else}}<form class="player-actions" action="/watch-together" method="post" data-room-create><input type="hidden" name="media" value="{{.ID}}"><input type="hidden" name="seconds" value="0"><button class="quiet">Start Watch Together</button></form>{{end}}</details>{{end}}
{{if .Cast}}<section class="cast-section"><header><span class="eyebrow">People</span><h2>Cast</h2></header><div class="grid rail">{{range $index, $person := .Cast}}<a class="card" href="/actor?name={{urlquery $person.Name}}">{{if $person.Image}}<img class="poster" src="/person/{{$.ID}}/{{$index}}" alt="{{$person.Name}}" loading="lazy">{{else}}<div class="poster">●</div>{{end}}<h2>{{$person.Name}}</h2>{{if $person.Role}}<small>{{$person.Role}}</small>{{end}}</a>{{end}}</div></section>{{end}}
{{if .Owner}}<details class="owner-tools player-disclosure"><summary><span>Manage media</span><small>Owner tools</small></summary><div class="owner-options"><details><summary>Edit metadata</summary>{{if .CanRefreshMetadata}}<form action="/metadata/{{.ID}}/refresh" method="post"><button>Refresh from TMDB</button></form>{{end}}<form action="/metadata/{{.ID}}" method="post"><label>Title <input name="title" value="{{.Title}}" maxlength="200" required></label><label>Year <input name="year" value="{{.Year}}" maxlength="4"></label><label>Rating <input name="rating" value="{{.Rating}}" maxlength="32"></label><label>Tagline <input name="tagline" value="{{.Tagline}}" maxlength="300"></label><label>Genres <input name="genres" value="{{.Genres}}" maxlength="500"></label><label>Plot <textarea name="plot" maxlength="5000">{{.Plot}}</textarea></label><button>Save metadata</button></form></details>{{if eq .Kind "video"}}<details><summary>Edit skip markers</summary>{{range .AdminMarkers}}<form action="/markers/{{$.ID}}" method="post"><label>Type <select name="type"><option value="intro" {{if eq .Type "intro"}}selected{{end}}>Intro</option><option value="recap" {{if eq .Type "recap"}}selected{{end}}>Recap</option><option value="commercial" {{if eq .Type "commercial"}}selected{{end}}>Commercial</option><option value="outro" {{if eq .Type "outro"}}selected{{end}}>Outro</option><option value="credits" {{if eq .Type "credits"}}selected{{end}}>Credits</option></select></label><label>Start in seconds <input name="start" type="number" min="0" step="0.1" value="{{.Start}}" required></label><label>End in seconds <input name="end" type="number" min="0" step="0.1" value="{{.End}}" required></label><button>Save marker</button></form><form action="/markers/{{$.ID}}/remove" method="post"><input type="hidden" name="type" value="{{.Type}}"><button>Remove {{.Label}} marker</button></form>{{end}}<details><summary>Add skip marker</summary><form action="/markers/{{.ID}}" method="post"><label>Type <select name="type"><option value="intro">Intro</option><option value="recap">Recap</option><option value="commercial">Commercial</option><option value="outro">Outro</option><option value="credits">Credits</option></select></label><label>Start in seconds <input name="start" type="number" min="0" step="0.1" required></label><label>End in seconds <input name="end" type="number" min="0" step="0.1" required></label><button>Save marker</button></form></details></details>{{end}}</div></details>{{end}}
<details class="media-information player-disclosure"><summary><span>Media information</span><small>File and stream details</small></summary><p>{{.Container}} · {{.FileSize}}{{if .MediaDetails}} · {{.MediaDetails}}{{end}}{{if .Subtitle}} · Subtitle tracks: {{len .Tracks}}{{end}} · {{if .HLS}}Compatible version{{else}}Original file{{end}}</p></details></main>
</body></html>`

var playerView = newLocalizedTemplate("player", playerTemplate(playerHTML))

func audioQueue(request *http.Request, index *libraryIndex, id string) ([]library.Item, bool) {
	items, _ := visibleLibrary(request, index)
	return catalogapi.AudioQueue(items, id)
}

func playerTemplate(template string) string {
	template = strings.Replace(template, `/static/downloads.js?v=3`, `/static/downloads.js?v=26`, 1)
	template = strings.Replace(template, `preload="{{if .HLS}}auto{{else}}metadata{{end}}"`, `preload="auto"`, 1)
	template = strings.Replace(template, `data-sleep-ended="{{t "Sleep timer ended"}}"`, `data-sleep-ended="{{t "Sleep timer ended"}}" data-offline-copy="{{t "Offline copy"}}" data-offline-description="{{t "Verified file stored on this device."}}" data-offline-ready="{{t "Ready offline on this device"}}" data-offline-unavailable="{{t "Offline copy is unavailable."}}" data-offline-audio="{{t "Audio is fixed in this downloaded copy."}}" data-playback-method="{{t "Playback method"}}" data-open-playback-settings="{{t "Open playback settings."}}"`, 1)
	template = strings.Replace(template, `/static/player.js?v=34`, `/static/player.js?v=77`, 1)
	return tvPlayerTemplate(sharedplayback.PlayerTemplate(template))
}

func registerPlayer(mux *http.ServeMux, index *libraryIndex, progress *progressStore, settings *settingsStore, lists *listStore, hls *hlsManager, probe *mediaProbe, metadata *metadataStore, rooms *watchRoomAdapter, auth *authentication) {
	listHandlers := newListHandlers(index, lists)
	progressHandlers := catalog.NewProgressHTTPHandlers(progress, index, timelineFromPlaybackToken, localizedError, localizedNotFound, apiStoreStatus)
	mux.HandleFunc("GET /item/{id}", showItemDetails(index, progress, lists))
	mux.HandleFunc("POST /item/{id}/list", saveDetailsList(index, lists))
	mux.HandleFunc("GET /watch/{id}", watch(index, progress, settings, lists, probe, metadata, rooms))
	mux.HandleFunc("GET /hls/{id}/{file...}", hls.serve)
	mux.HandleFunc("GET /hls/{id}/audio/{track}/{file...}", hls.serve)
	mux.HandleFunc("POST /progress/{id}", progressHandlers.Save())
	mux.HandleFunc("POST /continue-watching/{id}/remove", progressHandlers.Dismiss())
	mux.HandleFunc("POST /watched/{id}", progressHandlers.SaveWatched())
	mux.HandleFunc("POST /list/{id}", listHandlers.SaveList)
	mux.HandleFunc("POST /playlists", listHandlers.CreatePlaylist)
	mux.HandleFunc("POST /smart-playlists", listHandlers.CreateSmart)
	mux.HandleFunc("GET /playlist/{name}", browsePlaylist(index, lists))
	mux.HandleFunc("POST /playlist/{name}/items/{id}", managePlaylistItem(index, lists))
	mux.HandleFunc("POST /playlist/{name}/order", orderPlaylist(index, lists))
	mux.HandleFunc("POST /playlist/{name}/delete", listHandlers.DeletePlaylist)
	mux.HandleFunc("POST /playlist/{name}/{id}", listHandlers.SavePlaylist)
	rooms.register(mux, index, auth)
}

type (
	playerData     = sharedplayback.PlayerData
	playlistOption = sharedplayback.PlaylistOption
	subtitleTrack  = sharedplayback.SubtitleTrack
)

func watch(index *libraryIndex, progress *progressStore, settings *settingsStore, lists *listStore, probe *mediaProbe, metadata *metadataStore, rooms *watchRoomAdapter) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		item, found := visibleItem(request, index, request.PathValue("id"))
		if !found {
			localizedNotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		policy := writer.Header().Get("Content-Security-Policy")
		writer.Header().Set("Content-Security-Policy", strings.Replace(policy, "script-src 'self'", "script-src 'self' https://www.gstatic.com", 1))
		data := buildPlayerData(request, item, index, progress, settings, lists, probe, metadata)
		data.Room, data.RoomLeader, data.RoomItems = rooms.player(request, index, item)
		if err := playerView.Execute(writer, request, data); err != nil {
			localizedError(writer, request, err.Error(), http.StatusInternalServerError)
		}
	}
}

func buildPlayerData(request *http.Request, item library.Item, index *libraryIndex, progress *progressStore, settings *settingsStore, lists *listStore, probe *mediaProbe, metadata *metadataStore) playerData { //nolint:cyclop,gocognit // The player projection assembles each supported playback capability once.
	state, viewer := progress.Get(request, item.ID), currentViewer(request)
	data := sharedplayback.NewPlayerData(item, viewer.ID, randID())
	data.Start = state.Seconds
	data.Watched = state.Watched
	data.Listed = lists.Has(request, item.ID)
	data.CanDownload = viewer.Owner || viewer.Downloads
	data.CanTranscode = viewerPlaybackPolicy(viewer).AllowTranscode
	data.Next = autoNext(request, settings, index, item)
	data.AutoSkip = strings.Join(settings.autoSkip(), ",")
	data.FileSize = byteSize(item.Size)
	data.DefaultSubtitles = settings.subtitlesDefault()
	data.HomeAssistant = settings.homeAssistant()
	data.Audiobook = item.Kind == "audiobook"
	if supportsOffline(item) {
		data.OfflineQuality = optimizedDownloadLabel(item)
	}
	if queue, ok := audioQueue(request, index, item.ID); ok && len(queue) > 1 {
		data.Queue = "/api/v1/audio/" + item.ID + "/queue"
	}
	media := probe.inspect(request.Context(), item)
	data.ApplyMedia(media)
	for _, name := range lists.editablePlaylistNames(request) {
		data.Playlists = append(data.Playlists, playlistOption{Name: name, Included: len(lists.Playlist(request, name, []library.Item{item})) == 1})
	}
	items, _ := visibleLibrary(request, index)
	for _, name := range lists.collectionNames(items) {
		data.Collections = append(data.Collections, playlistOption{Name: name, Included: len(lists.collection(name, []library.Item{item})) == 1})
	}
	data.Owner, data.CanRefreshMetadata = viewer.Owner, viewer.Owner && metadata.configured()
	data.Tracks = playbackSubtitles(item, media, settings.subtitleLanguage(), data.DefaultSubtitles)
	applyPlayback(request, settings, mediaFactsFor(item, media), &data)
	setPlaybackSession(request, data.PlaybackSession)
	data.Finalize(request)
	return data
}

func subtitleLabel(media, subtitle string) string {
	label := strings.TrimPrefix(strings.TrimSuffix(filepath.Base(subtitle), filepath.Ext(subtitle)), strings.TrimSuffix(filepath.Base(media), filepath.Ext(media))+".")
	if label == strings.TrimSuffix(filepath.Base(media), filepath.Ext(media)) {
		return "Subtitles"
	}
	return strings.ToUpper(strings.ReplaceAll(label, ".", " · "))
}

func byteSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	}
	if size < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(size)/(1024*1024*1024))
}

func autoNext(request *http.Request, settings *settingsStore, index *libraryIndex, item library.Item) string {
	if settings.autoplay() {
		return nextEpisode(request, index, item)
	}
	return ""
}

func nextEpisode(request *http.Request, index *libraryIndex, current library.Item) string {
	items, _ := visibleLibrary(request, index)
	_, shows := library.Organize(items)
	for _, show := range shows {
		for position, episode := range show.Episodes {
			if episode.ID == current.ID && position+1 < len(show.Episodes) {
				return show.Episodes[position+1].ID
			}
		}
	}
	return ""
}
