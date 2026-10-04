# Subtitles isolated pipeline controls: failure analysis before edits

Baseline: `876b771a7dd0aef5e65957fcee87add0312535e8`. This analysis is committed before changing existing isolated assertions. No new test declaration or production behavior is planned.

## SubSource candidate rejection

Existing declaration: `TestSubSourceRejectsUnsafeOrAmbiguousResults`.

The provider can return a subtitle for another movie, language or episode; machine-translated, forced or foreign-only dialogue; an excessive archive file count; or an oversized result set. Installing any such candidate risks incorrect dialogue or unbounded work. Existing genuine E2E does not inject these provider responses.

The current negative table cannot reach those checks. It supplies zero pagination Page/Pages, rejected by `validSubSourcePagination`. It also passes requested language `english`, rejected by canonical language normalization; actual `subSourceProvider.search` passes `en`.

Repair the existing table using canonical `en`, admitted pagination and a valid unmutated candidate positive control before each mutation. For the 51-result bound, use 51 otherwise accepted candidates with admitted pagination; zero-value candidates would conceal a removed count bound. Assert admission of a single valid candidate first.

Validate by compiling controls with the candidate rejection guard disabled and, separately, the 50-result response limit disabled. The original should remain green for unrelated reasons, while the repaired declaration must fail at unsafe candidate admission or oversized response admission. Restore production source byte for byte and confirm the repaired control passes.

## Preservation during confident synchronization

Existing declaration: `TestSynchronizeCandidateRejectsPreservedSubtitle`.

A preserve-required provider file must remain byte-identical. A confident audio mismatch must reject it rather than alter timing or accept mismatched original bytes. Aligned preserved bytes must still be accepted. Analysis unavailability, weak evidence and ordinary non-preserved correction remain separate failure paths.

The original has neither cues nor audio and an unavailable FFmpeg path. Speech analysis rejects it before the `Preserve && changed` branch. Existing no-audio SubSource acquisition protects raw-byte preservation but cannot prove this confident mismatch boundary.

Repair the existing declaration with valid synthetic cues and preloaded speech shifted by seven seconds. A non-preserved positive control must succeed and change bytes; the preserved candidate must return `errSubtitleTimingMismatch`. Aligned speech must accept preserved input without changing bytes. Preloaded `attempt.audio` uses the real synchronization logic and needs no new production seam or fake executable.

Compile a control with only the changed-preserved rejection disabled. Record original green, repaired red and restored green. Save commands, revision, source checksums and JSON logs. Synthetic speech proves deterministic synchronization/preservation policy, not real FFmpeg extraction or speech-recognition accuracy.

## Spoken text during cleanup

Existing declaration: `TestSubtitleCleanupPreservesSpokenURLsAndRepeatedDialogue`.

Removing a spoken URL while preserving cue count would lose dialogue. Merging separated repeated dialogue would also lose timing. Current assertions protect the three-cue/zero-duplicate boundary but omit text. Add exact expected spoken URL and repeated cue text to this existing declaration after this analysis. Existing real browser data has no spoken URL and offers no equivalent text-loss control.

## Retained partial controls

- `TestLocalSubtitleDraftRejectsUnavailableDependenciesWithoutStarting` independently covers closed/running/no-probe/no-FFmpeg/duration/track failures. Its recognition-tool case can pass on later FFmpeg denial and remains an explicit gap until a deterministic dependency-specific control is written and validated.
- `TestSubtitleAppExtractsPreferredEmbeddedTextBeforeProviderSearch` proves app extraction/conversion with scripted tools, but no provider is configured or observed. Rename the existing declaration to describe text extraction; retain the provider-priority gap.
- `TestSubtitleReviewExplainsMatchingHistoryAndExportsOriginal` proves six ledger source projections and current-file recognition, but weak explanation text and no export call. Rename the existing declaration to describe that actual projection. Retained original-export tests independently verify bytes and tamper rejection; meaningful per-source explanation remains a gap.
