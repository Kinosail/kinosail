package server

import "strings"

func homeWithPlaylistImport() string {
	return strings.Replace(homeTemplateSource(),
		`</form><details><summary>New smart playlist`,
		`</form><details><summary>Import playlist</summary><form action="/playlists" method="post"><label for="playlist-document">Playlist JSON</label><textarea id="playlist-document" name="document" rows="8" maxlength="1048576" required></textarea><button>Import playlist</button></form><p>Paste a Kinosail playlist export.</p></details><details><summary>New smart playlist`, 1)
}

func playlistHTMLWithExport() string {
	return strings.Replace(playlistHTML,
		`<h1>{{.Name}}</h1><p>`,
		`<h1>{{.Name}}</h1>{{if not .Smart}}<p><a href="/api/v1/playlists/{{.Name}}?format=kinosail" download>Export playlist</a></p>{{end}}<p>`, 1)
}
