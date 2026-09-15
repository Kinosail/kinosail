package server

import "github.com/MikeO7/kinosail-subtitles/internal/subtitlelanguage"

func (provider *subtitleProvider) supports(language string) bool {
	if provider.cache == "" {
		return false
	}
	canonical, ok := subtitlelanguage.NormalizeTag(language)
	if !ok {
		return false
	}
	_, subDL := subtitlelanguage.Code(canonical, subtitlelanguage.SubDL)
	_, open := subtitlelanguage.Code(canonical, subtitlelanguage.OpenSubtitles)
	_, subSource := subtitlelanguage.Code(canonical, subtitlelanguage.SubSource)
	return subDL && provider.subDLConfigured() || open && provider.open.configured() || subSource && provider.subsource.configured()
}

func subtitleProviderAvailable(language string) bool {
	canonical, ok := subtitlelanguage.NormalizeTag(language)
	if !ok {
		return false
	}
	for _, provider := range []subtitlelanguage.Provider{subtitlelanguage.SubDL, subtitlelanguage.OpenSubtitles, subtitlelanguage.SubSource} {
		if _, supported := subtitlelanguage.Code(canonical, provider); supported {
			return true
		}
	}
	return false
}
