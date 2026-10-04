package playerweb

import "strings"

// Audio queue controls use existing page styles; item actions remain Server-rendered.
func audioQueueTemplate(source string) string {
	source = strings.Replace(source, `<audio aria-label="{{.Title}}"`, `{{if .Queue}}<img class="viewer" width="320" height="320" data-now-playing-artwork alt="" hidden>{{end}}<audio aria-label="{{.Title}}" data-track="{{.Track}}"`, 1)
	source = strings.Replace(source, `<p class="title-byline">`, `<p class="title-byline" data-now-playing-byline>`, 1)
	return strings.Replace(source, `<p class="playback-device-status"`, audioQueueControls+`<p class="playback-device-status"`, 1)
}

const audioQueueControls = `{{if .Queue}}<div class="player-actions" data-audio-queue-controls aria-busy="true"><button class="quiet" type="button" data-audio-previous disabled>{{t "Previous track"}}</button><span role="status" aria-live="polite" data-audio-queue-status data-track-label="{{t "Track"}}" data-of-label="{{t "of"}}">{{t "Loading queue…"}}</span><button class="quiet" type="button" data-audio-next disabled>{{t "Next track"}}</button><button class="quiet" type="button" data-audio-queue-retry hidden>{{t "Retry loading queue"}}</button></div><div class="player-actions" data-current-track-actions hidden><a data-current-track-details>{{t "Current track details and actions"}}</a></div>{{end}}`
