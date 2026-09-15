# Subtitle discovery, synchronization, and cleanup research

Date: 2026-08-30

## Research question

What pipeline can find, synchronize, clean, validate, and upgrade subtitles without routine owner action?

The target is a private, self-hosted Movies and TV library. The system must prefer no subtitle over a wrong subtitle.

## Decision

Use a staged pipeline with explicit abstention. Do not use one provider score or one model as the final decision.

1. Inspect local sidecars and embedded tracks.
2. Preserve forced, standard, and subtitles for the deaf and hard of hearing as separate roles.
3. Search documented provider application programming interfaces with stable media identifiers.
4. Reject identity conflicts before ranking.
5. Rank release fit and subtitle quality separately.
6. Parse every download in a bounded temporary area.
7. Confirm weak release matches against local audio.
8. Retiming starts with a global offset and frame-rate correction.
9. Piecewise alignment runs only when a global correction cannot explain the file.
10. Optical character recognition and speech recognition are local fallback operations.
11. Cleanup changes structure and encoding. It does not rewrite dialogue.
12. A candidate replaces a file only when stronger evidence proves an upgrade.

This design is more reliable than a large score that mixes title identity, timing, and text quality.

## Important terms

- ASR means automatic speech recognition.
- OCR means optical character recognition.
- VAD means voice activity detection.
- SDH means subtitles for the deaf and hard of hearing.
- CPS means characters per second.
- CPL means characters per line.
- NFC means Unicode Normalization Form C.

## Source order

| Order | Source | Main value | Default decision |
| --- | --- | --- | --- |
| 1 | Existing owner sidecar | Owner intent | Keep unless the owner requests management. |
| 2 | Embedded text track | Exact media edition and no network use | Extract when language and role match. |
| 3 | Embedded image track | Exact media edition | OCR locally when a text track is required. |
| 4 | SubDL | Stable title identifiers and release-aware search | Use first among free network providers. |
| 5 | OpenSubtitles | File hash, trusted-source, role, and translation filters | Use for exact hash proof and broader recovery. |
| 6 | Local ASR | No provider dependency | Generate only after all human sources fail. |
| 7 | Burned-in subtitle OCR | Can recover hardcoded text | Keep disabled by default. It has a high false-positive cost. |

