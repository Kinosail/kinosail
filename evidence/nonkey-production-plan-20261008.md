# Non-key copied HLS repair: source-only test-first plan

Recorded 2026-10-08. This is a design and ownership inventory, not an implemented or qualified production repair.

## Exact source and ownership

- Main: `c452a2e73cebac42d280247fa9684efb630ecc93`.
- PR503 inspected head: `696f026d7e20678b0053185a37fac593cb61102a`, base mainc452.
- Existing terminal handoff: `68ca42f9ac61c29a776d4d05d979befa821d458b`.
- Public ownership request: https://github.com/Kinosail/kinosail/pull/503#issuecomment-6057737326.
- The request needs a disjoint sole writer allocation, or a concrete overlapping writer/path/function/head.
- No ownership response had been observed at 10:41 UTC. Silence is not an allocation.
- No production, existing test, normal workflow, simulator, deployment or local filesystem mutation is part of this plan.

The requested additional seams are byte-identical on main and inspected PR503:

| Path | Exact seam | Blob |
| --- | --- | --- |
| apps/player/internal/server/hls_copied_index.go | type copiedHLSTimeline | ec2889ab25f722fe343127c1c381285fa521798b |
| apps/player/internal/server/hls_copied_scan.go | hlsManager.selectCopiedHLSTimeline | 93be88f6d78149036beb460ad05ac5e8f9b4b656 |

Previously effective allocations retain the five copied clock/timeline/startup/metadata/endpoint helper files; the copied refill publication helper; narrow encodeVariant arguments/output and encodeVariants handoff/master gate; prepareSegment; startupWindowSegments; hlsSettings; serveRecipe; and new narrow remaining cache helpers/tests. They do not transfer whole enclosing files, lifecycle, scanner, client presentation or arbitrary source-generation functions.

PR503's current hls.go, hls_seek.go and startup_window.go differ from main. Preserve its rooted readHLSMasterRenditions/readHLSRecipeManifest, initializationsReady, measured copied EOF and terminal audio admission, including their dependencies. Do not transplant an older whole function over those guards. The copied clock/timeline/publication helper blobs are identical at both inspected heads.

## Regression before implementation

The regular generated source has 768 source video frames at 24fps and genuine EOF at 32s. Expected rows below are obligations for new tests, not runtime results.

| Requested origin | Independent decode key | First requested frame | Raw decoded frames retained | Requested presented frames | First public cut | Total presentation | Four physical GOPs cover |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 12.5s | 12s / frame288 | frame300 | 480 | 468 | 1.5s | 19.5s | 7.5s |
| 13.5s | 12s / frame288 | frame324 | 480 | 444 | 0.5s | 18.5s | 6.5s |
| 18.2s | 18s / frame432 | frame437 | 336 | 331 | 1.8s | 13.8s | 7.8s |

Write public cold, prepared, two reopen, missing-zero regeneration and lazy-refill cases for all three offsets and both containers. Each test obtains the actual public response and bytes. Test assertions must fail against current source before implementing the relevant repair. Do not turn a recorded raw requested-sequence failure into a passing result by trimming rows.

1. Selection: legacy exact-key control still matches exactly; non-key requests are admitted only by a new explicitly qualified strategy. Reject before-first-key, at/past genuine EOF, nonfinite, out-of-bounds, ambiguous and unsupported source cases without claiming prepared support.
2. Origin: physical Keys and point() remain source IDR coordinates. Requested presentation origin and certified preceding independent decode key are separate immutable facts. Verify an exact source-IDR/configuration certificate; no key-flag shortcut.
3. Cuts: first projected presentation duration is next source key minus requested origin. Subsequent cuts remain exact source key differences. Validate physical and presentation extents separately.
4. Startup: four GOPs at these origins must not satisfy the unchanged eight-second window. Five available cuts do; a genuinely completed shorter tail may satisfy the existing EOF rule. A pending or inert EVENT prefix must not certify completion.
5. Refill: first later source key maps to 1.5/0.5/1.8 seconds respectively. Requesting a later missing segment must regenerate its exact source cut without gaps, overlaps, source-frame duplication or a master/init rewrite.
6. Reopen: two cold manager reopens reuse the bound canonical generation and corrected init, with zero new encoder where the existing cache contract requires it.
7. First fragment: absent zero is regenerated into a rooted bounded private stage, joined, checked and published create-if-absent. A concurrent winner remains untouched.
8. Failures: replace source/root/rendition/init/first/timeline/certificate/manifest during admission and publication. Existing no-write, inode, metadata, policy and cancellation assertions stay exact.
9. Preserve ordinary absent-index cold H264 and HEVC, sparse controls, exact-key prepared streams, audio-transcode controls and source-origin AAC controls. Unsupported new non-key eligibility must not delete or invalidate ordinary cold caches.
10. Add one-sample AAC and one-frame video negative controls. They must fail the same exact reference assertions; no increased tolerances, skips, retries or deadlines.

