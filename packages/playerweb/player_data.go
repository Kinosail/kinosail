package playerweb

import (
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/mediaprobe"
	"github.com/MikeO7/kinosail/packages/playback"
)

type PlayerData struct { //nolint:recvcheck // Templates need a value receiver for Resume while projection methods mutate during construction.
	library.Item
	ViewerProfile            string
	Start                    float64
	Source                   string
	ModeURL                  string
	ModeLabel                string
	PlaybackLabel            string
	PlaybackDescription      string
	PlaybackPolicy           string
	PlaybackOverride         bool
	CompatibilityLabel       string
	CompatibilityDescription string
	CompatibilityMode        string
	HLS                      bool
	Watched                  bool
	Listed                   bool
	Next                     string
	AutoSkip                 string
	CanDownload              bool
	CanTranscode             bool
	FileSize                 string
	DefaultSubtitles         bool
	Tracks                   []SubtitleTrack
	Playlists                []PlaylistOption
	MediaDetails             string
	Audio                    []mediaprobe.AudioTrack
	DirectSource             string
	AdaptiveSource           string
	FallbackSource           string
	DirectType               string
	DeferDirect              bool
	Duration                 float64
	Chapters                 []mediaprobe.Chapter
	Markers                  []mediaprobe.Marker
	AdminMarkers             []mediaprobe.Marker
	CanFetchSubtitles        bool
	SubtitleLanguage         string
	Owner                    bool
	CanRefreshMetadata       bool
	Collections              []PlaylistOption
	Room                     string
	RoomLeader               bool
	RoomItems                []library.Item
	Queue                    string
	Audiobook                bool
	ReplayGainTrack          string
	ReplayGainAlbum          string
	OfflineQuality           string
	PlaybackToken            string
	PlaybackSession          string
	Plan                     playback.PlaybackPlan
	HomeAssistant            bool
	MediaBitrate             int64
	MediaWidth               int
	MediaHeight              int
	MediaFrameRate           float64
}

func (value PlayerData) Resume() bool {
	return value.Start > 0 && (value.Duration == 0 || value.Start < value.Duration-10)
}

type PlaylistOption struct {
	Name     string
	Included bool
}

type SubtitleTrack struct {
	Label    string `json:"label"`
	Source   string `json:"source"`
	Default  bool   `json:"default"`
	Language string `json:"language,omitempty"`
	Role     string `json:"role,omitempty"`
	Kind     string `json:"kind,omitempty"`
	Forced   bool   `json:"forced,omitempty"`
	Embedded bool   `json:"embedded,omitempty"`
}
