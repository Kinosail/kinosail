package maintenance

import (
	"errors"
	"log/slog"
	"time"

	"github.com/MikeO7/kinosail/packages/downloads"
	"github.com/MikeO7/kinosail/packages/mediaprobe"
)

const (
	DefaultSubtitleCacheLimit = int64(5) << 30
	DefaultDownloadCacheLimit = int64(10) << 30
)

type CacheTrim struct {
	CacheDir      string
	SubtitleLimit int64
	DownloadLimit int64
	Downloads     *downloads.Manager
	Now           func() time.Time
}

func (trim CacheTrim) Run() error {
	now := time.Now
	if trim.Now != nil {
		now = trim.Now
	}
	subtitleLimit := positiveOr(trim.SubtitleLimit, DefaultSubtitleCacheLimit)
	subtitles, subtitleErr := mediaprobe.TrimEmbeddedCache(trim.CacheDir, subtitleLimit, now())
	logTrim("subtitles", subtitles.Groups+subtitles.Workspaces, subtitles.FreedBytes, subtitles.Bytes, subtitleLimit, subtitleErr)
	if trim.Downloads == nil {
		return subtitleErr
	}
	downloadLimit := positiveOr(trim.DownloadLimit, DefaultDownloadCacheLimit)
	prepared, downloadErr := trim.Downloads.Trim(downloadLimit, now())
	logTrim("downloads", prepared.Removed, prepared.FreedBytes, prepared.Bytes, downloadLimit, downloadErr)
	return errors.Join(subtitleErr, downloadErr)
}

func positiveOr(value, fallback int64) int64 {
	if value > 0 {
		return value
	}
	return fallback
}

func logTrim(cache string, removed int, freed, size, limit int64, err error) {
	switch {
	case err != nil:
		slog.Warn("cache trim incomplete", "operation", "cache.trim", "cache", cache, "outcome", "partial", "removed", removed, "freed_bytes", freed, "size_bytes", size, "limit_bytes", limit, "failure_class", "filesystem")
	case removed > 0:
		slog.Info("cache trimmed", "operation", "cache.trim", "cache", cache, "outcome", "trimmed", "removed", removed, "freed_bytes", freed, "size_bytes", size, "limit_bytes", limit)
	default:
		slog.Debug("cache within limit", "operation", "cache.trim", "cache", cache, "size_bytes", size, "limit_bytes", limit)
	}
}
