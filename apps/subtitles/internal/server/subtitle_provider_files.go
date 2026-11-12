package server

import (
	"os"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/subtitlelanguage"
)

func subtitleSidecarPath(item library.Item, language string) string {
	return library.SubtitleSidecarPath(item.Path, language, "")
}

func validLanguage(language string) bool {
	_, ok := subtitlelanguage.NormalizeTag(language)
	return ok
}

func subtitleVariantPath(item library.Item, language, variant string) string {
	return library.SubtitleSidecarPath(item.Path, language, variant)
}

func (provider *subtitleProvider) managedSidecarPath(item library.Item, language string) string {
	if provider != nil && provider.ledger != nil {
		if record, found, err := provider.ledger.record(subtitleRecordKey(item.ID, language)); err == nil && found {
			return subtitleVariantPath(item, language, record.Variant)
		}
	}
	return subtitleSidecarPath(item, language)
}

func (provider *subtitleProvider) backupAvailable(item library.Item, language string) bool {
	info, err := os.Lstat(provider.managedSidecarPath(item, language) + ".kinosail.bak")
	return err == nil && info.Mode().IsRegular() && info.Size() > 0 && info.Size() <= 4<<20
}
