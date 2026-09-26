# Subtitle excellence: evidence and default policy

Date: 2026-09-26

## Goal

Find the right human subtitle, keep its words intact, align it to the exact media cut, and deliver it as a separate text track. Optimize for very few wrong automatic installs and very little subtitle-caused video transcoding. No algorithm can guarantee perfect timing or text for every release. An uncertain result must remain available for review rather than silently replace a good file.

## Failure inventory from other apps

These are issue reports, not measured Kinosail failures. Closed issues remain useful regression examples.

| Failure | First-party evidence | Kinosail response |
| --- | --- | --- |
| A reported sync result damages an already correct track; the reports also dispute whether the default quality guard was skipped | [Bazarr #3599](https://github.com/morpheus65535/bazarr/issues/3599), [#3608](https://github.com/morpheus65535/bazarr/issues/3608) | Compare the proposed transform with the original across independent regions. Reject implausible scale, offset, and local regressions. Keep the guard unconditional and retain the original. |
| Extended, theatrical, and other cuts share titles but differ inside the video | [Bazarr #3232](https://github.com/morpheus65535/bazarr/issues/3232), [FFsubsync split guidance](https://github.com/smacke/ffsubsync) | Treat cut metadata as identity evidence. Fit a bounded number of step changes for actual edits, continuous drift for clock or frame-rate changes, and abstain when neither model explains the audio. |
| Wrong episode inside an archive | [Bazarr #2578](https://github.com/morpheus65535/bazarr/issues/2578) | Match each extracted member to season and episode before scoring or writing. |
| Forced-only or hearing-impaired tracks satisfy the wrong coverage rule | [Bazarr #3607](https://github.com/morpheus65535/bazarr/issues/3607), [#3579](https://github.com/morpheus65535/bazarr/issues/3579) | Keep language, full dialogue, forced, and SDH as separate facts. Never count forced-only text as full dialogue. |
| Provider failures and upgrade loops hide missing coverage | [Bazarr #3398](https://github.com/morpheus65535/bazarr/issues/3398), [#3235](https://github.com/morpheus65535/bazarr/issues/3235) | Distinguish invalid credentials from rate limits, honor retry headers, and require a material quality gain before replacing an installed result. |
| Subtitle burn-in stalls or needlessly converts video | [Jellyfin codec guide](https://jellyfin.org/docs/general/clients/codec-support/), [Jellyfin #11938](https://github.com/jellyfin/jellyfin/issues/11938), [#2203](https://github.com/jellyfin/jellyfin/issues/2203) | Convert supported text formats to WebVTT for browser delivery without touching video. Use text sidecars or HLS WebVTT on native clients. Keep image-track OCR as a reviewed fallback; bitmap tracks cannot be losslessly represented as text. |
| Whole-library scans consume too much memory or reread unchanged media | [Bazarr #3241](https://github.com/morpheus65535/bazarr/issues/3241), [#3419](https://github.com/morpheus65535/bazarr/issues/3419) | Bound work per cycle, cache facts by media version, and keep browsing responsive while background work runs. |

## Recommended automatic pipeline

1. **Local first.** Keep owner files and matching embedded text. Record their language and role independently. A text track can be delivered as WebVTT without video encoding. Apple HLS supports external WebVTT with a timestamp map; native clients need that path and device verification. [Apple HLS authoring specification](https://developer.apple.com/documentation/http-live-streaming/hls-authoring-specification-for-apple-devices/)
2. **Search by identity.** Use title IDs, episode identity, release tokens, cut, language, role, and exact file hash when available. Reject contradictions before ranking. A provider score alone cannot override a wrong episode or cut.
3. **Validate every downloaded candidate.** Bound archive bytes and members; parse text before writing; reject empty, corrupt, mislabeled, or implausible files. Preserve dialogue while normalizing encoding and cue structure.
4. **Diagnose timing before changing it.** Compare original cue activity with the selected dialogue audio. Try identity, global offset, and known frame-rate ratios. For genuine gradual drift, fit a tightly bounded affine map. For ads or scene edits, allow a small number of penalized step changes; do not smear one cut across unrelated scenes. FFsubsync and ALASS show the value of VAD, frame-rate search, and piecewise matching. Their published results are not Kinosail accuracy measurements. [FFsubsync](https://github.com/smacke/ffsubsync), [ALASS](https://github.com/kaegi/alass)
5. **Accept only a proven improvement.** Require start, middle, and end coverage; no newly bad region; plausible model parameters; monotonic cues; bounds within media duration; and a meaningful gain over the original. Retain a recovery copy. Never compare raw alignment scores across different videos. [FFsubsync maintainer's score explanation](https://github.com/smacke/ffsubsync/discussions/148)
6. **Use speech models as optional evidence.** Qwen3-ForcedAligner reports 11 supported alignment languages; its benchmarks do not establish accuracy on a household library. Run local, bounded experiments against corrected reference subtitles before considering default use. Use generated ASR/OCR text only as a marked draft when no human subtitle exists. [Qwen3-ASR report](https://arxiv.org/abs/2601.21337), [IWSLT 2026 system](https://aclanthology.org/2026.iwslt-1.7/)
7. **Expose reasons and recovery.** Show source, language, role, match evidence, timing method, and why automatic work abstained. Let the Owner mark a bad result so the same provider file is not installed again. Preserve an undo path for every automatic replacement or removal.

## Evaluation before claiming best-in-class

Build a consented test corpus across movies, episodes, animation, sparse speech, SDH, translated tracks, forced tracks, multiple languages, 23.976/25 fps, ads, recaps, alternate cuts, and direct-play devices. Include deliberately wrong titles and already-correct subtitles. Measure wrong-title installs, owner-file loss, cue timing error at start/middle/end, completeness, SubER against corrected references, abstention rate, CPU/time, and video-transcode rate. SubER measures text, segmentation, and timing together; it does not replace human review. [SubER paper](https://aclanthology.org/2022.iwslt-1.1/), [IWSLT 2026 evaluation](https://iwslt.org/2026/subtitling)

The current app already has provider ranking, bounded cleanup, VAD alignment, sidecar recovery, and WebVTT endpoints. The highest-value remaining work is a baseline-preserving synchronization gate, true cut detection, a verified native WebVTT path, and a representative corpus. The language cleanup action applies to explicitly identified sidecars and sets one preferred language to avoid automatic refetch. Embedded media remains separate because modifying embedded tracks requires a media-container rewrite.
