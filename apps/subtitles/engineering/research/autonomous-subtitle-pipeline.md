# Autonomous subtitle pipeline

## Product decision

Kinosail Subtitles uses one opinionated pipeline. The owner selects a language and adds a free provider key. The server makes the remaining decisions.

The default pipeline is:

1. Keep a valid local sidecar.
2. Prefer an embedded text track in the requested language.
3. Search free provider APIs with provider identifiers and the release filename.
4. Reject the wrong title, year, season, episode, language, or accessibility type.
5. Rank every remaining release instead of taking the provider's first result.
6. Download at most three candidates from each configured provider.
7. Parse and normalize each candidate before any media-folder write.
8. Use audio alignment when the release match is not exact.
9. Write only a confident result with an exclusive atomic operation.
10. Retry later when no result clears the confidence gate.

This policy prefers a missing subtitle over a convincing but incorrect subtitle.

## Evidence

### Discovery and ranking

The OpenSubtitles hash uses file size plus the first and last 64 KiB. It is fast and useful for exact release matching. It is not a cryptographic integrity check. Source: <https://github.com/opensubtitles/oshash>

SubDL supports title, filename, IMDb, TMDB, season, episode, year, language, release, frame-rate, accessibility, and archive-file metadata. Its free tier currently permits 2,000 searches and 50 downloads each day. Source: <https://subdl.com/developers>

Subliminal and Bazarr score candidates from independent matches. Their strongest signals are hash, title or series, year, season, episode, release group, source, and frame rate. Bazarr defaults to a 70 percent minimum score. Sources: <https://github.com/Diaoul/subliminal/blob/main/docs/user/usage.rst> and <https://github.com/morpheus65535/bazarr/blob/master/bazarr/app/config.py>

Kinosail changes this model in two ways. It uses hard identity conflicts instead of negative weights. It also verifies weak release matches against local audio before writing them.

### Synchronization

FFsubsync represents subtitle timing and detected speech as time signals. It uses cross-correlation to find an offset and tests common frame-rate corrections. Its current quality gate skips low-score or implausible results. Source: <https://github.com/smacke/ffsubsync>

ALASS uses language-independent voice activity and dynamic programming. Its published project results report 88 to 98 percent good files. It also handles offsets, frame-rate changes, and edition breaks. Source: <https://github.com/kaegi/alass>

Research on translated subtitle alignment supports anchor-based and monotonic alignment. Timing and normalized duration improve alignment when text differs. Sources: <https://aclanthology.org/L08-1576/> and <https://aclanthology.org/L08-1218/>

Kinosail uses a local Silero voice activity detector. It performs a coarse search, tests common 23.976 and 25 frame-rate ratios, then refines the offset. It does not modify a file when the signal score is weak.

### Cleaning and quality

Web Video Text Tracks require ordered start times and an end time after the start time. Source: <https://www.w3.org/TR/webvtt1/>

Unicode Normalization Form C gives canonically equivalent text one representation without compatibility folding. Source: <https://unicode.org/reports/tr15/>

Netflix recommends up to 17 characters per second for adult subtitle templates. Its timing guide also favors close audio timing and readable cue duration. Sources: <https://partnerhelp.netflixstudios.com/hc/en-us/articles/219375728-Timed-Text-Style-Guide-Subtitle-Templates> and <https://partnerhelp.netflixstudios.com/hc/en-us/articles/360051554394-Timed-Text-Style-Guide-Subtitle-Timing-Guidelines>

BBC audience research found that correct timing and wording matter more than one universal reading-speed ceiling. Source: <https://downloads.bbc.co.uk/rd/pubs/whp/whp-pdf-files/WHP306.pdf>

SubER research also shows that subtitle quality combines text, segmentation, and timing. Source: <https://aclanthology.org/2022.iwslt-1.1/>

Kinosail therefore reports reading-speed problems but does not rewrite dialogue. Automatic cleaning is limited to encoding normalization, invalid timing rejection, duplicate removal, safe tag cleanup, and clear provider-credit lines.

## Default thresholds

- Search results: 30 per provider.
- Download attempts: 3 per provider and 6 total with both providers enabled.
- Automatic cycle: 10 wanted titles, no more than once every 15 minutes.
- Minimum metadata score: 60 out of 100.
- Exact release threshold: 0.80 token similarity.
- Audio search window: plus or minus 120 seconds.
- Frame-rate ratios: 1.000, 25/23.976, and 23.976/25.
- Voice probability: 0.50.
- Maximum subtitle file: 4 MiB.
- Maximum cues: 100,000.
- Maximum cue timeline: 36 hours.
- Managed sidecars: upgrade only when a trusted candidate gains at least ten score points.
- Unknown sidecars: upgrade only for an exact OpenSubtitles hash match and retain one recovery backup.
- Hearing-impaired subtitles: valid fallback, but standard subtitles win by default.
- Machine translation: penalized and never preferred over a human subtitle with equal identity.

These defaults optimize for unattended household libraries. Advanced controls should remain exceptional.

## Provider policy

Supported providers must publish an API and permit automation. Kinosail must honor rate limits and must not bypass anti-bot controls.

The provider order is:

1. Local and embedded tracks.
2. SubDL free API.
3. OpenSubtitles REST API with a cached account session.
4. Local speech recognition after a bounded model and hardware policy are complete.

Website scraping is not a provider strategy.