Keep all PR517/527 tests unchanged, including unique JSON, rooted generation, bounded stage/assets, canonical publication, cancellation/join and EOF endpoint guards. Added tests must fit the existing 300 physical line cap.

## Proposed production representation

Use a separately identified non-key strategy and a small typed presentation mapping. The exact spelling/schema remains subject to source review after allocation.

- Requested origin uses an exact bounded rational or integer microsecond coordinate checked against recipe.offset and source identity.
- Decode origin is the independently certified preceding source IDR's exact PTS/DTS/time base.
- Physical source Keys, point() and the scanner/IDR proof retain their existing meaning.
- Presentation time is source time minus requested origin, with an explicitly certified negative decode preroll where needed.
- Initial presentation extent, native audio clock association and generated track edits are bound to the same source/policy, timeline bytes, canonical root/rendition, init and first fragment.
- The old h264-idr-keys-1 and its Clock>=0 admission remain unchanged. Do not store a negative physical origin in old Clock, fake Clock0, reinterpret point(0), or relax decodeCopiedHLSClock.
- New state is unreadable as ready until its own generated-media proof and asset binding are complete. Existing read/serve/reopen/refill consumers dispatch explicitly by strategy.
- Unknown/duplicate metadata, unsupported versions, missing mapping, invalid signed bounds or mismatched identities reject. They do not fall through to raw indexed manifests.
- No client/source descriptor change is assumed. Current native code uses selected.offset as the requested source offset, so media presentation zero must actually denote that requested origin.

Selecting a preceding key alone is insufficient. Initial seek arguments, physical manifest checks, public cut projection, refill offsets, generated-init edits and native presentation need the same mapping. Preserve previous independent decode rows; do not discard prerequisite frames to make a raw decoder count appear correct. Source PTS>=0, packet scanning, point(), source IDR and SPS/PPS certification remain unchanged.

## Consumer closure within allocated source

| Consumer | Required behavior |
| --- | --- |
| hls_copied_scan.go / selectCopiedHLSTimeline | New non-key selection separately certified; preserve exact-key, policy, adoption and idle checks. Pending allocation. |
| hls_copied_index.go / copiedHLSTimeline | Bind both origins without changing Keys/point semantics. Pending allocation. |
| hls_copied_timeline.go / validation, copiedHLSManifest, matchesCopiedHLSLength | Preserve old strategy validation. Validate new physical cuts and serialize projected presentation cuts explicitly. |
| hls_copied_startup.go / copiedHLSSeekArguments and copied projections | Retain earlier independent key only for new certified preroll. Keep legacy copypriorss0/refill behavior. Dispatch public projection only after proof. |
| hls_copied_clock.go / ensure, bind, commit | Old nonnegative Clock proof stays exact. New proof binds signed physical/preroll mapping and corrected generated init atomically before ready. |
| hls_copied_metadata.go / certificate verification | Strict schema/hash/generation checks bind new mapping and edits; old certificate behavior preserved. |
| hls_copied_endpoint.go / endpoint projection | Genuine generated EOF remains authoritative; map it to requested presentation extent without granting format-duration-only assets. |
| hls_seek.go / prepareSegment | Offset is source key minus requested presentation origin, while retaining rooted manifest and terminal/source guards. No lifecycle transfer. |
| hls_job.go / hlsSettings | Bind new strategy/correction policy identity to the recipe; preserve current PR503 hls18 and unsupported-source behavior. |
| hls_playlist_session.go / serveRecipe | Validate new strategy, policy and generated asset binding before cached init/full/range fragment fast paths; preserve rooted descriptor admission. |
| startup_window.go / startupWindowSegments | Sum projected presentation cuts; unchanged eight-second readiness and genuine EOF rule. No transfer of other readiness functions. |
| hls_copied_recovery_publication.go / staging, arguments, snapshot, validPrefix, publish | Preserve bounds, exact init/first/certificate/generation, no-overwrite, joined owner and canonical master rules for new mapping. |
| hls_presentation.go / hlsSegmentArguments initial mux slice | Add only causally verified fMP4 timestamp/edit options; preserve formats, names, modes, start numbers and unaffected transcode/audio flags. |
| hls.go / already allocated argument/output slices and handoff/master gate | Separate input decode coordinate from output presentation coordinate only within these slices. Retain absolute seek_timestamp1 and current source/codec policy. |

Any implementation requiring another existing function must identify its exact seam and coordinate before editing. New helpers split to respect the physical line cap; this plan grants no blanket file ownership.

## AAC mechanism and bounded eligibility

Accepted fixed-fixture causality is run37760596546, source919bf6051f8b58a431239779d50235e4702964f1, receipt b07ec9828dc0cd2b22359374d74d1af3f4ec4ece0fffad5ff5b140cd59c83ead. All18 observations and68 controls matched. Historical rejected coarse-PTS model and original failures remain in the terminal ledger.

The measured stateful CLI normalized sample clock and exact first copied AAC packet/native-frame identity produce these full-EOF reference results:

