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

func ServeHLS(writer http.ResponseWriter, request *http.Request, dependencies HLSHandlerDependencies) {
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
	DolbyVisionCodec                          string
	RemuxAudio                                []string
	Recipe                                    HLSRecipe
	Policy                                    HLSRecipePolicy
}

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
		return remuxCodecArguments(input, arguments)
	case "audio-transcode":
		arguments = append(arguments, "-map", input.CopyInput+":v:0", "-map", "0:a:"+strconv.Itoa(input.Recipe.Audio)+"?", "-sn")
		arguments = append(arguments, AudioFilterArguments(input.Recipe, input.Policy)...)
		arguments = append(append(arguments, copiedVideoArguments(input)...), "-c:a", "aac", "-ac", "2", "-b:a", input.AudioRate)
	case "transcode":
		return transcodeCodecArguments(input, arguments)
	default:
		return nil, errors.New("HLS playback mode is invalid")
	}
	return arguments, nil
}

func remuxCodecArguments(input HLSCodecInput, arguments []string) ([]string, error) {
	if input.Recipe.DialogueBoost || input.Recipe.NormalizeLoudness {
		return nil, errors.New("audio effects require audio conversion")
	}
	audio := input.RemuxAudio
	if len(audio) == 0 {
		audio = []string{"-c:a", "copy"}
	}
	arguments = append(arguments, "-map", input.CopyInput+":v:0", "-map", input.CopyInput+":a:"+strconv.Itoa(input.Recipe.Audio)+"?", "-sn")
	return append(append(arguments, copiedVideoArguments(input)...), audio...), nil
}

func copiedVideoArguments(input HLSCodecInput) []string {
	tag, found := map[string]string{"hevc": "hvc1", "av1": "av01"}[input.DolbyVisionCodec]
	if !found {
		return []string{"-c:v", "copy"}
	}
	return []string{"-c:v", "copy", "-bsf:v", "dovi_rpu=strip=1", "-tag:v", tag}
}

func DolbyVisionCopyCodec(video VideoFacts) string {
	if DolbyVisionBaseLayer(video) == "" {
		return ""
	}
	return Lower(video.Codec)
}

func transcodeCodecArguments(input HLSCodecInput, arguments []string) ([]string, error) {
	mapping, encoded := ApplyBurnIn(input.Video, input.ItemPath, input.Recipe, input.Policy)
	arguments = append(arguments, mapping...)
	arguments = append(arguments, "-map", "0:a:"+strconv.Itoa(input.Recipe.Audio)+"?", "-sn")
	arguments = append(arguments, AudioFilterArguments(input.Recipe, input.Policy)...)
	arguments = append(arguments, encoded...)
	videoRate := CappedVideoRate(input.VideoRate, input.AudioRate, input.Recipe.MaxBitrate)
	if videoRate == "" {
		return nil, errors.New("HLS video rate is missing")
	}
	arguments = append(arguments, "-maxrate", videoRate, "-bufsize", videoRate, "-c:a", "aac", "-ac", "2", "-b:a", input.AudioRate)
	arguments = append(arguments, input.Compatibility...)
	return append(arguments, "-force_key_frames", "expr:gte(t,n_forced*4)"), nil
}

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
	arguments = append(arguments, "-map", "0:a:"+strconv.Itoa(input.Recipe.Audio), "-vn", "-sn", "-dn")
	arguments = append(arguments, AudioFilterArguments(input.Recipe, input.Policy)...)
	return append(arguments, "-c:a", "aac", "-ac", "2", "-b:a", input.AudioRate), nil
}
