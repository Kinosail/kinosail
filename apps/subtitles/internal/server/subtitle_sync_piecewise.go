package server

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func parseSubtitleTiming(line string) (time.Duration, time.Duration, bool) {
	parts := strings.SplitN(line, "-->", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	right := strings.Fields(strings.TrimSpace(parts[1]))
	if len(right) == 0 {
		return 0, 0, false
	}
	start, startOK := parseSubtitleTime(strings.TrimSpace(parts[0]))
	end, endOK := parseSubtitleTime(right[0])
	return start, end, startOK && endOK && end > start
}

func parseSubtitleTime(value string) (time.Duration, bool) {
	match := subtitleTime.FindStringSubmatch(value)
	if match == nil {
		return 0, false
	}
	hours, _ := strconv.Atoi(match[1])
	minutes, _ := strconv.Atoi(match[2])
	seconds, _ := strconv.Atoi(match[3])
	millis, _ := strconv.Atoi(match[4])
	if minutes > 59 || seconds > 59 {
		return 0, false
	}
	return time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second + time.Duration(millis)*time.Millisecond, true
}

func (provider *subtitleProvider) downloadSubtitleCandidates(ctx context.Context, item library.Item, candidates []subtitleDownloadCandidate, accept func(subtitleDownloadCandidate) bool) (cleanedSubtitle, subtitleRecord, error) {
	attempt := subtitleCandidateAttempt{provider: provider, ctx: ctx, item: item}
	for _, candidate := range candidates {
		if cleaned, record, ok := attempt.download(candidate, accept); ok {
			return cleaned, record, nil
		}
	}
	return cleanedSubtitle{}, subtitleRecord{}, errNoTrustedSubtitle
}

type subtitleCandidateAttempt struct {
	provider *subtitleProvider
	ctx      context.Context
	item     library.Item
	audio    []float64
	audioErr error
}

func (attempt *subtitleCandidateAttempt) download(candidate subtitleDownloadCandidate, accept func(subtitleDownloadCandidate) bool) (cleanedSubtitle, subtitleRecord, bool) {
	if !attempt.acceptCandidate(candidate, accept) {
		return cleanedSubtitle{}, subtitleRecord{}, false
	}
	cleaned, ok := attempt.prepareCandidate(candidate)
	if !ok {
		return cleanedSubtitle{}, subtitleRecord{}, false
	}
	if !candidate.ExactHash {
		synchronized, syncErr := attempt.synchronizeCandidate(cleaned, candidate)
		if syncErr == nil {
			cleaned = synchronized
		} else if errors.Is(syncErr, errSubtitleTimingMismatch) || candidate.ReleaseMatch < 0.8 {
			return cleanedSubtitle{}, subtitleRecord{}, false
		} else {
			cleaned.TimingEvidence = "unverified"
		}
	} else {
		cleaned.TimingEvidence = "file-hash"
	}
	return acceptedSubtitleCandidate(cleaned, candidate)
}

func (attempt *subtitleCandidateAttempt) acceptCandidate(candidate subtitleDownloadCandidate, accept func(subtitleDownloadCandidate) bool) bool {
	return (accept == nil || accept(candidate)) && (candidate.Source != "subdl" || attempt.provider.ledger.takeSubDLDownload(time.Now()))
}

func (attempt *subtitleCandidateAttempt) prepareCandidate(candidate subtitleDownloadCandidate) (cleanedSubtitle, bool) {
	data, err := candidate.Download(attempt.ctx)
	if err != nil {
		return cleanedSubtitle{}, false
	}
	cleaned, err := convertSubtitle(data, firstNonempty(candidate.Language, "en"), subtitleConversionOptions{})
	if err != nil {
		return cleanedSubtitle{}, false
	}
	if candidate.Preserve {
		cleaned.Data = data
		cleaned.Cleanup = []string{"Preserved provider file unchanged"}
		cleaned.Synchronization = "none"
	}
	return cleaned, true
}

func (attempt *subtitleCandidateAttempt) synchronizeCandidate(cleaned cleanedSubtitle, candidate subtitleDownloadCandidate) (cleanedSubtitle, error) {
	if attempt.audio == nil && attempt.audioErr == nil {
		attempt.audio, attempt.audioErr = attempt.provider.sync.speechForLanguage(attempt.ctx, attempt.item, candidate.Language)
	}
	if attempt.audioErr != nil {
		return cleanedSubtitle{}, attempt.audioErr
	}
	synchronized, changed, err := synchronizeSubtitle(attempt.audio, cleaned)
	if err != nil {
		return cleanedSubtitle{}, err
	}
	if candidate.Preserve {
		if changed {
			return cleanedSubtitle{}, errSubtitleTimingMismatch
		}
		cleaned.TimingEvidence = "audio"
		return cleaned, nil
	}
	return synchronized, nil
}

func acceptedSubtitleCandidate(cleaned cleanedSubtitle, candidate subtitleDownloadCandidate) (cleanedSubtitle, subtitleRecord, bool) {
	now := time.Now().Unix()
	if cleaned.Synchronization == "" {
		cleaned.Synchronization = "none"
	}
	role := "translation"
	if candidate.HI {
		role = "captions"
	}
	return cleaned, subtitleRecord{Role: role, Source: candidate.Source, Score: candidate.Score, ReleaseMatch: candidate.ReleaseMatch, CheckedAt: now, InstalledAt: now, Cleanup: cleaned.Cleanup, Synchronization: cleaned.Synchronization, TimingEvidence: cleaned.TimingEvidence, Managed: true}, true
}

func findPiecewiseSubtitleAlignment(audio []float64, cues []subtitleCue, global subtitleAlignment) ([]subtitleAlignmentAnchor, bool) {
	if len(cues) < 30 {
		return nil, false
	}
	maximum := time.Duration(0)
	for _, cue := range cues {
		maximum = max(maximum, cue.End)
	}
	if maximum < 15*time.Minute {
		return nil, false
	}
	coarseAudio := averageSubtitleFrames(audio, subtitleCoarseStep)
	frame := subtitleFrame * subtitleCoarseStep
	center := int(global.Offset / frame)
	radius := int(30 * time.Second / frame)
	anchors := make([]subtitleAlignmentAnchor, 0, 3)
	totalImprovement := 0.0
	for region := 0; region < 3; region++ {
		start, end := maximum*time.Duration(region*3+1)/9, maximum*time.Duration(region*3+2)/9
		anchor, improvement, ok := piecewiseSubtitleRegion(coarseAudio, cues, global, frame, center, radius, start, end)
		if !ok {
			return nil, false
		}
		totalImprovement += improvement
		anchors = append(anchors, anchor)
	}
	if totalImprovement < 0.045 {
		return nil, false
	}
	return validPiecewiseSubtitleAnchors(anchors, global.Offset)
}

func piecewiseSubtitleRegion(audio []float64, cues []subtitleCue, global subtitleAlignment, frame time.Duration, center, radius int, start, end time.Duration) (subtitleAlignmentAnchor, float64, bool) {
	regional := make([]subtitleCue, 0, len(cues)/3+1)
	for _, cue := range cues {
		if cue.Start >= start && cue.Start < end {
			regional = append(regional, cue)
		}
	}
	if len(regional) < 8 {
		return subtitleAlignmentAnchor{}, 0, false
	}
	activity := subtitleActivity(regional, global.Scale, frame, len(audio))
	// Compare only this region: dialogue elsewhere is not negative evidence.
	first := max(0, int(time.Duration(float64(start)*global.Scale)/frame)-radius-subtitleAbsInt(center))
	last := min(len(audio), int(time.Duration(float64(end)*global.Scale)/frame)+radius+subtitleAbsInt(center))
	if last <= first {
		return subtitleAlignmentAnchor{}, 0, false
	}
	audio, activity = audio[first:last], activity[first:last]
	baseline, baselineOK := subtitleCorrelation(audio, activity, center)
	best, runner, bestShift := bestPiecewiseSubtitleShift(audio, activity, center, radius)
	if !baselineOK || best < 0.08 || best-runner < 0.01 {
		return subtitleAlignmentAnchor{}, 0, false
	}
	at := time.Duration(float64(regional[len(regional)/2].Start) * global.Scale)
	return subtitleAlignmentAnchor{At: at, Offset: time.Duration(bestShift) * frame}, best - baseline, true
}

func bestPiecewiseSubtitleShift(audio, activity []float64, center, radius int) (float64, float64, int) {
	best, runner, bestShift := -1.0, -1.0, center
	for shift := center - radius; shift <= center+radius; shift++ {
		score, ok := subtitleCorrelation(audio, activity, shift)
		if !ok {
			continue
		}
		if score > best {
			if subtitleAbsInt(shift-bestShift) > 8 {
				runner = max(runner, best)
			}
			best, bestShift = score, shift
		} else if subtitleAbsInt(shift-bestShift) > 8 {
			runner = max(runner, score)
		}
	}
	return best, runner, bestShift
}

func validPiecewiseSubtitleAnchors(anchors []subtitleAlignmentAnchor, globalOffset time.Duration) ([]subtitleAlignmentAnchor, bool) {
	changed := 0
	for index, anchor := range anchors {
		if absDuration(anchor.Offset-globalOffset) >= 500*time.Millisecond {
			changed++
		}
		if index > 0 && (anchor.At+anchor.Offset <= anchors[index-1].At+anchors[index-1].Offset || absDuration(anchor.Offset-anchors[index-1].Offset) > 30*time.Second) {
			return nil, false
		}
	}
	return anchors, changed >= 2
}

func subtitleInterpolatedOffset(at time.Duration, anchors []subtitleAlignmentAnchor) time.Duration {
	if len(anchors) == 0 {
		return 0
	}
	if at <= anchors[0].At {
		return anchors[0].Offset
	}
	for index := 1; index < len(anchors); index++ {
		if at <= anchors[index].At {
			span := anchors[index].At - anchors[index-1].At
			progress := float64(at-anchors[index-1].At) / float64(span)
			return anchors[index-1].Offset + time.Duration(float64(anchors[index].Offset-anchors[index-1].Offset)*progress)
		}
	}
	return anchors[len(anchors)-1].Offset
}
