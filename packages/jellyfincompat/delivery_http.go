package jellyfincompat

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/downloads"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

// DeliveryViewer contains the profile facts used by Jellyfin delivery.
type DeliveryViewer struct {
	ID                  string
	Revision            uint64
	Owner, CanTranscode bool
}

// DeliverySession exposes one app play session to the shared delivery policy.
type DeliverySession interface {
	playback.JellyfinSession
	JellyfinPlan() playback.PlaybackPlan
}

// DeliveryPolicy preserves true Player and Subtitles delivery differences.
type DeliveryPolicy struct {
	HLS                            playback.HLSRecipePolicy
	NestedStream, SourceHLS        bool
	SingleQuality, RequireHLSCache bool
	SessionTimelineOnly            bool
	EmbeddedFailureStatus          int
}

// PlaybackRejection records the complete public play-session decision.
type PlaybackRejection struct {
	ItemID                                               string
	ItemFound, SessionFound, SessionValid, ProfileActive bool
	ProfileRevisionMatch, SessionItemMatch, PublicMatch  bool
	Expired, ViewerAllowed, ItemVisible                  bool
}

func (value PlaybackRejection) allowed() bool {
	return value.ItemFound && value.SessionFound && value.SessionValid && value.ProfileActive &&
		value.ProfileRevisionMatch && value.SessionItemMatch && value.PublicMatch && !value.Expired &&
		value.ViewerAllowed && value.ItemVisible
}

// DeliveryHTTP contains true app adapters for media I/O and profile policy.
type DeliveryHTTP struct {
	Playback            PlaybackHTTP
	Policy              DeliveryPolicy
	CurrentViewer       func(*http.Request) DeliveryViewer
	FindItem            func(string) (library.Item, bool)
	FindViewer          func(string) (DeliveryViewer, bool)
	ViewerAllowed       func(*http.Request, string, time.Time) bool
	CanView             func(string, library.Item) bool
	Public              func(*http.Request) bool
	LoadSession         func(string) (any, bool)
	ActiveRevision      func(string, uint64) bool
	Now                 func() time.Time
	Rejected            func(context.Context, PlaybackRejection)
	ServeHLS            func(http.ResponseWriter, *http.Request, library.Item, playback.HLSRecipe, string)
	HLSConfigured       func() bool
	CanDownload         func(*http.Request) bool
	DownloadsConfigured func() bool
	StartDownload       func(string, library.Item) (downloads.Job, error)
	WaitDownload        func(context.Context, string, string) (downloads.Job, error)
	WriteEmbedded       func(http.ResponseWriter, *http.Request, library.Item, int)
	ExtractEmbedded     func(context.Context, library.Item, int) (string, []byte, error)
	WriteSubtitle       func(http.ResponseWriter, *http.Request, string, *playback.Timeline)
	SubtitleUnavailable func(context.Context, library.Item, int, error)
}

// Stream selects explicit HLS, download, planned HLS, automatic skip, or direct delivery.
func (module DeliveryHTTP) Stream(writer http.ResponseWriter, request *http.Request) { //nolint:cyclop // The order is the Player delivery contract.
	stream := strings.ToLower(request.PathValue("stream"))
	if !validOptionalValue(stream, 2048) {
		http.NotFound(writer, request)
		return
	}
	item, viewer, found := module.PlaybackContext(request)
	if !found || item.Kind == "photo" {
		http.NotFound(writer, request)
		return
	}
	if module.serveExplicitHLS(writer, request, item, viewer, stream) {
		return
	}
	if module.Policy.NestedStream {
		if slash := strings.LastIndexByte(stream, '/'); slash >= 0 && strings.HasPrefix(stream[slash+1:], "stream") {
			stream = stream[slash+1:]
		}
	}
	if stream != "" && !strings.HasPrefix(stream, "stream") {
		http.NotFound(writer, request)
		return
	}
	if module.serveDownload(writer, request, item, viewer) || module.serveSessionHLS(writer, request, item) || module.serveAutomaticSkip(writer, request, item, viewer) {
		return
	}
	http.ServeFile(writer, request, item.Path)
}

