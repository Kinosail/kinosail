# Automatic intro, recap, commercial, outro, and credit detection

Research snapshot: 2026-08-26.

This note evaluates practical automatic playback-segment detection for Kinosail using primary research papers, official product/project documentation, official source, and the current working tree. It covers detection and the decision to auto-skip. Physical omission/remux is covered separately in [Automatic skip without whole-file transcoding](automatic-skip-without-full-transcoding.md).

## Executive decision

There is no credible single state-of-the-art model for all five labels. The strongest systems are task-specific, use incompatible private datasets, and report metrics that cannot be compared directly. Kinosail should build a **local, typed, confidence-gated cascade**:

1. Treat owner markers, named chapters, imported sidecars, and semantically specific in-band broadcast cues as authoritative evidence.
2. Detect recurring intros/outros with cached **audio plus sparse visual recurrence**, using temporal alignment and season consensus. Chromaprint stays the cheap first lane; MPEG-7/perceptual signatures are the no-model visual lane, and embeddings cover silent, re-scored, and visually stable sequences that simpler signatures miss. For Movies, detect studio/distributor idents only through an owner-approved reusable template or equally strong cross-library recurrence; never infer “the story starts here” from position alone.
3. Detect credits from **text density/motion plus visual/audio structure**, not black frames alone. Retain multiple credit ranges around mid- and post-credit scenes.
4. Detect commercials only for finalized DVR/broadcast recordings. Parse preserved SCTE-35 ad/break signaling first, use Comskip as the fallback lane, then evaluate a two-stage audio-boundary plus audio-visual classifier against it.
5. Treat untagged recaps as a semantic problem. Subtitle/ASR phrases and matches to prior Episodes may generate a candidate, but only a validated multimodal temporal model should auto-skip it.
6. Fuse evidence per type, decode a legal timeline, calibrate on a held-out Kinosail corpus, and **abstain** when risk is too high. Optimize first for false skipped story seconds, not aggregate F1.

This is the best evidence-backed design for a private self-hosted server: authoritative metadata where available, cheap local recurrence where repetition is the signal, learned semantics only where necessary, and no hosted dependency. Generated markers may auto-skip only after their detector/type reaches the configured high-confidence tier; medium-confidence markers prompt, and low-confidence candidates stay hidden.

There is therefore no honest “absolute” semantic detector. Authored metadata, strictly interpreted broadcast events, and exact matches to approved reusable templates can be deterministic within their declared scope. Every content-inference method can be wrong and must retain an abstention path.

## Semantics Kinosail should preserve

Use distinct internal meanings even when a compatibility client has fewer types:

| Type | Kinosail meaning | Strongest practical evidence |
| --- | --- | --- |
| `recap` | Prior-story summary before the Episode proper | Named chapter; multimodal sequence model; phrase seed plus dense matches to prior Episodes |
| `intro` | Opening title/theme sequence | Named chapter; recurring audio and visual sequence |
| `commercial` | Advertising break inside recorded broadcast content | Sidecar/import; broadcaster-tuned multi-cue or learned DVR analysis |
| `outro` | Recurrent closing sequence or next-Episode preview before explicit credits | Named chapter; recurring tail audio/visual sequence |
| `credits` | Opening or ending credit roll/cards | Named chapter; sustained text/visual temporal pattern |

