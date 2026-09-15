package main

import (
	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func configuredSubtitleProviders(configured configuration.Snapshot) server.SubtitleConfig {
	return server.SubtitleConfig{
		URL:    configured.String("integrations.subdl.url"),
		APIKey: configured.String("integrations.subdl.api_key"),
		SubSource: server.SubSourceConfig{
			URL:         configured.String("integrations.subsource.url"),
			APIKey:      configured.String("integrations.subsource.api_key"),
			PersonalUse: configured.Bool("integrations.subsource.personal_use"),
		},
		OpenSubtitles: server.OpenSubtitlesConfig{
			URL:      configured.String("integrations.opensubtitles.url"),
			APIKey:   configured.String("integrations.opensubtitles.api_key"),
			Username: configured.String("integrations.opensubtitles.username"),
			Password: configured.String("integrations.opensubtitles.password"),
		},
	}
}
