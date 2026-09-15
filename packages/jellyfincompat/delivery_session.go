package jellyfincompat

import (
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

// PlaySession returns one request-bound session after Player's authorization checks.
func (module DeliveryHTTP) PlaySession(request *http.Request, itemID string) (DeliverySession, bool) {
	id, valid := StrictQuery(request.URL.Query(), "playSessionId", 256)
	if !validRequiredValue(id, 256) || !valid {
		return nil, false
	}
	value, found := module.LoadSession(id)
	session, sessionValid := value.(DeliverySession)
	if !sessionValid {
		return nil, false
	}
	viewer := module.CurrentViewer(request)
	allowed := playback.AuthorizeJellyfinPlaySession(value, found, itemID, viewer.ID, module.Public(request), module.now(), module.ActiveRevision)
	return session, allowed
}

// PlaybackContext resolves authenticated content or validates a public play session.
func (module DeliveryHTTP) PlaybackContext(request *http.Request) (library.Item, DeliveryViewer, bool) { //nolint:cyclop // Each authorization fact remains explicit.
	id := request.PathValue("id")
	if !validRequiredValue(id, 128) {
		return library.Item{}, DeliveryViewer{}, false
	}
	id = RawID(id)
	playID, queryValid := StrictQuery(request.URL.Query(), "playSessionId", 256)
	if !queryValid {
		return library.Item{}, DeliveryViewer{}, false
	}
	current := module.CurrentViewer(request)
	if current.ID != "" {
		item, found := module.Playback.VisibleItem(request, id)
		return item, current, found
	}
	if !validRequiredValue(playID, 256) {
		return library.Item{}, DeliveryViewer{}, false
	}
	item, itemFound := module.FindItem(id)
	value, sessionFound := module.LoadSession(playID)
	session, sessionValid := value.(DeliverySession)
	profileID, revision := "", uint64(0)
	if sessionValid {
		profileID, revision = session.JellyfinProfile()
	}
	viewer, profileActive := module.FindViewer(profileID)
	now := module.now()
	rejection := PlaybackRejection{
		ItemID: id, ItemFound: itemFound, SessionFound: sessionFound, SessionValid: sessionValid, ProfileActive: profileActive,
		ProfileRevisionMatch: profileActive && revision == viewer.Revision,
		SessionItemMatch:     sessionValid && session.JellyfinItemID() == id,
		PublicMatch:          sessionValid && module.Public(request) == session.JellyfinPublic(),
		Expired:              !sessionValid || now.After(session.JellyfinExpires()),
		ViewerAllowed:        profileActive && module.ViewerAllowed(request, profileID, now),
		ItemVisible:          itemFound && profileActive && module.CanView(profileID, item),
	}
	if !rejection.allowed() {
		if module.Rejected != nil {
			module.Rejected(request.Context(), rejection)
		}
		return library.Item{}, DeliveryViewer{}, false
	}
	return item, viewer, true
}

// Subtitle selects embedded or sidecar delivery and applies the play-session timeline.
func (module DeliveryHTTP) Subtitle(writer http.ResponseWriter, request *http.Request) {
	index, err := strconv.Atoi(request.PathValue("index"))
	if err != nil || index < 0 || index > 1024 {
		http.NotFound(writer, request)
		return
	}
	item, found := module.PlaybackItem(request)
	if !found {
		http.NotFound(writer, request)
		return
	}
	if module.serveEmbeddedSubtitle(writer, request, item, index) {
		return
	}
	if index >= len(item.Subtitles) {
		http.NotFound(writer, request)
		return
	}
	if session, valid := module.PlaySession(request, item.ID); valid && session.JellyfinPlan().MarkerMode == "server" {
		timeline := session.JellyfinPlan().Timeline
		module.WriteSubtitle(writer, request, item.Subtitles[index], &timeline)
		return
	}
	module.WriteSubtitle(writer, request, item.Subtitles[index], nil)
}

func (module DeliveryHTTP) serveEmbeddedSubtitle(writer http.ResponseWriter, request *http.Request, item library.Item, index int) bool { //nolint:cyclop,gocognit // Embedded and timeline subtitle paths share one ordered delivery boundary.
	for _, track := range module.Playback.Inspect(request.Context(), item).Facts.Subtitles {
		if track.SourceIndex != index || !track.Text {
			continue
		}
		session, mapped := module.PlaySession(request, item.ID)
		if !mapped || session.JellyfinPlan().MarkerMode != "server" {
			module.WriteEmbedded(writer, request, item, index)
			return true
		}
		path, data, err := module.ExtractEmbedded(request.Context(), item, index)
		if err == nil && path != "" {
			data, err = os.ReadFile(path)
		}
		if err != nil {
			if module.SubtitleUnavailable != nil {
				module.SubtitleUnavailable(request.Context(), item, index, err)
			}
			if module.Policy.EmbeddedFailureStatus == http.StatusServiceUnavailable {
				http.Error(writer, "embedded subtitles are unavailable", http.StatusServiceUnavailable)
			} else {
				http.NotFound(writer, request)
			}
			return true
		}
		writer.Header().Set("Content-Type", "text/vtt; charset=utf-8")
		_, _ = writer.Write(playback.MapWebVTT(data, session.JellyfinPlan().Timeline))
		return true
	}
	return false
}

// PlaybackItem returns only the item portion of one authorized context.
func (module DeliveryHTTP) PlaybackItem(request *http.Request) (library.Item, bool) {
	item, _, found := module.PlaybackContext(request)
	return item, found
}

func (module DeliveryHTTP) now() time.Time {
	if module.Now != nil {
		return module.Now()
	}
	return time.Now()
}