SubDL currently exposes title, IMDb, TMDB, filename, season, episode, language, and SDH filters. Its filename search returns a release `match_score`. The service calls 0.8 a confident match. The free plan currently lists 2,000 searches and 50 downloads each day. Every response includes rate-limit headers. [SubDL API](https://subdl.com/developers)

OpenSubtitles exposes movie hash, byte size, title IDs, Episode fields, language, trusted source, foreign-only, hearing-impaired, and machine-translation filters. Its official Model Context Protocol server documents these current search fields. [OpenSubtitles tools](https://github.com/opensubtitles/mcp.opensubtitles.com)

Do not scrape provider web pages. Add a provider only when it has a documented automation interface and acceptable terms.

## Media identity and candidate discovery

### Normalize library facts once

Store these facts before any provider search:

- media kind;
- canonical title and alternate titles;
- year;
- season and episode set;
- IMDb and TMDB identifiers;
- edition or cut;
- release filename tokens;
- source, resolution, video codec, audio codec, release group, and frame rate;
- media size and OpenSubtitles hash;
- audio languages;
- embedded subtitle languages and roles.

Use BCP 47 as the internal language form. Map provider-specific codes only at provider boundaries. BCP 47 defines language tags, and the IANA registry supplies current subtags. [RFC 5646](https://www.rfc-editor.org/info/rfc5646/) [IANA registry](https://www.iana.org/assignments/language-subtags-tags-extensions)

Keep standard, forced, SDH, commentary, and text-description tracks distinct. Matroska defines forced and hearing-impaired flags with separate selection behavior. [RFC 9559](https://www.rfc-editor.org/rfc/rfc9559.html)

### Apply hard identity gates

Reject a candidate when any known value conflicts with the library fact:

- wrong language or script;
- wrong Movie or Series;
- wrong season or episode;
- wrong year when both values are reliable;
- forced-only when a full subtitle is wanted;
- commentary when ordinary dialogue is wanted;
- incompatible edition evidence;
- provider response fields that disagree with each other.

Missing evidence is not a conflict. It lowers confidence.

### Treat the OpenSubtitles hash as strong, not absolute

The OpenSubtitles hash adds the file size to 64-bit words from the first and last 64 KiB. It identifies media releases quickly. It is not a cryptographic content hash. [OSHash specification and implementations](https://github.com/opensubtitles/oshash)

Bazarr also validates a reported hash with supporting media facts. Its current score code requires Episode identity and source facts, or Movie codec and source facts. [Bazarr score code](https://github.com/morpheus65535/bazarr/blob/master/custom_libs/subliminal_patch/score.py)

Use an exact hash only when byte size matches and no identity fact conflicts. Compute SHA-256 separately for local change detection.

### Rank with a quality vector

Do not let many weak signals defeat one hard conflict. Rank accepted candidates in this order:

1. identity proof tier;
2. requested language and role;
3. human source before machine translation, OCR, or ASR;
4. exact release hash or release filename fit;
5. audio synchronization confidence;
6. trusted uploader and provider quality facts;
7. structural quality;
8. provider rating and download count;
9. newest upload only as a final tie-breaker.

Use these identity tiers:

- Tier A: matching media hash and size, with no metadata conflict.
- Tier B: matching provider title ID, Episode facts, and release score of at least 0.80.
- Tier C: matching provider title ID and Episode facts, but weak release evidence.
- Tier D: title text only. Never install this tier without strong audio proof.

Run local language identification on enough dialogue text. Restrict the classifier to plausible languages. Reject low-confidence results. FastText publishes an offline 176-language model, but its model uses a CC BY-SA 3.0 license. [fastText language identification](https://fasttext.cc/docs/en/language-identification)

## Synchronization

### Diagnose before modification

Measure the original file first. Record:

- cue activity against speech activity;
- offset consistency across the program;
- likely frame-rate ratio;
- unexplained timeline breaks;
- cue coverage near the start, middle, and end;
- baseline and proposed alignment scores.

Do not retime an exact local or embedded track unless measurements show a defect.

### Use a staged synchronizer

| Stage | Method | Best case | Failure control |
| --- | --- | --- | --- |
| 1 | Global VAD cross-correlation | Constant offset | Require improvement across sampled windows. |
| 2 | Frame-rate search | 23.976, 24, 25, or 29.97 conversion | Bound the scale and test common ratios first. |
| 3 | Piecewise VAD alignment | Deleted ads, recaps, or edition breaks | Penalize every split and limit split count. |
| 4 | Text-to-audio forced alignment | Same-language text with weak timing | Require supported language and word coverage. |
| 5 | Synced-subtitle reference | Translated text with a good sibling track | Preserve monotonic cue order. |

FFsubsync converts audio speech and subtitle activity into 10 ms signals. It uses Fast Fourier Transform cross-correlation for a global offset. It also tests frame-rate corrections. Current options include sampled multi-segment synchronization, fused WebRTC and Silero VAD, piecewise alignment, and low-quality abstention. [FFsubsync](https://github.com/smacke/ffsubsync)

Do not compare raw FFsubsync scores across Movies. Its maintainer states that the score depends on duration and speech proportion. [FFsubsync score explanation](https://github.com/smacke/ffsubsync/discussions/148)

ALASS uses a language-independent dynamic program. It handles offsets, frame-rate changes, and internal edition breaks. The project reports 88% to 98% good files on its test corpus. Treat that result as project evidence, not a Kinosail guarantee. [ALASS](https://github.com/kaegi/alass)

Use VAD alignment before ASR. VAD is language-independent and does not alter text. Use ASR alignment when speech activity is insufficient or same-language words can add strong anchors.

WhisperX combines VAD with phoneme forced alignment. Its paper reports better word timing and fewer long-form drift problems than raw Whisper timestamps. [WhisperX paper](https://arxiv.org/abs/2303.00747) [WhisperX implementation](https://github.com/m-bain/whisperX)

Qwen3-ForcedAligner-0.6B is a 2026 Apache-2.0 model. It aligns text and speech in 11 languages. It is a useful experiment for supported languages, but it needs a Kinosail corpus comparison before default use. [Qwen3-ASR report](https://arxiv.org/abs/2601.21337) [official implementation](https://github.com/QwenLM/Qwen3-ASR)

### Accept a synchronization only with proof

Require all conditions:

- no hard identity conflict;
- better speech overlap than the original;
- consistent improvement in several program regions;
- plausible offset and scale;
- monotonic cues after transformation;
- no cue outside the media duration allowance;
- no large new overlap or unreadable duration rate;
- a recovery copy or reproducible source.

If the evidence is weak, keep the candidate unmodified or try the next candidate.

## OCR and ASR fallback

### Embedded image subtitles

Use the image track timestamps. OCR only the images. Collapse repeated images before recognition. Merge consecutive equal OCR results and use the text variant with the strongest duration and confidence.

Subtitle Edit uses this pattern for burned-in video OCR. It collapses near-identical frames and merges repeated text into one timed cue. [Subtitle Edit video OCR](https://github.com/SubtitleEdit/subtitleedit/blob/main/docs/features/video-ocr.md)

PaddleOCR PP-OCRv5 supplies language-specific mobile models for 106 languages. The project reports more than 30% multilingual recognition improvement over PP-OCRv3 on its evaluation. Kinosail must test subtitle images because the published task is general OCR. [PP-OCRv5 multilingual documentation](https://github.com/PaddlePaddle/PaddleOCR/blob/main/docs/version3.x/algorithm/PP-OCRv5/PP-OCRv5_multi_languages.en.md) [PaddleOCR 3.0 report](https://arxiv.org/abs/2507.05595)

Use the requested language model. Keep per-line OCR confidence. Reject lines with weak confidence instead of guessing punctuation or names.

### Local speech recognition

Use ASR only when no valid human subtitle exists. Mark the result as generated. Never replace a human subtitle with ASR based only on newer model output.

OpenAI Whisper `turbo` is an optimized `large-v3` model. The official repository lists about 6 GB of video memory and a large speed improvement. The code and weights use the MIT license. Accuracy varies by language and hardware. [Whisper models](https://github.com/openai/whisper)

Whisper can hallucinate absent text, repeat text, and perform unevenly across languages. OpenAI requires domain evaluation before deployment. [Whisper model card](https://github.com/openai/whisper/blob/main/model-card.md)

Use local VAD before ASR. Reject speech output when language, token probability, repetition, silence, or duration checks fail. Forced-align the accepted transcript before cue segmentation.

Recent IWSLT systems use VAD, long-form ASR, forced alignment, translation, and a separate compliance pass. This supports a staged design. It does not justify automatic dialogue rewriting. [IWSLT 2026 HW-TSC](https://aclanthology.org/2026.iwslt-1.10/) [IWSLT 2026 FBK](https://aclanthology.org/2026.iwslt-1.7/)

## Cleanup and normalization

### Safe automatic changes

Apply these changes to a generated copy:

- decode once into Unicode;
- write UTF-8;
- normalize text to NFC;
- normalize line endings;
- trim stray horizontal and line-end whitespace;
- remove empty cues;
- sort by start time when order is unambiguous;
- renumber SRT cues;
- remove exact duplicate cues;
- merge consecutive identical text with a small gap;
- balance supported emphasis tags;
- reject invalid timestamps and impossible cue counts;
- preserve meaningful punctuation, case, line breaks, and music marks.

The WHATWG Encoding Standard says new formats should use UTF-8 decode and UTF-8 encode. [Encoding Standard](https://encoding.spec.whatwg.org/)

Unicode recommends NFC for general text. NFC preserves canonical meaning, unlike compatibility normalization. [Unicode Normalization](https://www.unicode.org/reports/tr15/) [Unicode FAQ](https://www.unicode.org/faq/normalization.html)

WebVTT requires ordered start times and an end time after the start time. [WebVTT](https://www.w3.org/TR/webvtt1/)

Use IMSC Text Profile 1.3 as the strict interchange reference when rich timed text is required. It became a W3C Recommendation in May 2026. [IMSC 1.3](https://www.w3.org/TR/ttml-imsc1.3/)

### Conditional changes

Only apply these changes when the language and format are known:

- split a long line at a grammatical boundary;
- join a short continuation;
- open a minimum cue gap;
- remove known provider credit lines at the file edge;
- convert safe styling between formats;
- repair common OCR substitutions with a language dictionary.

Subtitle Edit separates generic, language-gated, timing, duplicate, and OCR repair rules. It also reports skipped rules. This is a good model for auditable cleanup. [Subtitle Edit command line](https://github.com/SubtitleEdit/subtitleedit/blob/main/docs/reference/command-line.md)

### Do not change automatically

Do not change:

- dialogue wording;
- spelling of names;
- profanity;
- translation meaning;
- song lyrics;
- speaker identity;
- forced-narrative scope;
- intentional repetitions;
- SDH meaning;
- ASS positioning or karaoke effects without a lossless target.

### SDH policy

Prefer provider role metadata over text heuristics. Keep SDH when the owner prefers accessibility content. Prefer a standard provider candidate when the owner requests standard subtitles.

Do not delete bracketed text from the only subtitle by default. Netflix requires plot-relevant sounds, speaker identifiers, music, and some silence in SDH. [Netflix English SDH guide](https://partnerhelp.netflixstudios.com/hc/en-us/articles/30806198616339-English-UK-Timed-Text-Style-Guide)

If standard and SDH versions both exist, store both with correct role metadata. If conversion is requested, create a derived file and keep the SDH source.

## Readability validation

Use language profiles, not one global rule. The opinionated Latin-script profile is:

- at most 42 CPL;
- at most two lines;
- 20 CPS for adults;
- 17 CPS for children;
- minimum duration near 0.8 seconds;
- maximum duration 7 seconds;
- at least two video frames between adjacent cues when timing permits.

Netflix currently specifies 42 CPL for most Latin languages, two lines, and 20 CPS for adult English. Its timing guide specifies a 20-frame minimum at 24 fps and a two-frame gap. Its template guide sets a 7-second maximum. [Netflix template guide](https://partnerhelp.netflixstudios.com/hc/en-us/articles/219375728-Timed-Text-Style-Guide-Subtitle-Templates) [Netflix English guide](https://partnerhelp.netflixstudios.com/hc/en-us/articles/217350977-English-USA-Timed-Text-Style-Guide) [Netflix timing guide](https://partnerhelp.netflixstudios.com/hc/en-us/articles/360051554394-Timed-Text-Style-Guide-Subtitle-Timing-Guidelines)

Treat readability failures as warnings before text edits. First extend timing into safe gaps. Then re-segment without changing words. Keep difficult cases unchanged.

SubER measures text, segmentation, and timing together. It correlates better with human post-editing effort than text-only metrics in its published evaluation. Use it when a human reference exists. [SubER paper](https://aclanthology.org/2022.iwslt-1.1/) [SubER implementation](https://github.com/apptek/SubER)

## Confidence and upgrade policy

Keep a provenance ledger for each managed subtitle:

- source and provider item ID;
- media identity facts;
- original and output SHA-256;
- source role and generated role;
- candidate evidence vector;
- synchronization transform and measurements;
- cleanup operations;
- model and version when OCR or ASR ran;
- install time and last check;
- user-edit detection state.

Use deterministic proof tiers now. Do not call a raw model or provider score a probability.

If learned ranking is added later, calibrate it on held-out library examples. Evaluate risk against coverage. Selective classification research formalizes the trade between abstention and error. [Selective classification](https://www.jmlr.org/papers/v11/el-yaniv10a.html) Calibration research shows that neural confidence often needs post-processing. [Confidence calibration](https://proceedings.mlr.press/v70/guo17a.html)

Use this replacement policy:

- Never replace an owner-edited file automatically.
- Treat a changed managed fingerprint as an owner edit.
- Replace a managed file only when the new vector dominates all stronger dimensions.
- Also require at least a ten-point gain in the current compatibility score.
- Replace an unknown sidecar only with Tier A identity and a recovery copy.
- Never replace human text with OCR, ASR, or machine translation automatically.
- Never replace standard with SDH, or SDH with standard, as an upgrade.
- Roll back when post-write validation fails.

## Provider limits and scheduling

Use provider-returned quota values. Do not encode assumed OpenSubtitles limits.

Use these SubDL defaults while its published free limit remains 50 downloads each day:

- reserve ten downloads for manual action;
- allow at most 40 automatic downloads each UTC day;
- stop before the provider reports zero remaining;
- update the local ledger before another worker starts;
- honor `Retry-After` and rate-limit reset fields;
- add bounded exponential backoff with jitter;
- cache successful searches for 24 hours;
- cache empty searches for 6 hours;
- deduplicate searches by media revision, language, and role.

Keep provider keys in protected configuration. Do not put them in URLs, logs, telemetry, or subtitle provenance shown to viewers.

## Privacy and security

Run probing, hashing, VAD, OCR, ASR, synchronization, and cleanup locally. Never upload media or audio for routine subtitle work.

SubDL search can send a video filename and parsed Episode facts. Its service logs IP address, user agent, requested URLs, timestamps, and key usage. [SubDL privacy policy](https://subdl.com/privacy)

Send stable title IDs instead of filenames when they provide enough evidence. A movie hash is also a content identifier. Keep provider request details out of ordinary logs.

Treat every provider file and archive as hostile input:

- limit compressed and decompressed bytes;
- limit archive entries and nesting;
- reject absolute paths, traversal, devices, links, and non-subtitle files;
- parse in a private temporary directory;
- use an allowlist of text subtitle formats;
- set cue, line, timestamp, and allocation limits;
- never run scripts or load remote fonts and attachments;
- write through an application-chosen path;
- use an atomic same-directory replacement;
- keep the last recovery file outside subtitle discovery patterns.

OWASP ASVS 5 requires checks for archive entry count, uncompressed size, symlinks, content type, and trusted output paths. [OWASP ASVS 5 file controls](https://github.com/OWASP/ASVS/blob/master/5.0/en/0x14-V5-File-Handling.md) The OWASP upload guide also requires post-decompression limits. [OWASP file guide](https://cheatsheetseries.owasp.org/cheatsheets/File_Upload_Cheat_Sheet.html)

## Evaluation before default activation

Build a private corpus with licensed or owner-supplied media. Include:

- Movies and Episodes;
- 23.976, 24, 25, 29.97, and variable frame rates;
- recap and commercial edits;
- director, extended, and regional cuts;
- multi-Episode files;
- multiple audio languages and dubs;
- standard, forced, SDH, commentary, and bilingual subtitles;
- SRT, WebVTT, ASS, TTML, PGS, and VobSub;
- UTF-8, UTF-16, and common legacy encodings;
- right-to-left and complex scripts;
- low dialogue, music, overlapping speech, and noisy audio;
- correct, offset, drifted, piecewise, wrong-title, and damaged subtitles.

Label identity, role, text source, cue timing, and acceptability. Keep the test set separate from threshold tuning.

Report:

- wrong-title and wrong-Episode install rate;
- correct automatic coverage;
- owner-file replacement count;
- median and 95th-percentile cue boundary error;
- global and piecewise synchronization success;
- OCR character error rate;
- ASR word or character error rate by language;
- SubER when a reference exists;
- CPS, CPL, overlap, and invalid-cue rates;
- provider calls and downloads per installed file;
- CPU time, memory, disk reads, and queue delay;
- recovery and rollback success.

The release gate must contain zero wrong-title installs and zero owner-file loss. Compare every advanced method with the existing metadata-only baseline.

## Recommended implementation sequence

1. Separate identity, role, synchronization, and structural scores.
2. Add strict parsing and normalization with an operation report.
3. Add sampled VAD diagnosis and global retiming.
4. Add frame-rate search and piecewise alignment behind confidence gates.
5. Add per-provider quota headers, caching, and a manual reserve.
6. Add OCR for embedded image subtitles.
7. Add local ASR generation behind a hardware budget.
8. Evaluate WhisperX and Qwen3 forced alignment on the private corpus.
9. Calibrate acceptance thresholds from risk-coverage results.
10. Keep burned-in OCR and automatic text rewriting disabled until separate evidence supports them.

## Final product position

The best hands-off system is conservative and layered. It searches broadly, validates narrowly, and changes files only with proof.

The most useful recent techniques are forced alignment, multi-window VAD, local multilingual OCR, long-form ASR, and timing-aware quality metrics. They complement exact media identity. They do not replace it.
