package playback

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

type HLSRequest struct {
	Recipe  HLSRecipe
	File    string
	Track   int
	Planned bool
}

type HLSHandlerDependencies struct {
	Policy      HLSRecipePolicy
	Lookup      func(*http.Request, string) (library.Item, bool)
	Allowed     func(*http.Request) bool
	Legacy      func(*http.Request, library.Item, int) HLSRecipe
	Duration    func(*http.Request, library.Item) float64
	Serve       func(http.ResponseWriter, *http.Request, library.Item, HLSRecipe, string)
	NotFound    func(http.ResponseWriter, *http.Request)
	Forbidden   func(http.ResponseWriter, *http.Request)
	InvalidSeek func(http.ResponseWriter, *http.Request)
}

// ServeHLS validates and authorizes an HLS route before invoking app-owned work.
func ServeHLS(writer http.ResponseWriter, request *http.Request, dependencies HLSHandlerDependencies) { //nolint:cyclop // Route validation and delegation form one side-effect boundary.
	if request == nil || dependencies.Lookup == nil || dependencies.Allowed == nil || dependencies.Legacy == nil || dependencies.Duration == nil || dependencies.Serve == nil || dependencies.NotFound == nil || dependencies.Forbidden == nil || dependencies.InvalidSeek == nil {
		http.Error(writer, "compatible playback is unavailable", http.StatusInternalServerError)
		return
	}
	item, found := dependencies.Lookup(request, request.PathValue("id"))
	selection, err := ParseHLSRequest(request.PathValue("file"), request.PathValue("track"), dependencies.Policy)
	if !found || err != nil {
		dependencies.NotFound(writer, request)
		return
	}
	if !dependencies.Allowed(request) {
		dependencies.Forbidden(writer, request)
		return
	}
	recipe := selection.Recipe
	if !selection.Planned {
		recipe = dependencies.Legacy(request, item, selection.Track)
		recipe.Codec = ""
	} else if recipe.Offset > 0 && !ValidHLSOffset(recipe.Offset, dependencies.Duration(request, item)) {
		dependencies.InvalidSeek(writer, request)
		return
	}
	dependencies.Serve(writer, request, item, recipe, selection.File)
}

// ParseHLSRequest validates the complete route input before a caller starts work.
func ParseHLSRequest(name, track string, policy HLSRecipePolicy) (HLSRequest, error) {
	recipe, file, planned := PlannedHLSFile(name, policy)
	if len(name) >= 2 && name[:2] == "p/" && !planned {
		return HLSRequest{}, errors.New("HLS request is invalid")
	}
	if planned {
		name = file
	}
	audio, err := AudioTrackIndex(track)
	if err != nil || !HLSFile(name) {
		return HLSRequest{}, errors.New("HLS request is invalid")
	}
	return HLSRequest{Recipe: recipe, File: name, Track: audio, Planned: planned}, nil
}

type HLSCodecInput struct {
	AudioOnly                                 bool
	Arguments, Video, Compatibility           []string
	ItemPath, VideoRate, AudioRate, CopyInput string
	Recipe                                    HLSRecipe
	Policy                                    HLSRecipePolicy
}

// HLSCodecArguments applies Player's mode-specific stream mapping and codecs.
func HLSCodecArguments(input HLSCodecInput) ([]string, error) {
	if input.AudioRate == "" || input.CopyInput != "0" && input.CopyInput != "1" {
		return nil, errors.New("HLS codec input is invalid")
	}
	arguments := append([]string(nil), input.Arguments...)
	if input.AudioOnly {
		return audioCodecArguments(input, arguments)
	}
	switch input.Recipe.Mode {
	case "remux":
		arguments = append(arguments, "-map", input.CopyInput+":v:0", "-map", input.CopyInput+":a:"+strconv.Itoa(input.Recipe.Audio)+"?", "-sn", "-c:v", "copy", "-c:a", "copy")
	case "audio-transcode":
		arguments = append(arguments, "-map", input.CopyInput+":v:0", "-map", "0:a:"+strconv.Itoa(input.Recipe.Audio)+"?", "-sn")
		arguments = append(arguments, AutomaticSkipAudioArguments(input.Recipe, input.Policy)...)
		arguments = append(arguments, "-c:v", "copy", "-c:a", "aac", "-ac", "2", "-b:a", input.AudioRate)
	case "transcode":
		mapping, encoded := ApplyBurnIn(input.Video, input.ItemPath, input.Recipe, input.Policy)
		arguments = append(arguments, mapping...)
		arguments = append(arguments, "-map", "0:a:"+strconv.Itoa(input.Recipe.Audio)+"?", "-sn")
		arguments = append(arguments, AutomaticSkipAudioArguments(input.Recipe, input.Policy)...)
		arguments = append(arguments, encoded...)
		videoRate := CappedVideoRate(input.VideoRate, input.AudioRate, input.Recipe.MaxBitrate)
		if videoRate == "" {
			return nil, errors.New("HLS video rate is missing")
		}
		arguments = append(arguments, "-maxrate", videoRate, "-bufsize", videoRate, "-c:a", "aac", "-ac", "2", "-b:a", input.AudioRate)
		arguments = append(arguments, input.Compatibility...)
		arguments = append(arguments, "-force_key_frames", "expr:gte(t,n_forced*4)")
	default:
		return nil, errors.New("HLS playback mode is invalid")
	}
	return arguments, nil
}

// StoreJellyfinPlaySession prunes expired entries and atomically publishes one session.
type JellyfinSession interface {
	JellyfinItemID() string
	JellyfinProfile() (string, uint64)
	JellyfinExpires() time.Time
	JellyfinPublic() bool
}

type JellyfinSessionAuthorization struct {
	Found, Valid, RevisionValid, ItemMatch, ViewerMatch, PublicMatch, NotExpired bool
}

func (authorization JellyfinSessionAuthorization) Allowed() bool {
	return authorization.Found && authorization.Valid && authorization.RevisionValid && authorization.ItemMatch && authorization.ViewerMatch && authorization.PublicMatch && authorization.NotExpired
}

func AuthorizeJellyfinPlaySession(value any, found bool, itemID, viewerID string, public bool, now time.Time, activeRevision func(string, uint64) bool) bool {
	session, valid := value.(JellyfinSession)
	if !found || !valid || activeRevision == nil || now.IsZero() {
		return false
	}
	profileID, revision := session.JellyfinProfile()
	revisionValid := !session.JellyfinPublic() || activeRevision(profileID, revision)
	return (JellyfinSessionAuthorization{Found: found, Valid: true, RevisionValid: revisionValid, ItemMatch: session.JellyfinItemID() == itemID, ViewerMatch: viewerID == "" || profileID == viewerID, PublicMatch: !public || session.JellyfinPublic(), NotExpired: now.Before(session.JellyfinExpires())}).Allowed()
}

func audioCodecArguments(input HLSCodecInput, arguments []string) ([]string, error) {
	if !audioOnlyRecipe(input.Recipe) || input.CopyInput != "0" {
		return nil, errors.New("HLS audio recipe is invalid")
	}
	return append(arguments, "-map", "0:a:"+strconv.Itoa(input.Recipe.Audio), "-vn", "-sn", "-dn", "-c:a", "aac", "-ac", "2", "-b:a", input.AudioRate), nil
}