| Offset | MKV derived edit delta | MKV native samples | MP4 derived edit delta | MP4 native samples |
| --- | --- | --- | --- | --- |
| 12.5s | -16 samples | 936000 | 0 | 936008 |
| 13.5s | -16 samples | 888000 | 0 | 888008 |
| 18.2s | 0 | 662400 | 0 | 662408 |

All six adjacent edit controls differ by one sample. Every complete per-track packet row, payload/order/count, raw video row, source/generated asset and EOF assertion stays intact.

The production algorithm is not yet qualified. A container-name -16 rule and local raw ffprobe interpolation are rejected. Production needs a bounded, independently associated normalized source clock and unique first copied packet identity. Reference PCM must not choose the correction.

Start with failing bounded-clock qualification tests. Keep the existing two-second metadata commit/verification lease unchanged. Any source-clock preflight is separately bounded within the existing preparation/job lifetime and one-thread/resource limits; it must finish before final metadata admission and bind the same current source/generation. Do not add a nested encoder reservation or increase any existing deadline. Measure only enough source state to independently establish the candidate's clock association; reject ambiguity, timestamp discontinuity, sample-rate/format changes, unsupported delay/skip metadata and budget exhaustion. A full arbitrary-duration decode is not an acceptable implementation. The bounded preflight method is not yet qualified; unproved eligibility remains unadmitted by the new strategy.

Apply an admitted audio edit only to a private generated init before public ready/master publication. Certify the corrected init and first fragment together; later refill/reopens retain their canonical bytes. Never modify original media or an already published init.

## Unchanged native and presentation acceptance

Actual renderer qualification remains separate from complete raw decode and PCM proofs. The prior MP4 native control presented a preceding frame and produced two InvalidStateErrors; the prior MKV native result did not establish all quality assertions. These failures stay recorded. Historical raw requested-sequence and decoded-quality-equality assertions remain failed; a separately guarded new presentation result does not make those historical assertions pass.

The native/presentation owner must run the existing actual-media assertions unchanged, with added offset cases rather than weaker expectations:

- First presented source frame300/324/437 and exact subsequent requested source order to genuine EOF, while the separate raw decode record retains480/480/336 rows.
- Exact full-EOF native PCM sample count/SHA reference and complete packet/skip/delay/side-data association.
- Current native source clock maps media time back to the requested offset without rebound; resume, seek, reentry and source-generation cancellation retain their existing assertions.
- Existing first-moving-frame/startup deadline, console/InvalidStateError, black-video, duration/endpoint, progress/retry and fullscreen/readiness assertions remain unchanged.
- Actual preparation adoption-before-cancel is explicitly exercised. Stopped-then-refill evidence with two sequential joined encoders and peak1 does not prove adoption or a leak.
- Existing worker stop/cancel/join and resource assertions remain; no forced termination is counted as a natural joined success.

Hosted Go/public-media proof can establish backend/certificate/cache behavior after the source allocation and independent exact-source review. Hosted browser checks do not replace the same-iOS all50 retest or renderer ownership.

Same-iOS retest remains assigned to the existing native owner, using the preserved baseline video IDs and receipts when the coordinator releases capacity. The coordinator-verified current live Nox baseline is PR514 revision 389090769f31fc639e7c3b95655e131b5d79cef4, image sha256:e9687afb5f5e7e7c267d0ebf88b0b61296907d270c2bafeb6106470f6fee2e2c, deployed 2026-10-07 at 22:29 UTC. PR517/527 are published but have not been deployed to Nox. Historical f532 receipts remain preserved as earlier evidence; f532 is not the current live baseline. No simulator interaction, Nox update, deployment, local build/download/cleanup or Library write is authorized by this plan.

## Delivery admissions and next action

1. Obtain explicit allocation or concrete source overlap for the two requested seams through the PR503 source-only channel. Reconcile its fresh head and all incoming guards before writing.
2. Add failing tests on a separate recovery branch rooted in then-current main. Implement the smallest admitted mapping and bounded AAC mechanism within allocated source.
3. Independently review the exact candidate. Run bounded existing hosted public-media proof, preserving each failed attempt and unchanged control.
4. Run normal CI; all four required exact-head checks must be green. Track separate browser/native failures honestly rather than treating required CI as presentation acceptance.
5. Open a narrow linked draft with exact admitted scope, source/proof identities and remaining native boundary. Ordinary protected merge requires review/guard/public proof admissions and reconciliation of current main and competing publication.
6. Verify merged ancestry and normal signed publication. No manual deployment.
7. Complete the same-iOS all50 retest with the native owner after capacity release; no claim of completion before its actual receipts.

Independent plan review read exact bd7fde23/blob04ec7d99 and admitted its fixed-fixture geometry, signed-strategy separation and preservation boundaries. This revision adds the requested policy/serve fast-path closure and separates bounded source preflight from the unchanged metadata lease. This is design review only.

Current blocking facts: the source owner has not allocated the two new seams or identified overlap; bounded production AAC clock eligibility is unproved; native requested-first-frame and full50 acceptance remain unqualified. The completed fixed-fixture counterfactual is useful mechanism evidence, not closure of these blockers.
