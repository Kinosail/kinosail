package quickconnect

func NewApplication(broker *Broker, limiters Limiters, adapter Adapter) *Application {
	return NewApplicationFor(broker, limiters, adapter, PlayerPage())
}

func SubtitlesPage() Page {
	return Page{Product: "Kinosail Subtitles", ThemeScript: "/static/theme.js?v=cinema-1", Stylesheet: "/static/app.css?v=cinema-1", Icon: "/static/icon.svg?v=9"}
}
