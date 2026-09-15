# Public v1 subtitle automation and synchronization research

Date: 2026-08-30

## Question

Which current methods support safe, automatic subtitle work in one private server container?

The system must keep media local. It must not require video transcoding. It must prefer no change over a harmful change.

## Decision

Use a conservative local pipeline.

1. Prefer a verified embedded text track or matching sidecar.
2. Search healthy providers with stable media identifiers.
3. Validate the candidate identity, language, role, archive, and text structure.
4. Diagnose timing with local audio analysis.
5. Test a global offset first.
6. Test bounded linear drift next.
7. Test penalized piecewise alignment only when simpler models fail.
8. Apply a change only when several independent checks agree.
9. Keep a recovery copy and a complete operation record.
10. Serve text tracks as UTF-8 SRT or WebVTT. Do not modify the video.

Do not make automatic speech recognition or a large alignment model a public v1 dependency.

## Method comparison

| Method | Handles | Main strength | Main risk | Public v1 role |
| --- | --- | --- | --- | --- |
| Global offset | One fixed delay | Fast, small, and language-independent | Cannot repair drift or internal cuts | Default first attempt |
| Offset plus linear scale | Frame-rate drift | Repairs common 23.976, 24, and 25 frame-rate mismatches | A wrong scale damages the whole file | Default second attempt |
| Piecewise monotonic alignment | Removed recaps, ads, scenes, or disc joins | Repairs a small number of timeline breaks | Too many anchors can overfit speech noise | Bounded fallback |
| Unconstrained dynamic time warping | Continuous local timing changes | Finds a monotonic path through two sequences | Can warp every cue and hide a wrong release | Do not use without strong regularization |
| Voice activity alignment | Same-language and translated subtitles | Local, inexpensive, and text-independent | Music, sparse speech, SDH cues, and commentary weaken the signal | Default acoustic evidence |
| Audio-text forced alignment | Verbatim text in the audio language | Produces word or phoneme anchors | Needs language models and fails on translations or paraphrases | Optional later evidence |
| Automatic speech recognition matching | Poorly timed same-language text | Can find lexical anchors without a reference subtitle | High compute, model storage, transcription errors, and hallucinations | Post-v1 experiment |

