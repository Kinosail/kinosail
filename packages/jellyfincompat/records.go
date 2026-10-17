package jellyfincompat

type ImageTags struct {
	Primary string `json:"Primary"`
}

type UserData struct {
	IsFavorite            bool   `json:"IsFavorite"`
	ItemID                string `json:"ItemId"`
	Key                   string `json:"Key"`
	PlaybackPositionTicks int64  `json:"PlaybackPositionTicks"`
	Played                bool   `json:"Played"`
}

type FolderUserData struct {
	ItemID string `json:"ItemId"`
	Key    string `json:"Key"`
}

type ItemRecord struct {
	CanDelete      bool              `json:"CanDelete"`
	CanDownload    bool              `json:"CanDownload"`
	Container      string            `json:"Container"`
	DateCreated    string            `json:"DateCreated"`
	Genres         []string          `json:"Genres,omitempty"`
	HasSubtitles   bool              `json:"HasSubtitles"`
	ID             string            `json:"Id"`
	ImageTags      *ImageTags        `json:"ImageTags,omitempty"`
	IsFolder       bool              `json:"IsFolder"`
	LocationType   string            `json:"LocationType"`
	MediaSources   []map[string]any  `json:"MediaSources,omitempty"`
	MediaType      string            `json:"MediaType"`
	Name           string            `json:"Name"`
	OfficialRating string            `json:"OfficialRating"`
	Overview       string            `json:"Overview"`
	ParentID       string            `json:"ParentId"`
	ProductionYear *int              `json:"ProductionYear,omitempty"`
	ProviderIDs    map[string]string `json:"ProviderIds"`
	SortName       string            `json:"SortName"`
	Type           string            `json:"Type"`
	UserData       UserData          `json:"UserData"`
	*EpisodeRecord
}

type EpisodeRecord struct {
	IndexNumber       int    `json:"IndexNumber"`
	ParentIndexNumber int    `json:"ParentIndexNumber"`
	SeasonID          string `json:"SeasonId"`
	SeriesID          string `json:"SeriesId"`
	SeriesName        string `json:"SeriesName"`
}

type SeriesRecord struct {
	ChildCount   int               `json:"ChildCount"`
	ID           string            `json:"Id"`
	ImageTags    *ImageTags        `json:"ImageTags,omitempty"`
	IsFolder     bool              `json:"IsFolder"`
	LocationType string            `json:"LocationType"`
	MediaType    string            `json:"MediaType"`
	Name         string            `json:"Name"`
	ParentID     string            `json:"ParentId"`
	ProviderIDs  map[string]string `json:"ProviderIds"`
	SortName     string            `json:"SortName"`
	Type         string            `json:"Type"`
	UserData     FolderUserData    `json:"UserData"`
}

type SeasonRecord struct {
	ChildCount  int            `json:"ChildCount"`
	ID          string         `json:"Id"`
	ImageTags   *ImageTags     `json:"ImageTags,omitempty"`
	IndexNumber int            `json:"IndexNumber"`
	IsFolder    bool           `json:"IsFolder"`
	MediaType   string         `json:"MediaType"`
	Name        string         `json:"Name"`
	ParentID    string         `json:"ParentId"`
	SeriesID    string         `json:"SeriesId"`
	SeriesName  string         `json:"SeriesName"`
	Type        string         `json:"Type"`
	UserData    FolderUserData `json:"UserData"`
}

type FolderRecord struct {
	CollectionType string `json:"CollectionType"`
	ID             string `json:"Id"`
	IsFolder       bool   `json:"IsFolder"`
	Name           string `json:"Name"`
	SortName       string `json:"SortName"`
	Type           string `json:"Type"`
}