func (module DeliveryHTTP) serveExplicitHLS(writer http.ResponseWriter, request *http.Request, item library.Item, viewer DeliveryViewer, stream string) bool { //nolint:cyclop,gocognit // Ordered route forms have distinct wire responses.
	if stream == "master.m3u8" {
		if !viewer.Owner && !viewer.CanTranscode {
			http.Error(writer, "transcoding is not allowed", http.StatusForbidden)
			return true
		}
		value, valid := StrictQuery(request.URL.Query(), "playbackPlan", 2048)
		recipe, err := playback.ParseHLSRecipe(value, module.Policy.HLS)
		if !valid || err != nil {
			http.NotFound(writer, request)
			return true
		}
		module.ServeHLS(writer, request, item, module.hlsRecipe(request, recipe), "index.m3u8")
		return true
	}
	recipe, file, planned := playback.PlannedHLSFile(stream, module.Policy.HLS)
	if planned && playback.HLSFile(file) {
		if !viewer.Owner && !viewer.CanTranscode {
			http.Error(writer, "transcoding is not allowed", http.StatusForbidden)
			return true
		}
		module.ServeHLS(writer, request, item, module.hlsRecipe(request, recipe), file)
		return true
	}
	if module.Policy.SourceHLS {
		if playback.HLSFile(stream) {
			return module.serveSessionHLSFile(writer, request, item, stream)
		}
		if file, valid := SourceHLSFile(stream); valid {
			return module.serveSessionHLSFile(writer, request, item, file)
		}
	}
	return false
}

func (module DeliveryHTTP) serveSessionHLSFile(writer http.ResponseWriter, request *http.Request, item library.Item, file string) bool {
	if session, valid := module.PlaySession(request, item.ID); valid && session.JellyfinPlan().Mode != "direct" {
		module.ServeHLS(writer, request, item, module.hlsRecipe(request, playback.RecipeFor(session.JellyfinPlan())), file)
		return true
	}
	http.NotFound(writer, request)
	return true
}

func (module DeliveryHTTP) serveDownload(writer http.ResponseWriter, request *http.Request, item library.Item, viewer DeliveryViewer) bool {
	if !strings.HasSuffix(request.URL.Path, "/Download") {
		return false
	}
	if !module.CanDownload(request) {
		http.Error(writer, "downloads are not enabled for this Viewer Profile", http.StatusForbidden)
		return true
	}
	if !module.DownloadsConfigured() {
		return false
	}
	job, err := module.StartDownload(viewer.ID, item)
	if err == nil {
		job, err = module.WaitDownload(request.Context(), viewer.ID, job.ID)
	}
	if err != nil {
		http.Error(writer, "download is unavailable", http.StatusServiceUnavailable)
		return true
	}
	if downloads.Serve(writer, request, job) != nil {
		http.NotFound(writer, request)
	}
	return true
}

func (module DeliveryHTTP) serveSessionHLS(writer http.ResponseWriter, request *http.Request, item library.Item) bool {
	session, valid := module.PlaySession(request, item.ID)
	if !valid {
		return false
	}
	plan := session.JellyfinPlan()
	selected := plan.Mode != "direct"
	if module.Policy.SessionTimelineOnly {
		selected = plan.MarkerMode == "server"
	}
	if module.Policy.RequireHLSCache && !module.HLSConfigured() {
		selected = false
	}
	if selected {
		module.ServeHLS(writer, request, item, module.hlsRecipe(request, playback.RecipeFor(plan)), "index.m3u8")
	}
	return selected
}

func (module DeliveryHTTP) serveAutomaticSkip(writer http.ResponseWriter, request *http.Request, item library.Item, viewer DeliveryViewer) bool {
	if item.Kind != "video" || !viewer.Owner && !viewer.CanTranscode {
		return false
	}
	media := module.Playback.Inspect(request.Context(), item)
	capabilities := playback.JellyfinCapabilities(playback.JellyfinDeviceProfile{}, media.Facts)
	plan := module.Playback.Decide(media.Facts, capabilities, module.Playback.ViewerPolicy(request), playback.NetworkIntent{}, media.Markers, module.Playback.AutomaticSkip())
	if plan.MarkerMode != "server" {
		return false
	}
	module.ServeHLS(writer, request, item, module.hlsRecipe(request, playback.RecipeFor(plan)), "index.m3u8")
	return true
}

func (module DeliveryHTTP) hlsRecipe(request *http.Request, recipe playback.HLSRecipe) playback.HLSRecipe {
	return HLSRecipe(request, recipe, module.Policy.SingleQuality, module.Public)
}

// HLSRecipe applies Player's local single-quality rule.
func HLSRecipe(request *http.Request, recipe playback.HLSRecipe, singleQuality bool, public func(*http.Request) bool) playback.HLSRecipe {
	if singleQuality {
		recipe.SingleQuality = recipe.Mode == "transcode" && !public(request)
	}
	return recipe
}

// SourceHLSFile recognizes Player's official-client source-prefixed HLS paths.
func SourceHLSFile(path string) (string, bool) {
	parts := strings.Split(strings.ToLower(path), "/")
	if len(parts) < 2 {
		return "", false
	}
	child := strings.Join(parts[len(parts)-2:], "/")
	if len(parts) >= 3 && parts[len(parts)-3] == "main" && playback.HLSFile(child) {
		return child, true
	}
	last := parts[len(parts)-1]
	if parts[len(parts)-2] == "main" && (last == "index.m3u8" || last == "master.m3u8") {
		return "index.m3u8", true
	}
	return "", false
}