Jellyfin formalizes media segments as typed begin/end metadata and currently defines `Commercial`, `Preview`, `Recap`, `Outro`, and `Intro`; clients choose the action. It runs scans in the background and ships an official chapter-name provider. [Jellyfin media segments](https://jellyfin.org/docs/general/server/metadata/media-segments/)

Kinosail can keep its more precise internal `credits` type and map it to Jellyfin `Outro`, as the current adapter does. That lossy compatibility mapping should not collapse the Kinosail domain model.

“Actual Movie start” is not an objective audiovisual label. A studio/distributor ident, opening credits, a cold open, and story content can overlap or appear in different orders. Kinosail may expose a proven studio-ident interval as an `intro` through the current public type set, but its evidence subtype must remain `ident-template`; that prevents a position/black-frame heuristic from being mistaken for a semantic story boundary.

## What 2023-2026 research changes

### Modern visual temporal models are promising, but not a drop-in dependency

A 2025 preprint, revised in March 2026, directly targets intro/credit detection. It samples 224×224 frames at 1 FPS, encodes them with CLIP, and classifies 60-second windows with a 16-layer multihead-attention model. On a series-disjoint test split from 972 Episodes/27 hours it reports 89% precision, 97% recall, and 91% F1; the FP16 model is 290 MB, and preprocessing time is excluded from its runtime table. The model combines intro and credits into one label, excludes recaps, still misses overlaid credits, and does worse on intros under five seconds. The paper does not publish a reproducible training corpus or redistributable checkpoint, so the 2026 revision still validates the **architecture direction**, not a Kinosail dependency. [Korolkov and Yanchenko, current revision](https://arxiv.org/abs/2504.09738)

The metric also cannot be compared directly with boundary-F1 results: the 2025 work reports per-second binary classification after balancing each positive sequence with equal-length film content, while Hao et al. score detected start/end boundaries. Kinosail should require item-level detection, boundary error, and false-story-seconds evaluation on natural class prevalence before accepting either design. [2025 dataset and evaluation](https://arxiv.org/pdf/2504.09738), [Hao et al. evaluation](https://openaccess.thecvf.com/content/WACV2021/papers/Hao_Intro_and_Recap_Detection_for_Movies_and_TV_Series_WACV_2021_paper.pdf)

Modern video-copy localization supplies the missing visual recurrence lane. A 2024 ECCV method extracts sparse frame features, forms a frame-to-frame similarity map, and localizes diagonal copied regions. Its self-supervised ViT-small plus detector reached 68.83% segment F1 and 95.42% video-level F1 on VCSL; full fine-tuning reached 71.56%/96.46%. Those numbers are on adversarial copied-video benchmarks, not TV intros, and the ViT-small/YOLOX training stack is excessive for Kinosail. The useful result is the robust pattern: **cached frame embeddings + similarity matrix + temporal alignment**, with audio recurrence as an independent modality. [Lu et al., ECCV 2024](https://www.ecva.net/papers/eccv_2024/papers_ECCV/papers/01818.pdf)

TransVCL similarly learns an enhanced similarity matrix and temporal alignment from frame features. Its official implementation reports 66.51% segment F1 on VCSL and is MIT-licensed, but it requires a Python/PyTorch model stack and pretrained features. It is research evidence for alignment, not production code Kinosail should copy. [TransVCL paper](https://ojs.aaai.org/index.php/AAAI/article/view/25158), [official implementation and license](https://github.com/transvcl/TransVCL)

DINOv3 is the newer 2025 Meta feature family and publishes compact 21M-parameter ViT-S and 29M-parameter ConvNeXt-Tiny variants, but it has no playback-segment benchmark, requires gated weight access, and uses a custom DINOv3 license for both code and weights rather than DINOv2's Apache-2.0 baseline. It is not a justified automatic upgrade for Kinosail. Benchmark the already available no-model signature lane first; if an embedding is still needed, keep DINOv2 as the permissively licensed reference until a candidate is measured on Kinosail's corpus. [DINOv3 official repository](https://github.com/facebookresearch/dinov3), [DINOv3 license](https://github.com/facebookresearch/dinov3/blob/main/LICENSE.md)

### FFmpeg already supplies a no-model visual recurrence probe

FFmpeg's `signature` filter calculates the ISO/IEC MPEG-7 Video Signature, can compare multiple inputs, distinguishes whole-video from partial matches, and exposes thresholds for per-word similarity, per-frame similarity, minimum matching-sequence length, and matching-frame ratio. The standard defines video-signature tools for copy identification. Kinosail's pinned Jellyfin FFmpeg build already exposes this filter, so it is the smallest visual-template experiment before adding ONNX weights. [FFmpeg `signature` filter](https://ffmpeg.org/ffmpeg-filters.html#signature), [ISO/IEC 15938-3 video-signature amendment](https://www.iso.org/standard/54890.html), [pinned FFmpeg package](../../Containerfile)

This is not a ready-made semantic classifier. FFmpeg documents signature-file output and match calculation but no stable JSON result schema; its implementation reports match indices, frame-rate ratio, offset, score, and matched-frame count through logs. A Kinosail adapter must be version-pinned, output-bounded, fail closed on any parse drift, and remain prompt-only until fixture and corpus evaluation proves its boundary behavior. [FFmpeg matching implementation](https://github.com/FFmpeg/FFmpeg/blob/master/libavfilter/signature_lookup.c)

### Repeated audio remains the best cheap first detector

Plex says intro analysis is processor intensive and compares an Episode with other Episodes in its season. It ignores candidates shorter than 20 seconds or ending after the Episode midpoint and runs analysis during maintenance or media-add workflows. [Plex intro detection](https://support.plex.tv/articles/skip-content/), [Plex CPU explanation](https://support.plex.tv/articles/201697383-why-is-plex-using-my-cpu/)

Chromaprint is designed for near-identical audio, duplicate detection, and long-stream monitoring. Its own documentation says it is not a general-purpose fingerprint and trades precision/robustness for search performance. That is a good first-stage trade for recurring themes but cannot classify semantically similar or re-scored sequences. [Chromaprint README](https://github.com/acoustid/chromaprint/blob/master/README.md)

The Jellyfin Intro Skipper project demonstrates a practical bounded pipeline: cache opening/tail fingerprints, compare Episode pairs by alignment offsets and Hamming distance, and retain contiguous shared runs. It is GPL-3.0-only, so it is evidence rather than code for Kinosail to copy. [fingerprint comparison](https://github.com/intro-skipper/intro-skipper/blob/e09843f2f972d7fe2b7d98fe3ed6b898f4978459/IntroSkipper/Analyzers/ChromaprintAnalyzer.cs#L193-L523), [bounded ranges](https://github.com/intro-skipper/intro-skipper/blob/e09843f2f972d7fe2b7d98fe3ed6b898f4978459/IntroSkipper/Data/QueuedEpisode.cs#L143-L155), [license](https://github.com/intro-skipper/intro-skipper/blob/e09843f2f972d7fe2b7d98fe3ed6b898f4978459/LICENSE)

The maintained 2026 fork now separately hashes analysis configuration and extracted-evidence configuration, and its credits path adds keyframe-entropy fallback to black-frame proposals. It publishes no held-out accuracy, so this is operational evidence for versioned caches and multi-cue fallback—not proof that its markers are safe to auto-skip. The fork remains GPL-3.0. [configuration hashing](https://github.com/intro-skipper/intro-skipper/blob/4861a5e943da6ec36b63e677fbdee2876d4c4778/IntroSkipper/Helper/ConfigHasher.cs), [credit entropy fallback](https://github.com/intro-skipper/intro-skipper/blob/4861a5e943da6ec36b63e677fbdee2876d4c4778/IntroSkipper/Analyzers/Credits/CreditEntropyFallback.cs), [current license](https://github.com/intro-skipper/intro-skipper/blob/4861a5e943da6ec36b63e677fbdee2876d4c4778/LICENSE)

### Repetition does not solve recap semantics

Hao et al. identify three failures of unsupervised recurrence: shared story content creates false positives, singleton titles cannot work, and per-Episode-changing intros do not match. Their supervised model fuses visual and audio CNN features, models time with a bidirectional LSTM, and decodes with a CRF. It was trained on 46,946 manually reviewed titles using each title's first ten minutes. At a strict one-second tolerance the reported F1 was 70.85% for intros and 70.57% for recaps; at three seconds both rose to about 80%. Its 70 ms inference figure excludes feature extraction and was measured on a Tesla V100. [paper](https://openaccess.thecvf.com/content/WACV2021/html/Hao_Intro_and_Recap_Detection_for_Movies_and_TV_Series_WACV_2021_paper.html), [evaluation](https://openaccess.thecvf.com/content/WACV2021/papers/Hao_Intro_and_Recap_Detection_for_Movies_and_TV_Series_WACV_2021_paper.pdf), [tolerance supplement](https://openaccess.thecvf.com/content/WACV2021/supplemental/Hao_Intro_and_Recap_WACV_2021_supplemental.pdf)

The 2025 CLIP work explicitly excludes recap because visual-only distinction is ambiguous. [Korolkov and Yanchenko limitations](https://arxiv.org/pdf/2504.09738)

Recent copy-localization research does make an additional local signal feasible: after a “previously on” subtitle/ASR seed, sparse visual matches against prior Episodes can measure whether a run is a montage of prior story. That is a Kinosail design inference from the VCL work, not a published recap accuracy result. It should improve candidate boundaries, but it must remain prompt-only until validated.

### Credits require text and temporal context, not black alone

FFmpeg exposes inexpensive candidate cues: `blackdetect` emits black intervals and exact metadata, `blackframe` reports per-frame black-pixel percentage, `silencedetect` emits low-energy intervals, and `entropy`/`signalstats` expose visual statistics. FFmpeg explicitly lists chapter/commercial detection as a `blackframe` use. [FFmpeg filter documentation](https://ffmpeg.org/ffmpeg-filters.html)

No single cue is a semantic label. Dark drama scenes, fades, letterboxing, and silent scenes create false positives; bright, animated, and live-action-overlay credits create false negatives. The 2025 CLIP paper reports that temporal visual semantics outperform its heuristic baseline but still miss overlaid credits. [Korolkov and Yanchenko results and error analysis](https://arxiv.org/pdf/2504.09738)

The deployable lean ensemble should therefore combine text-region density/persistence, vertical text motion, low scene-change rate, luma/entropy/saturation, black/silence boundaries, and tail position. Tesseract is an Apache-2.0 OCR engine that can supply recognized-text confidence, but exact recognition is unnecessary: stable text-box geometry and density are the stronger language-independent cues for rolls/cards. FFmpeg also exposes an `ocr` filter backed by libtesseract, but Kinosail's pinned Jellyfin build does not currently compile that filter, so OCR is a deliberate image/dependency addition rather than a free existing capability. [Tesseract official documentation](https://tesseract-ocr.github.io/tessdoc/Installation.html), [FFmpeg `ocr` filter](https://ffmpeg.org/ffmpeg-filters.html#ocr), [Kinosail FFmpeg package](../../Containerfile)

If Tesseract localization proves insufficient, PaddleOCR is the strongest current lightweight local OCR experiment: its Apache-2.0 project publishes trainable models, the 2025 PaddleOCR 3.0 report targets local deployment, and the 2026 PP-OCRv5/PP-OCRv6 work includes 5M and 1.5M-34.5M parameter tiers. Those benchmarks are OCR benchmarks, not credit-boundary evaluation. Kinosail should invoke OCR only on bounded candidate frames and use box geometry, motion, and text-role density as evidence; recognized names or phrases alone must never authorize a skip. [PaddleOCR 3.0](https://arxiv.org/abs/2507.05595), [PP-OCRv5](https://arxiv.org/abs/2603.24373), [PP-OCRv6](https://arxiv.org/abs/2606.13108), [official repository and license](https://github.com/PaddlePaddle/PaddleOCR)

Plex preserves mid-/post-credit scenes by jumping to the next scene rather than skipping from the first detected credit frame to EOF; it uses a separate final marker for post-play. It also supports local-only analysis or hash-addressed marker reuse. Kinosail should likewise store **multiple credit ranges** and keep intervening content watchable. [Plex credits detection](https://support.plex.tv/articles/credits-detection/)

### Commercial detection has a strong modern two-stage pattern

For source recordings that preserve it, SCTE-35 is stronger than content inference. ANSI/SCTE 35-1 defines in-stream messages for advertising breaks/content, programming content, splice events, and returns from breaks across MPEG-2 TS, DASH, and HLS. The 2026 SCTE 35-2 standard adds a streamlined event descriptor and is intended to replace 35-1 integrations. These messages can identify a placement opportunity rather than prove that every enclosed frame is an advertisement, so Kinosail should auto-accept only allowlisted, paired advertisement/break event semantics; generic splice opportunities are evidence for Comskip/classifier fusion. [ANSI/SCTE 35-1 2023r2](https://account.scte.org/standards/library/catalog/scte-35-1-digital-program-insertion-cueing-message-part-1-legacy-splice-based-and-time-based-signaling/), [SCTE 35-2 2026](https://account.scte.org/standards/library/catalog/scte-35-2-digital-program-insertion-cueing-message-part-2-event-based-signaling/)

FFmpeg recognizes SCTE-35 as an MPEG-TS data-stream codec, and `ffprobe` can select a stream and emit packet PTS, duration, and base64 payload data as JSON. That permits a small local Go provider without a new daemon or model: strictly bound packet count and payload length, validate section/descriptor lengths and CRC before publication, allowlist known event semantics, pair start/end or use a bounded declared duration, and reject unknown, conflicting, unclosed, or out-of-runtime events without side effects. Cues are often removed during remuxing, so absence proves nothing. [FFmpeg MPEG-TS SCTE-35 mapping](https://ffmpeg.org/doxygen/trunk/mpegts_8c.html), [`ffprobe` packet/data output](https://ffmpeg.org/ffprobe.html)

Comskip remains the most deployable offline baseline. It segments recordings using black frames, silence, aspect-ratio changes, scene changes, logo presence, closed captions, and other measurements, then applies configurable heuristics. Its manual supports post-recording, in-progress, and cached analysis. [Comskip manual](https://www.kaashoek.com/files/manual.htm), [official source](https://github.com/erikkaashoek/Comskip)

A 2023 WACV system shows a stronger learned architecture: lightweight audio proposes boundaries globally, then an audio-visual classifier and post-processing decide which segments are ads. On a private test set of 32 movies and 16 live sports streams containing 621 ads, its variants reported over 96% correct detections, under 1% over/under-segmentation and misses, and under 2.5% false positives; its audio segmentation took 87 seconds for a 60-minute movie versus 839 seconds for the paper's color-based baseline. Because the corpus and trained model are private and represent streaming-inserted ads rather than arbitrary regional DVR broadcasts, those numbers are not a redistributable solution. They do support adopting the **cheap proposal → expensive classification → temporal cleanup** pattern. [Liu et al., WACV 2023](https://cdn.amazon.science/3f/1a/d55611204b2791cb2ef97e3f98b5/a-deep-neural-framework-to-detect-individual-advertisement-ad-from-videos.pdf)

A 2025 IEEE Latin America Transactions paper focuses instead on channel bumpers: sparse-frame difference hashes propose known bumpers, OCR distinguishes start/end language, and scene detection proposes unknown 1.3-5-second bumpers. It reports perfect offline precision/recall on 114 hours and 264 annotated bumpers within a loose plus-or-minus four-second boundary, but its 15-day live test detected only 258 breaks against 337 estimated; reception errors and animated bumpers caused misses. This is valuable only when a broadcaster reliably brackets breaks with repeated regulatory assets. Kinosail should clean-room the simple template idea per tuner/channel and fuse matches with Comskip or learned interval evidence; the published repository is CC BY-NC-ND 4.0 plus an all-rights-reserved notice and is unsuitable for incorporation or modification. [Rondan et al., 2025](https://latamt.ieeer9.org/index.php/transactions/article/view/9985), [official implementation and license](https://github.com/nicolasrondan/tv_commercial_break_detector/blob/main/licence.md)

Two other 2025 commercial papers report 99.79% accuracy from conventional acoustic features and 93% F1 from ASR plus RoBERTa, respectively. Their public primary records omit the data/split, temporal-boundary tolerance, full feature/runtime recipe, and reusable checkpoints. They support adding bounded acoustic and transcript evidence, not selecting a reproducible detector or authorizing automatic skips. [audio feature/ML paper](https://ieeexplore.ieee.org/document/10929855), [speech-to-text/transformer paper](https://ieeexplore.ieee.org/document/11275469)

Plex warns that commercial detection is CPU intensive and imperfect, defaults to non-destructive “mark for skip,” exposes `comskip.ini` for tuning, and gives 2–4 minutes as a typical analysis time for a 30-minute recording on a reasonably fast CPU. [Plex commercial detection](https://support.plex.tv/articles/115003944134-removing-commercials/)

Kinosail should not apply broadcast-ad heuristics to ordinary ripped TV libraries. A hard cut, repeated shot, black frame, or absent channel logo can be story content.

## Practical method comparison

| Method | Best fit | Accuracy boundary | Compute/storage | Privacy/offline | License/deployment boundary |
| --- | --- | --- | --- | --- | --- |
| Manual/chapter/sidecar | All types | Highest precision when authored correctly; absent or mislabeled otherwise | Existing probe; negligible | Fully local | No new runtime |
| Chromaprint recurrence | Repeated intro/outro/theme | Strong for near-identical audio; misses changed/silent variants | Bounded mono decode; raw `uint32` cache is about 20 KB per 10-minute window at Kinosail's current cadence | Fully local | Upstream says combined work is LGPL-2.1; binary distribution needs review. [license](https://github.com/acoustid/chromaprint/blob/master/LICENSE.md) |
| MPEG-7 video signature / owner ident template | Repeated visual intro/outro and Movie studio/distributor idents | Strong for copied sequences; not semantic and may miss heavily customized idents | Existing FFmpeg; bounded opening/tail decode and compact signature files | Fully local | No new runtime/model; FFmpeg build license already applies. Match output needs a version-pinned fail-closed adapter |
| Sparse visual recurrence | Silent/re-scored intros/outros; prior-Episode recap evidence | Handles visual copies; can match repeated story shots or overlays | 1 FPS bounded decode; 384-d float16 embeddings are about 0.46 MB per 10 minutes before indexes | Fully local | DINOv2 code/ordinary weights are Apache-2.0, but model version, checksum, ONNX export, and notices must be pinned. [official repository](https://github.com/facebookresearch/dinov2) |
| FFmpeg + text/temporal credits | Ending/opening credits | Broad styles need cue fusion; black-only is unsafe | Bounded tail scan plus local boundary refinement | Fully local | FFmpeg license depends on build configuration; OCR is optional; Tesseract and PaddleOCR are Apache-2.0 |
| SCTE-35 typed cues | DVR/FAST/broadcast commercials | Highest source evidence when preserved and semantically specific; generic placement opportunities and missing end cues need corroboration | Packet scan only; negligible marker storage | Fully local | Standard parser only; no new runtime. Strict untrusted-binary parsing required |
| Comskip | Finalized DVR commercials | Mature/tunable but region, channel, and content dependent | Full decode; small EDL/evidence cache | Fully local | GPL-2.0 executable; image/distribution review required. [license](https://github.com/erikkaashoek/Comskip/blob/master/LICENSE) |
| 2025 CLIP-attention model shape | Variable intros/credits | Promising 91% per-second F1 on private 27-hour dataset; combined label and overlaid-credit failures | Paper reports 545 MB FP32/290 MB FP16; 1 FPS frame decode excluded from speed | Can infer locally | No published checkpoint/corpus/license for the task model; do not ship from paper alone |
| Multimodal recap model | Recap and variable sequences | Best task-specific published evidence still depends on a large private labeled set | Model/runtime plus bounded audio/video/subtitle features | Can infer locally | Requires a redistributable checkpoint and licensed representative data |
| Shared marker database | Exact media revision hits | Fast on hits; trust, poisoning, collision, and coverage risks | Low local compute | Leaks a derived media identifier unless explicitly opted in | Hosted future option, never required |

## Recommended Kinosail algorithm

### 1. Persist evidence, candidates, and decisions separately

Do not persist only the winning range. Preserve enough provenance to reproduce and recalibrate it:

```text
item ID, media revision, stream IDs,
candidate type/start/end,
evidence vector and supporting item IDs,
detector/model/config versions,
raw score, calibrated risk tier,
source, created time, owner disposition
```

Owner/manual markers outrank chapters; chapters outrank generated markers; no background run may overwrite either. Cache reproducible fingerprints/embeddings/statistics under `/cache`; persist accepted markers, overrides, provenance, and model/config versions under `/config`. Invalidate generated evidence when content revision, selected stream, detector version, or feature schema changes.

The current store persists `Revision`, final markers, and owner-suppressed types, but it still cannot distinguish a model/config change from a content change or recalibrate without decoding again. [current marker store](../../internal/server/marker_store.go)

### 2. Generate cheap, independent evidence

Run outside the play request and in this order:

1. Normalize chapter titles and sidecars using Unicode case-folding, punctuation/whitespace normalization, localized phrase dictionaries, and explicit negative names. On DVR/transport sources, parse strictly validated SCTE-35 before content analysis.
2. Extract bounded mono audio fingerprints from opening/tail windows. Use all preferred-language/program audio tracks where practical, never a commentary/descriptive track as the only source.
3. Sample bounded visual windows at 1 FPS. Benchmark the already bundled MPEG-7 `signature` filter and cheap perceptual hashes to propose matching diagonals; compute a compact embedding only for ambiguous candidates.
4. Extract transition evidence once: shot boundaries, black/silence, luma entropy, saturation, text-box density/persistence/motion, logo presence, and subtitle/ASR phrase hits.
5. For DVR media only, run Comskip and import its interval scores/output as another evidence provider.

The current analyzer has chapter/manual precedence, detector-versioned records and visual caches, cached bounded Chromaprint, 1 FPS difference-hash visual recurrence, and a meaningful false-positive gate: each Episode needs two agreeing pair matches (therefore at least three Episodes), and the published range is the conservative intersection of overlapping candidates. Large groups use sparse audio/visual anchor indexes before exact alignment rather than comparing every pair. Movie studio-ident detection requires independent audio and visual consensus across at least three unrelated Movies for auto-skip; single-modality and visual-only recurrence stay prompt-only. Credits now require sustained text-like edge structure on a mostly uniform light or dark background, retain the tail anchor and separated ranges, and preserve the last good revision after malformed extraction. It still uses only the first audio stream; bright, animated, and live-action-overlay credits need stronger temporal/OCR evidence; confidence evidence is represented only coarsely by marker source; and there is no generated recap, SCTE-35, or commercial-analysis lane. [current analyzer](../../internal/server/marker_analyzer.go), [matching and consensus](../../internal/server/marker_matching.go), [credits and tail anchor](../../internal/server/credits_analyzer.go)

### 3. Align recurrence without quadratic season work

For intro/outro recurrence:

1. Build compact audio/visual anchor indexes for the season.
2. Retrieve the top `k` sibling candidates by anchor overlap rather than comparing every pair.
3. Form audio and visual similarity matrices for each candidate pair.
4. Use monotonic dynamic programming/local alignment to find contiguous diagonals and allow small time-scale/encode drift.
5. Cluster aligned intervals across Episodes. Retain the current conservative shared intersection for auto-skip; also estimate median/MAD boundaries for diagnostics and prompt-only recovery when the intersection is shortened by one noisy Episode.
6. Require support from multiple distinct Episodes and at least one non-positional modality. Position/duration are priors, never sufficient evidence.

This is the practical adaptation of modern copy-localization research. On a small season, exact pairwise comparison remains acceptable; the index prevents an all-library quadratic path and makes specials/large seasons predictable.

### 4. Build type-specific candidates

#### Intro

- Search the first `min(25% of runtime, 10 minutes)`.
- Accept a named chapter immediately unless an owner override exists.
- Otherwise require an aligned 10–180 second audio or visual run supported by multiple siblings. Do not freeze Plex's 20–120 second product thresholds into the domain model; learn accepted duration priors by content class and keep wide safety bounds.
- Reject/abstain when the same run also occurs deep in Episodes, when sibling boundaries disagree strongly, or when evidence comes only from time position.
- Refine the coarse start/end within ±10 seconds using the nearest supported shot, audio, black, chapter, or text-state transition.
- For a Movie studio/distributor ident, search the opening window against owner-approved reusable audio/video templates or a cross-library cluster supported by several unrelated Movies. Publish only the matched template interval, never a guessed “content start.” Require both modalities for auto-skip when both exist; a single-modality/customized-logo match stays prompt-only.

#### Outro

- Search the final `min(25% of runtime, 15 minutes)` with the same recurrence machinery.
- Require the run to end near EOF, meet a detected credit range, or recur at a consistent tail-relative position.
- Keep previews/outros distinct from credits; do not create an outro for a Movie or singleton Episode from position alone.

#### Credits

- Search both the opening window and final `min(25% of runtime, 20 minutes)`; put a much stronger prior on the tail.
- Propose runs when text boxes persist across frames, text density is sustained, or text moves coherently upward. Add low scene-change rate, black/white/muted backgrounds, entropy/saturation, and music/speech state as supporting features.
- Require text evidence or a validated semantic-model score for auto-skip. Black/silence alone may only propose a boundary.
- Decode candidate boundaries at 4–8 FPS (or native frames in a final ±2-second pass) and snap to the highest-scoring transition.
- Split when ordinary-content evidence persists, producing separate credit ranges around mid-/post-credit scenes. Mark only the final range as terminal.

#### Commercial

- Restrict to finalized DVR/broadcast media and preserve the source.
- Start with strictly parsed, semantically specific, paired SCTE-35 advertisement/break events when the source preserves them. Treat a generic splice/placement opportunity, missing pair, invalid CRC/length, overlap, or out-of-runtime timestamp as non-authoritative.
- Otherwise use Comskip EDL intervals and its broadcaster/library profile.
- Fuse interval duration, black/silence boundaries, logo absence/return, shot rate, aspect/caption changes, loudness/speech/music, and repeated-ad matches.
- Use neighboring context: a candidate should look like an ad **and** sit inside a plausible break. Keep per-channel/region calibration separate.
- If Kinosail later trains a model, follow the 2023 two-stage pattern: lightweight audio boundary proposals, expensive audio-visual classification only on proposed segments, then duration/context cleanup.

#### Recap

- Accept an authoritative recap chapter.
- Otherwise use subtitle text first; optionally run local ASR only on the bounded opening window. `whisper.cpp` provides a dependency-light MIT-licensed native runtime for this one-container shape, while the OpenAI Whisper code and weights are also MIT-licensed; adding pinned model weights remains a separate image-size and supply-chain decision. [whisper.cpp official repository](https://github.com/ggml-org/whisper.cpp), [Whisper official repository](https://github.com/openai/whisper)
- A localized “previously on” phrase only seeds the start. Estimate the end from a sustained opening montage with dense visual/audio matches to several earlier Episodes, followed by a shot/audio/text state transition or an accepted intro start.
- Feed phrase, prior-Episode copy density, shot rate, speech/music, title-text density, and position to a trained temporal `recap | intro | content` decoder.
- Until a representative model and corpus meet the false-story-seconds gate, recap candidates remain prompt/manual only.

### 5. Fuse evidence and decode a legal timeline

Use a small calibrated model per segment type, not one opaque global score. Gradient-boosted trees or logistic regression over explicit interval features are sufficient for the first learned fusion stage and are easier to audit than an end-to-end video model. A later ONNX sequence encoder may add a semantic score, but it remains one feature provider.

Run a semi-Markov/Viterbi decoder over candidate intervals with soft duration/position/order priors. It should reward plausible structures such as `recap → intro → content`, repeated `content ↔ commercial`, and `content → outro → credits`, while still allowing missing labels, opening credits, and multiple credit ranges. Hard rules should be limited to impossibilities: invalid times, whole-title removal, manual-marker conflicts, and overlapping incompatible labels.

Final boundary selection should use a shared boundary lattice from chapters, shot changes, black/silence edges, recurrence ends, text-state changes, and semantic-model transitions. This avoids five detectors producing inconsistent near-duplicate timestamps.

### 6. Calibrate to safe auto-skip, prompt, or abstain

Split the evaluation corpus by Show/franchise and source, never randomly by Episode, to prevent repeated titles from leaking across train/calibration/test. Calibrate separately by type and detector version.

Use asymmetric loss:

```text
false skipped story seconds  >>  missed skippable seconds
bad end boundary             >   bad start boundary
recap/commercial false skip  >   intro/credit false skip
```

Conformal risk control can tune a threshold for a chosen bounded loss with finite-sample guarantees under exchangeability, but that guarantee does not survive a genre/region/source distribution shift. Use it only after Kinosail owns an adequate calibration set; otherwise report empirical risk and abstain aggressively. [Angelopoulos et al., ICLR 2024](https://proceedings.iclr.cc/paper_files/paper/2024/hash/f3549ef9b5ff520a7e41ff3cc306ab2b-Abstract-Conference.html)

Expose three decisions:

- `auto`: authoritative or calibrated high-confidence candidate;
- `prompt`: plausible candidate below the auto threshold;
- `hidden`: insufficient/conflicting evidence.

Store the decision reason and calibrated detector version. Owner edits become labeled evaluation data after explicit opt-in; never silently train on them or upload them.

## Background execution, resource controls, and privacy

```text
library refresh
  -> identify new/changed media revisions
  -> durable, deduplicated analysis jobs
  -> one low-priority worker under transcode admission
       -> authoritative metadata
       -> cached bounded audio/visual evidence
       -> recurrence index/alignment
       -> type-specific candidates
       -> calibrated fusion + temporal decode
       -> atomic marker revision
  -> API/web/Jellyfin adapters read one accepted marker operation
```

Operational requirements:

- Never block browse or playback on analysis; interactive transcodes preempt analysis.
- Cache by content/stream revision + extractor/model/config version. Threshold changes should rerun fusion, not decoding.
- On a new Episode, extract only its features and query the existing season index; schedule a delayed consensus refresh.
- Bound child-process CPU, memory, wall time, output, and concurrency. Kill the process group on cancellation/shutdown.
- Publish complete revisions atomically and retain the last good markers after failure.
- Keep paths, titles, fingerprints, embeddings, transcripts, and frame statistics out of telemetry. Default to zero network egress.
- Surface `queued`, `running`, `complete`, `unsupported`, `failed`, detector version, and stable reason codes through `/api/v1`; the web adapter calls the same application operation.

Plex and Jellyfin place expensive analysis in scheduled/background work rather than playback requests. [Plex Library settings](https://support.plex.tv/articles/200289526-library/), [Jellyfin media-segment scan](https://jellyfin.org/docs/general/server/metadata/media-segments/)

This remains one Kinosail Server container. The Go process owns jobs/state and strict SCTE-35 parsing; FFmpeg, ffprobe, fpcalc, optional OCR/Comskip, and an optional pinned ONNX runtime are bounded in-container dependencies, not sidecars or network services.

## Licensing and supply-chain gates

- Pin every executable, model, and vocabulary by version and checksum; include notices and an SBOM; verify signatures where upstream supplies them.
- Chromaprint says the combined project should be treated as LGPL-2.1 and warns binary distributors to account for the external FFT library. [Chromaprint license](https://github.com/acoustid/chromaprint/blob/master/LICENSE.md)
- Comskip ships GPL-2.0 source. Bundling/invocation and redistribution need a deliberate review before it enters the runtime image. [Comskip license](https://github.com/erikkaashoek/Comskip/blob/master/LICENSE)
- DINOv2's ordinary code/weights are Apache-2.0, but similarly named specialized model families in the repository may have different terms; pin the exact artifact and its license. [DINOv2 repository](https://github.com/facebookresearch/dinov2)
- DINOv3 is not an Apache-2.0 drop-in replacement: both code and weights use Meta's custom DINOv3 license and gated access. It has no measured Kinosail advantage, so do not add it by version number alone. [DINOv3 license](https://github.com/facebookresearch/dinov3/blob/main/LICENSE.md)
- Tesseract and PaddleOCR are Apache-2.0, but PaddleOCR model files, native libraries, size, and notices still need exact artifact pinning and image review. [Tesseract repository](https://github.com/tesseract-ocr/tesseract), [PaddleOCR repository](https://github.com/PaddlePaddle/PaddleOCR)
- Whisper code and weights are MIT-licensed; model size and multilingual accuracy still require product evaluation. [Whisper repository](https://github.com/openai/whisper)
- Do not ship the 2025 CLIP-attention task model: the paper describes weights but supplies no redistributable checkpoint/data license. [Korolkov and Yanchenko](https://arxiv.org/pdf/2504.09738)
- Do not copy GPL Intro Skipper implementation code into Kinosail. Reimplement only the independently researched algorithmic ideas behind Kinosail's own interfaces.

## Acceptance evidence

### Deterministic and integration tests

- Unicode/localized chapter normalization, false-positive names, precedence, overlapping ranges, invalid timestamps, and manual override protection.
- Audio/visual alignment under offset, codec, loudness, crop, resize, overlay, frame-rate, and small speed changes; short/unrelated/shared-story rejection.
- Movie studio-ident templates across identical, shortened, re-scored, custom, and false-similar logos; cold-open and opening-credit negatives; exact matched-interval enforcement.
- Season clustering, outlier Episodes, two-Episode abstention, multi-intro variants, specials, reordered Episodes, and incremental index invalidation.
- Credits on black, white, muted, bright, animated, and live-action backgrounds; horizontal/vertical/static text; dark-scene/fade/subtitle negatives; HDR normalization; and mid-/post-credit splitting.
- SCTE-35 advertisement/break pairs, generic placement opportunities, unknown events, invalid CRC/length, missing ends, declared-duration bounds, overlapping/out-of-runtime cues, and cue-free remuxes.
- Comskip import and learned commercial candidate parsing, regional duration profiles, malformed/overlapping intervals, cancellation/failure, and source immutability.
- Recap phrase localization, subtitle/ASR disagreement, flashback negatives, prior-Episode copy density, and intro/recap adjacency.
- Queue durability/deduplication, resource preemption, stale-version reanalysis, atomic publication, and playback never waiting on analysis.
- Identical marker output through the Kinosail API, web adapter, and Jellyfin tick/type mapping.

### Corpus and release gates

Build an owner-controlled, legally usable ground-truth corpus and report per type and source class:

- segment-existence precision/recall/F1 at natural prevalence;
- start/end error distribution and recall within 1, 2, 3, and 5 seconds;
- false skipped story seconds per media hour and worst-item error;
- coverage at the `auto` and `prompt` tiers plus risk-versus-coverage curves;
- performance by genre, language, source, decade, runtime, and recurring/non-recurring style;
- CPU/GPU time, decoded seconds, peak memory, model bytes, cache bytes, and cold/incremental wall time;
- calibration drift after a detector/version/source change.

Include cold opens, short/long/variable/anime openings, re-scored and silent intros, multilingual and commentary audio, bright/animated/live-action credits, mid-/post-credit scenes, regional broadcasts, ad-free negatives, flashbacks, dark scenes, subtitles, watermarks, HDR, and replaced files.

Do not claim “best” from paper metrics or unit fixtures. Ship automatic action per detector/type only when a held-out Kinosail corpus meets an explicit false-story-seconds limit with a lower confidence bound; otherwise ship prompt-only.

## Delivery phases

### Phase 1 — harden the lean local cascade

- Extend the implemented detector version and source gate with explicit evidence, confidence tier, and owner disposition.
- Keep authoritative chapter/manual precedence.
- Add the bounded SCTE-35 evidence provider for finalized broadcast sources, with strict parser rejection and no side effects on malformed input.
- Preserve the implemented three-title consensus, conservative intersection, and bounded candidate index; add explicit evidence/confidence records and an incremental persistent index when measurements justify it.
- Benchmark the already bundled MPEG-7 `signature` filter as the first visual recurrence/owner-ident lane; keep it prompt-only until the Kinosail corpus establishes safe boundaries.
- Extend the implemented text-structure, tail-anchored, multi-range credit proposals with motion, OCR, and boundary refinement.
- Add a shared boundary lattice, calibrated `auto | prompt | hidden` decisions, and the evaluation harness.

### Phase 2 — visual recurrence and DVR commercials

- Add sparse perceptual-hash proposals, then a pinned compact visual embedding only for ambiguity; cache and align diagonals.
- Complete image-size, security, SBOM, and GPL review for Comskip; import finalized-recording EDL output without rewriting media.
- Benchmark a two-stage audio-boundary/audio-visual commercial classifier against Comskip before considering replacement.

### Phase 3 — semantic recap/variable-sequence model

- Acquire/create licensed, representative labels and a redistributable checkpoint.
- Add bounded subtitle/ASR and audio-visual temporal scores through a pinned offline model runtime.
- Calibrate by type/source and keep recap/commercial prompt-only until false-story-seconds risk is demonstrated.
- Consider exact-file marker sharing only as explicit privacy opt-in with signed data, collision resistance, poisoning defenses, and a local-only default.

## Bottom line

The recommended algorithm is a **typed late-fusion cascade with temporal alignment, a semi-Markov timeline decoder, and calibrated abstention**:

```text
authoritative metadata
  -> bounded audio + sparse visual recurrence
  -> type-specific text/audio/visual candidate evidence
  -> shared boundary refinement
  -> per-type calibrated fusion
  -> legal timeline decode
  -> auto, prompt, or hidden
```

That design incorporates authoritative SCTE-35 signaling where a broadcast source preserves it, the practical strength of Chromaprint/Comskip, the temporal-alignment lesson from 2023-2024 video-copy research, the visual-semantic lesson from the 2025/2026 CLIP-attention work, and the safety discipline required for automatic skipping. Kinosail now has the conservative recurrence shape—three-title agreement plus interval intersection—an audio-and-visual Movie-ident gate, versioned visual evidence, and text-structured tail credit proposals. The highest-value next corrections are source-aware SCTE-35 parsing, explicit confidence/evidence records, owner-approved ident templates plus a pinned MPEG-7 experiment, and stronger motion/OCR credit fusion before adding a heavyweight semantic model.
