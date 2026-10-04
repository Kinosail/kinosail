package playerweb

const progressNoticeTemplate = `{{if or (eq .Kind "video") (eq .Kind "audio") (eq .Kind "audiobook")}}
<div class="primary-player-actions" data-progress-notice hidden>
<span role="status" aria-live="polite" data-progress-status
data-authentication="{{t "Your position is not saved. Reload this page to sign in again."}}"
data-policy="{{t "Your position is not saved. Reload this page to check access to this title."}}"
data-response="{{t "Your position is not saved. Reload this page to reconnect to Kinosail Server."}}"
data-unsaved="{{t "Your latest position is not saved. Retry while this page is open."}}"
data-watched="{{t "Watched status is not saved. Retry while this page is open."}}"
data-continue="{{t "Watched status is not saved. Retry or continue without saving."}}"
data-saving="{{t "Saving progress…"}}"></span>
<button class="quiet" type="button" data-progress-retry>{{t "Retry saving position"}}</button>
<button class="quiet" type="button" data-progress-continue hidden>{{t "Continue without saving"}}</button>
</div>{{end}}`
