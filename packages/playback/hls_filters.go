package playback

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

func ApplyBurnIn(video []string, itemPath string, recipe HLSRecipe, policy HLSRecipePolicy) ([]string, []string) {
	if recipe.Burn == "" && len(recipe.Omitted) == 0 {
		return []string{"-map", "0:v:0"}, video
	}
	filter, remaining := splitVideoFilter(video)
	timeline := automaticSkipVideoFilter(recipe.Omitted, policy.StartPresentationAtZero)
	if recipe.Burn == "" {
		return []string{"-map", "0:v:0", "-vf", joinFilters(timeline, filter)}, remaining
	}
	// Color conversion and deinterlacing run before drawing SDR subtitles;
	// scaling and hardware upload stay after all software-only operations.
	beforeBurn := ""
	if index := strings.Index(filter, "scale=w="); index > 0 {
		beforeBurn, filter = strings.TrimSuffix(filter[:index], ","), filter[index:]
	}
	if recipe.Burn == "image" {
		base, prefix := "[0:v:0]", ""
		if beforeBurn != "" {
			prefix, base = "[0:v:0]"+beforeBurn+"[prepared];", "[prepared]"
		}
		return []string{"-filter_complex", prefix + fmt.Sprintf("%s[0:%d]overlay[base];[base]%s[v]", base, recipe.Subtitle, joinFilters(timeline, filter)), "-map", "[v]"}, remaining
	}
	if recipe.SubtitlePath != "" {
		itemPath = recipe.SubtitlePath
	}
	path := subtitleFilterPath(filepath.ToSlash(itemPath))
	before, after := "", ""
	if recipe.SubtitleTime > 0 {
		before = "setpts=PTS+" + FFmpegSeconds(recipe.SubtitleTime) + "/TB"
		after = "setpts=PTS-" + FFmpegSeconds(recipe.SubtitleTime) + "/TB"
	}
	return []string{"-map", "0:v:0", "-vf", joinFilters(beforeBurn, before, "subtitles=filename="+path+":si="+strconv.Itoa(recipe.SubtitleOrdinal), after, timeline, filter)}, remaining
}

func AutomaticSkipAudioArguments(recipe HLSRecipe, policy HLSRecipePolicy) []string {
	filter := automaticSkipAudioFilter(recipe, policy)
	if filter == "" {
		return nil
	}
	return []string{"-af", filter}
}

// AudioFilterArguments keeps timeline and opt-in listening effects in one
// filter graph so later arguments cannot silently replace an earlier effect.
func AudioFilterArguments(recipe HLSRecipe, policy HLSRecipePolicy) []string {
	filters := []string{automaticSkipAudioFilter(recipe, policy)}
	if recipe.DialogueBoost {
		filters = append(filters, "equalizer=f=1600:t=q:w=0.9:g=4", "equalizer=f=3200:t=q:w=1.1:g=3")
	}
	if recipe.NormalizeLoudness {
		filters = append(filters, "dynaudnorm=f=150:g=15:p=0.9:m=10:r=0.25")
	} else if recipe.DialogueBoost {
		filters = append(filters, "alimiter=limit=0.95:level=false")
	}
	if filter := joinFilters(filters...); filter != "" {
		return []string{"-af", filter}
	}
	return nil
}

func automaticSkipAudioFilter(recipe HLSRecipe, policy HLSRecipePolicy) string {
	if len(recipe.Omitted) == 0 {
		return ""
	}
	return "aselect=" + keepExpression(recipe.Omitted) + ",asetpts=" + shiftedPTS(recipe.Omitted, policy.StartPresentationAtZero)
}

func automaticSkipVideoFilter(ranges []Range, startAtZero bool) string {
	if len(ranges) == 0 {
		return ""
	}
	return "select=" + keepExpression(ranges) + ",setpts=" + shiftedPTS(ranges, startAtZero)
}

func keepExpression(ranges []Range) string {
	removed := make([]string, 0, len(ranges))
	for _, value := range ranges {
		removed = append(removed, "gte(t\\,"+FFmpegSeconds(value.Start)+")*lt(t\\,"+FFmpegSeconds(value.End)+")")
	}
	return "not(" + strings.Join(removed, "+") + ")"
}

func shiftedPTS(ranges []Range, startAtZero bool) string {
	removed := make([]string, 0, len(ranges))
	for _, value := range ranges {
		removed = append(removed, "gte(PTS*TB\\,"+FFmpegSeconds(value.End)+")*"+FFmpegSeconds(value.End-value.Start))
	}
	base := "PTS-"
	if startAtZero {
		base = "PTS-STARTPTS-"
	}
	return base + "(" + strings.Join(removed, "+") + ")/TB"
}

func FFmpegSeconds(value float64) string { return strconv.FormatFloat(value, 'f', 3, 64) }

func joinFilters(filters ...string) string {
	result := make([]string, 0, len(filters))
	for _, filter := range filters {
		if filter != "" {
			result = append(result, filter)
		}
	}
	return strings.Join(result, ",")
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

// The option parser and filtergraph parser each consume one escaping layer.
func subtitleFilterPath(path string) string {
	option := strings.NewReplacer(`\`, `\\`, `:`, `\:`, `'`, `\'`).Replace(path)
	return strings.NewReplacer(`\`, `\\`, `'`, `\'`, `,`, `\,`, `;`, `\;`, `[`, `\[`, `]`, `\]`).Replace(option)
}

func splitVideoFilter(video []string) (string, []string) {
	filter, remaining := "", make([]string, 0, len(video))
	for index := 0; index < len(video); index++ {
		if video[index] == "-vf" && index+1 < len(video) {
			filter, index = video[index+1], index+1
			continue
		}
		remaining = append(remaining, video[index])
	}
	return filter, remaining
}