FFsubsync converts audio speech and subtitle activity into short windows. It uses Fast Fourier Transform cross-correlation for the global offset. Its current implementation also includes frame-rate search, distributed sampling, fused voice activity detection, piecewise alignment, and low-quality abstention. [FFsubsync design and current options](https://github.com/smacke/ffsubsync) [FFsubsync 2026 history](https://github.com/smacke/ffsubsync/blob/master/HISTORY.rst)

ALASS uses a language-independent dynamic program. It handles fixed offsets, frame-rate changes, and internal edition breaks. Its project reports 88% to 98% good files on its corpus. This result is project evidence, not a Kinosail guarantee. [ALASS implementation and thesis](https://github.com/kaegi/alass)

CTC segmentation is stronger than plain dynamic time warping when a suitable speech model and same-language transcript exist. Its German evaluation also shows that unknown audio and domain mismatch affect accuracy. [Kürzinger et al., 2020](https://arxiv.org/abs/2007.09127)

WhisperX combines voice activity detection with forced alignment. It reduces long-form timestamp drift and produces word timestamps. It still adds an automatic speech recognition model and a language-specific alignment model. [Bain et al., Interspeech 2023](https://www.isca-archive.org/interspeech_2023/bain23_interspeech.html)

Qwen3-ForcedAligner-0.6B is a 2026 non-autoregressive model. It supports 11 languages and inputs up to five minutes. It is not a small universal server dependency. [Qwen3-ASR report](https://arxiv.org/abs/2601.21337) [official implementation](https://github.com/QwenLM/Qwen3-ASR)

Traditional forced alignment also needs an acoustic model, pronunciation data, and matching speech text. Training on the target domain can improve its boundary accuracy. [Montreal Forced Aligner paper](https://www.isca-archive.org/interspeech_2017/mcauliffe17_interspeech.html)

## Recommended synchronization pipeline

### 1. Analyze audio without changing media

Use FFmpeg inside the existing server container.

- Decode the selected audio stream to 16 kHz mono pulse-code modulation for analysis.
- Do not encode, remux, overwrite, or upload the video.
- Use bounded temporary storage and a process timeout.
- Sample the start, middle, and end before a full analysis pass.
- Reserve central processing unit capacity for browsing and playback.

Distributed sampling is important. Analysis of only the program start cannot prove that late drift or an internal cut is absent.

### 2. Build comparable activity signals

Create a speech-probability signal from the audio. Create a cue-active signal from subtitle timestamps.

Keep standard, forced, and subtitles for the deaf and hard of hearing as separate roles. Ignore non-dialogue cues only for scoring. Never remove their text.

Voice activity detection is cheaper than speech recognition. WhisperX also uses it to limit expensive work and avoid speech-boundary cuts. [WhisperX paper](https://www.isca-archive.org/interspeech_2023/bain23_interspeech.html)

### 3. Fit simple models before complex models

Fit these models in order:

1. Identity mapping.
2. Fixed offset.
3. Fixed offset with a common frame-rate scale.
4. A small set of monotonic offset segments.

Test common frame-rate ratios before a free scale search. Bound any free scale near 1.0.

Use a split penalty for piecewise alignment. Require each split to produce a material regional improvement. Limit the split count.

Do not move each cue independently. Cue-by-cue fitting can overfit noise and create unreadable timing.

### 4. Validate every region

Compute evidence for the original and proposed timing.

- Normalized speech overlap.
- Improvement over the unchanged file.
- Margin above the next-best transform.
- Start, middle, and end region coverage.
- Plausible offset and scale.
- Monotonic cue order.
- Cue bounds within the media duration.
- No large new overlap or duration loss.
- No hard media identity conflict.

Do not compare a raw correlation score across different programs. FFsubsync states that its raw score depends on duration and speech proportion. [Maintainer score explanation](https://github.com/smacke/ffsubsync/discussions/148)

### 5. Abstain when evidence is weak

Reject automatic synchronization for these conditions:

- Few dialogue cues.
- Low speech coverage.
- Similar scores for several transforms.
- Improvement in only one sampled region.
- An implausible offset, scale, or split count.
- A likely commentary track.
- A wrong edition or title signal.
- A translated subtitle that conflicts with lexical anchors.
- A transformation that fails structural validation.

Record the reason. Keep the downloaded candidate uninstalled or try the next candidate.

Selective classification formalizes this risk and coverage trade. A safe system can reduce errors by declining weak cases. [El-Yaniv and Wiener, 2010](https://www.jmlr.org/papers/v11/el-yaniv10a.html)

## Text-track compatibility

Use UTF-8 SRT as the managed sidecar format. Generate WebVTT at the browser boundary when needed.

Accept these inputs:

- UTF-8;
- UTF-8 with a byte order mark;
- UTF-16 little-endian or big-endian with a byte order mark;
- Windows-1252;
- SRT;
- WebVTT.

Decode once. Reject ambiguous or invalid input. Write managed output as UTF-8 without a byte order mark.

WebVTT files must use UTF-8. The format permits an optional UTF-8 byte order mark. [W3C WebVTT](https://www.w3.org/TR/webvtt1/)

The WHATWG Encoding Standard defines UTF-8, UTF-16LE, UTF-16BE, and Windows-1252 decoding behavior. It recommends UTF-8 for new interchange. [WHATWG Encoding Standard](https://encoding.spec.whatwg.org/)

Apple HTTP Live Streaming uses WebVTT or text-profile IMSC1 subtitles. WebVTT uses `text/vtt` or `text/plain`. [Apple HLS authoring specification](https://developer.apple.com/documentation/http-live-streaming/hls-authoring-specification-for-apple-devices/)

Plex lists SRT and WebVTT as supported text formats. It recommends UTF-8. It states that PGS and VobSub usually require video burn-in. [Plex local subtitle support](https://support.plex.tv/articles/200471133-adding-local-subtitles-to-your-media/) [Plex streaming overview](https://support.plex.tv/articles/200430303-streaming-overview/)

Jellyfin states that unsupported subtitles can require conversion or video burn-in. It identifies burn-in as the most intensive path. [Jellyfin codec support](https://jellyfin.org/docs/general/clients/codec-support/)

These sources support a text-first policy. They do not prove that every player supports every styling feature.

Do not convert ASS or SSA styling automatically in public v1. Do not attempt PGS or VobSub optical character recognition in public v1.

## Content safety

Synchronization can change cue times. Cleanup can change encoding and structure.

Neither operation can change:

- dialogue;
- names;
- profanity;
- lyrics;
- speaker identity;
- translation meaning;
- forced-subtitle meaning;
- subtitles for the deaf and hard of hearing meaning.

Keep an original digest, output digest, operation list, transform, confidence evidence, and recovery copy.

## Provider rate and health control

Track each provider independently.

- Configuration state.
- Last successful request.
- Remaining quota when supplied.
- Reset time when supplied.
- Last safe error.
- Next retry time.
- Consecutive transient failures.
- Circuit state.

Parse provider fields defensively. A missing header is not unlimited quota.

`Retry-After` can contain delay seconds or an HTTP date. A recipient must handle both forms. [RFC 9110, section 10.2.3](https://www.rfc-editor.org/rfc/rfc9110.html#section-10.2.3)

The proposed standard `RateLimit` fields remain an Internet-Draft in August 2026. Provider-specific `X-RateLimit-*` fields still need provider-specific parsing. [IETF RateLimit fields draft](https://datatracker.ietf.org/doc/draft-ietf-httpapi-ratelimit-headers/)

Use this failure policy:

| Result | Action |
| --- | --- |
| Success | Close the circuit and update quota evidence. |
| HTTP 401 or 403 | Stop retries until credentials change. |
| HTTP 429 | Honor `Retry-After` or the documented reset. Do not probe early. |
| HTTP 408, 500, 502, 503, 504, or network error | Use capped exponential backoff with jitter. |
| Repeated transient failure | Open the provider circuit. |
| Open circuit timeout | Permit one half-open health probe. |
| Malformed provider response | Record a safe error and open a short circuit. |

Retries can amplify overload. Amazon recommends timeouts, bounded retries, exponential backoff, and jitter. [Amazon Builders' Library](https://aws.amazon.com/builders-library/timeouts-retries-and-backoff-with-jitter/)

A circuit breaker prevents repeated requests to a dependency that is likely to remain unavailable. Pair it with bounded retries for transient faults. [Microsoft circuit breaker pattern](https://learn.microsoft.com/en-us/azure/architecture/patterns/circuit-breaker)

Current provider facts checked on 2026-08-30:

- SubDL documents `X-RateLimit-Limit`, `X-RateLimit-Remaining`, and `X-RateLimit-Reset` on every response. Its free plan lists 2,000 searches and 50 downloads each day. Its `/api/v2/me` call does not consume search quota. [SubDL developer API](https://subdl.com/at/developers)
- SubSource documents 60 requests per minute, 1,800 per hour, and 7,200 per day. It states that responses contain rate-limit headers. [SubSource API](https://beta.subsource.net/api-docs)
- OpenSubtitles requires the current `.com` REST API. The service announced the final shutdown of the old `.org` API in 2026. [OpenSubtitles announcement](https://forum.opensubtitles.com/t/opensubtitles-org-api-final-shutdown-notice-for-non-vip-users/5045) [REST API documentation](https://opensubtitles.stoplight.io/docs/opensubtitles-api)

Test credentials when they change. Cache the result. Do not spend provider quota each time a dashboard loads.

## Adaptive searching

Cache search outcomes by:

- stable media identity and media revision;
- requested language;
- subtitle role;
- provider;
- active subtitle plan revision.

Bazarr reduces the search frequency for media that repeatedly returns no subtitle. Its guide recommends adaptive searching for large wanted lists. [Bazarr performance guidance](https://wiki.bazarr.media/Additional-Configuration/Performance-Tuning/)

Use this initial Kinosail schedule. This schedule is a product proposal, not a provider rule.

| Outcome | Next automatic search |
| --- | --- |
| New media | Immediately |
| Transient provider failure | Provider backoff time |
| First complete no-result | Six hours |
| Second complete no-result | One day |
| Third complete no-result | Three days |
| Repeated complete no-result | Seven days |
| Installed subtitle | When the media, plan, or managed file changes |

Manual search can bypass the negative-result cache. It cannot bypass provider quota or an open circuit.

Invalidate relevant cache entries after a media revision, language change, role change, credential change, provider change, or manual restore.

## Public v1 evaluation

Use licensed or owner-supplied private media. Include:

- correct timing;
- fixed offsets;
- common frame-rate drift;
- one and several internal cuts;
- wrong editions and wrong titles;
- sparse speech, music, noise, and commentary;
- translated, standard, forced, and accessibility tracks;
- UTF-8, UTF-16, Windows-1252, SRT, and WebVTT;
- programs longer than the current analysis window.

Measure:

- wrong-file installation rate;
- harmful synchronization rate;
- automatic coverage after abstention;
- median and 95th-percentile cue boundary error;
- start, middle, and end improvement;
- provider requests per installed file;
- quota and retry compliance;
- processing time, memory, and disk input/output;
- restore success.

Use SubER when a human reference exists. It evaluates text, segmentation, and timing together. [SubER paper](https://aclanthology.org/2022.iwslt-1.1/) [official implementation](https://github.com/apptek/SubER)

The release corpus must show zero owner-file loss. It must also show zero automatic changes to wrong-title or wrong-edition subtitles.

## Implementation boundary

Public v1 should include:

- local voice activity detection;
- global offset and bounded frame-rate correction;
- penalized piecewise monotonic alignment;
- distributed timeline checks;
- confidence scoring and abstention;
- UTF-8 SRT output and WebVTT delivery;
- provider quota, backoff, circuit, and adaptive-search state;
- complete history and recovery.

Public v1 should not require:

- Whisper or another transcription model;
- Qwen3 or another large forced aligner;
- cloud audio upload;
- a second service container;
- video transcoding;
- optical character recognition;
- subtitle text rewriting.

This boundary gives broad player support with small setup cost. It also keeps the most damaging automatic failures behind explicit evidence gates.
