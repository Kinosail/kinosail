# Best free subtitle providers

Research snapshot: 2026-08-30.

This note evaluates current subtitle sources for Kinosail Subtitles. It uses provider-owned API documentation, terms, privacy policies, service notices, and source repositories. It excludes undocumented scraping as a production interface.

## Executive decision

Ship this provider set:

1. **OpenSubtitles.com** for the best exact-file match and broad general coverage.
2. **SubDL** for broad movie and television coverage, generous free search limits, and release-aware results.
3. **SubSource** for additional broad coverage, including imported Subscene material and useful forced and hearing-impaired metadata.

Defer **Jimaku**. It is the best documented Japanese specialty candidate, but it does not meet this delivery's minimal-setup and low-ambiguity bar.

The three listed providers form the general automatic pool. Do not add an undocumented website adapter merely to increase the provider count.

SubSource has one material policy condition. Its official disclaimer restricts its content to personal use and prohibits commercial use. Kinosail should require explicit Owner acceptance, keep files inside the household, and obtain provider clarification before marketing or enabling its adapter in a commercial service. SubSource also says subtitle copyright stays with the translator and alteration is not allowed. [SubSource terms](https://subsource.net/terms), [SubSource DMCA and use disclaimer](https://subsource.net/dmca)

Jimaku has a stable official API but no public subtitle-content license or general terms page was found. Its OpenAPI document labels the API description as AGPL-3.0. That label does not grant rights to subtitle files. The API needs a separate account and key, anime matching is best with an AniList ID that Kinosail does not currently store, and language and accessibility facts are filename guesses. No fee or paid tier is documented, but no fixed free allowance is documented either. Defer it until Jimaku clarifies integration terms and Kinosail has an AniList identity seam. [Jimaku API](https://jimaku.cc/api/docs), [Jimaku help](https://jimaku.cc/help)

## User assumptions

The default behavior should reflect common household needs:

- Use an existing safe sidecar or embedded text track before any provider request.
- Get the Owner's preferred language with no manual search.
- Prefer human, complete dialogue subtitles over machine translations.
- Prefer a non-hearing-impaired file unless the Owner requests subtitles for deaf and hard-of-hearing users.
- Also retain a verified forced or foreign-parts track when one exists.
- Prefer an exact file hash, then exact media identifiers and episode identity, then release-name similarity.
- Download only the best candidate. Do not spend download quota on every search result.
- Keep unknown household sidecars unless the existing exact-hash upgrade rule permits replacement.
- Send only bounded title, filename, language, media type, identifiers, episode facts, and local file hash data.
- Never upload media bytes for subtitle search.
- Do not require Sonarr, Radarr, a scraper proxy, a private tracker, or another container.

## Ranked comparison

| Rank | Provider | Setup | Coverage role | Match data | Accessibility data | Free limits | File behavior | Decision |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | OpenSubtitles.com | Application API key, account username, and password | Broad movie and television catalog | OpenSubtitles ID, IMDb, TMDB, parent IDs, season, episode, title, filename, and OpenSubtitles movie hash | Structured hearing-impaired and foreign-parts-only flags; trusted, artificial intelligence, and machine-translation flags | A free account currently gets 20 downloads each day. Search and download endpoints also return plan-specific rate or quota data. | The download endpoint returns a temporary direct text-file URL. The file is UTF-8. The URL expires within three hours. | Keep and query first when a movie hash exists. |
| 2 | SubDL | Free account and one API key | Broad movie, television, and anime-friendly catalog | IMDb, TMDB, SubDL ID, title, filename, year, type, season, episode, and release names | Structured hearing-impaired flag; no documented forced-only flag | 2,000 free searches each day. Current version 2 documentation shows 50 free downloads each day. The version 1 public-link documentation also describes an anonymous per-IP limit, so code must use returned account and rate data instead of promising one fixed limit. | Results provide `dl.subdl.com` ZIP links. `unpack=1` can expose individual files with name, language, hearing-impaired flag, format, size, MD5, and direct URL. | Keep. Prefer exact identifiers and unpacked text files. |
| 3 | SubSource | Free account and one API key | Broad movie and television catalog with Subscene-derived coverage | Text or IMDb search returns an internal movie ID plus IMDb, TMDB, type, year, and season. Subtitle search uses that ID or exact release information. | Structured hearing-impaired and foreign-parts flags, but imported records often lack these fields | 60 requests per minute, 1,800 per hour, and 7,200 per day. Responses include rate-limit headers. | Download returns one ZIP stream from the official API. Results include language, release information, production type, frame rate, rating, size, and uploader data. | Add behind explicit personal-use terms acceptance. Do not hard-filter missing imported metadata. |
| 4 | Jimaku | Account and one API key; no paid tier is documented | Japanese subtitles for anime and Japanese live action | AniList ID, encoded TMDB movie or television ID, fuzzy title, entry ID, and best-effort episode filename matching | No structured language, hearing-impaired, or forced field. These facts appear only in filenames such as `.ja[sdh].srt`. | The service publishes no fixed numeric allowance. It returns HTTP 429 and `x-ratelimit-*` reset headers for IP-based limits. | File results contain URL, name, byte size, and modification time. The site accepts SRT, ASS, SSA, ZIP, 7z, SUB, SUP, and IDX, while recommending SRT or ASS. Downloads use Jimaku-relative URLs. | Defer. It adds setup and weak matching for most households. Reassess after terms clarification and AniList support. |

Sources: [OpenSubtitles API schema](https://stoplight.io/api/v1/projects/opensubtitles/opensubtitles-api/nodes/open_api.json), [OpenSubtitles free-account statement](https://forum.opensubtitles.com/t/id-like-to-renew/5925), [SubDL API](https://subdl.com/api-doc), [SubDL developer limits](https://subdl.com/at/developers), [SubSource API](https://subsource.net/api-docs), [Jimaku API](https://jimaku.cc/api/docs), [Jimaku file guidance](https://jimaku.cc/help).

## Provider details

### OpenSubtitles.com

OpenSubtitles has the strongest deterministic matching interface. Its search documentation says to send a movie hash when available, prefer IMDb or TMDB identifiers over text, and send the parent television ID with season and episode when an episode ID is unavailable. A movie-hash result receives `moviehash_match`, and matching results come first. [OpenSubtitles search contract](https://opensubtitles.stoplight.io/docs/opensubtitles-api/a172317bd5ccc-search-for-subtitles)

Search results include language, release, frames per second, hearing-impaired, foreign-parts-only, trusted-source, artificial-intelligence translation, machine translation, ratings, feature identifiers, and file IDs. Machine-translated results are excluded by default. Artificial-intelligence-translated results are included by default, but Kinosail should rank them below known human work unless the Owner changes this rule. [OpenSubtitles API schema](https://stoplight.io/api/v1/projects/opensubtitles/opensubtitles-api/nodes/open_api.json)

The download operation needs the application key and authenticated token. It returns `remaining` and reset-time values plus a temporary file URL. The file URL is valid for no more than three hours and returns UTF-8 text. Kinosail should create one download request only after it selects the best candidate. [OpenSubtitles API schema](https://stoplight.io/api/v1/projects/opensubtitles/opensubtitles-api/nodes/open_api.json)

The current free account allowance is 20 subtitle downloads each day. OpenSubtitles explicitly supports third-party applications and says the new API supports commercial projects. The legacy OpenSubtitles.org XML-RPC API is shutting down and must not be added. [Free account statement](https://forum.opensubtitles.com/t/id-like-to-renew/5925), [OpenSubtitles API direction](https://blog.opensubtitles.com/opensubtitles/web/understanding-opensubtitles-websites-and-services), [legacy shutdown notice](https://forum.opensubtitles.com/t/opensubtitles-org-api-final-shutdown-notice-for-non-vip-users/5045)

Privacy is reasonable for this use but not local-only. OpenSubtitles logs the IP address, requested pages or files, time, response, and transferred volume for security. Its published policy says ordinary server logs are retained for up to seven days unless needed as evidence. Kinosail must disclose that the provider receives search metadata and network identifiers. [OpenSubtitles privacy policy](https://www.opensubtitles.com/hr/privacy)

### SubDL

SubDL accepts movie and television searches by IMDb, TMDB, SubDL ID, film name, filename, type, year, season, episode, and language. It can return release lists, hearing-impaired flags, full-season packs, and unpacked file entries. This maps well to Kinosail's current media facts. [SubDL API](https://subdl.com/api-doc)

Prefer `unpack=1`. Each unpacked item has a filename, release, season, episode, language, hearing-impaired flag, format, size, MD5, and direct URL. This avoids downloading a full-season archive when one episode is needed. If only a ZIP is available, retain the existing archive count, entry name, uncompressed size, and subtitle-content limits. [SubDL API](https://subdl.com/api-doc)

The free search limit is 2,000 requests each day. The current version 2 account documentation shows a separate 50-download daily free allowance and rate-limit headers. The version 1 page describes anonymous `dl.subdl.com` links with a 300-download per-IP daily limit and paid key-authenticated downloads. These surfaces can change independently. Kinosail should inspect `/me`, `X-RateLimit-Limit`, `X-RateLimit-Remaining`, and `X-RateLimit-Reset`; it should not show a hard-coded promise. [SubDL API](https://subdl.com/api-doc), [SubDL developer limits](https://subdl.com/at/developers)

SubDL permits API automation subject to published authentication, caching, and rate limits. It says users are responsible for lawful subtitle use. The privacy policy says API requests and downloads are counted, and server logs include IP address, user agent, requested URLs, and timestamps. [SubDL terms](https://subdl.com/terms), [SubDL privacy](https://subdl.com/privacy)

### SubSource

SubSource released an official API with a simple `X-API-Key` header. Movie search accepts text or IMDb input and returns an internal movie ID, IMDb ID, TMDB ID, content type, year, season, and subtitle count. This needs one discovery request before subtitle search when Kinosail has no saved SubSource ID. [SubSource API](https://subsource.net/api-docs)

Subtitle results provide language, release information, commentary, file count, size, hearing-impaired and foreign-parts flags, frame rate, production and release type, downloads, rating, preview, uploader, and contributors. The API warns that imported Subscene records often lack production, release, hearing-impaired, foreign-parts, and frame-rate metadata. Kinosail should use these fields when present but not exclude an otherwise strong candidate only because an imported field is absent. [SubSource API](https://subsource.net/api-docs)

The service allows 60 requests each minute, 1,800 each hour, and 7,200 each day per key. It documents HTTP 429 and rate-limit response headers. The download endpoint returns an `application/zip` stream. Kinosail should apply its existing bounded ZIP validation before candidate parsing. [SubSource API](https://subsource.net/api-docs)

SubSource collects account email and IP address. Its policy says it uses encrypted storage and retains data as needed to provide the service. This is more account data than Jimaku claims, but it does not change the media boundary. [SubSource privacy](https://subsource.net/policy)

The legal boundary needs product treatment. SubSource says translators retain subtitle ownership, third parties may not alter subtitles, and resynchronization plus re-upload requires preserved attribution. It also says content is for personal use and commercial use is prohibited. The text does not clearly permit private local cleanup or synchronization. Preserve the downloaded subtitle content unchanged and perform validation only. Do not clean, synchronize, shift frame rate, or convert SubSource files without written provider clarification. Store and serve the file only for the Owner's household. [SubSource terms](https://subsource.net/terms), [SubSource use disclaimer](https://subsource.net/dmca)

### Jimaku

Jimaku is a useful future specialty source, not a fourth general provider. Its directory entries use AniList identifiers for anime and encoded TMDB identifiers such as `movie:1234` or `tv:1234` for other media. Search also supports fuzzy title text. Episode filtering is a best-effort filename guess and is ignored for movies. [Jimaku API](https://jimaku.cc/api/docs)

The API key goes directly in the `Authorization` header. The official documentation requires an account but presents no price, subscription, or paid tier. Rate limits are IP-based. A limited response uses HTTP 429 and supplies limit, remaining, reset timestamp, and wait-duration headers. No fixed numeric allowance is part of the contract, so a future adapter must schedule from the headers. [Jimaku API](https://jimaku.cc/api/docs)

File responses have only a URL, name, size, and modification time. Language, release, closed-caption, subtitles for deaf and hard-of-hearing, and artificial-intelligence facts must be parsed from the filename. Jimaku's own examples use names such as `.ja.srt`, `.ja[sdh].srt`, and `[Generated by Whisper]`. Kinosail must treat parsed flags as weaker evidence than structured provider metadata. [Jimaku help](https://jimaku.cc/help)

Jimaku supports SRT, ASS, SSA, ZIP, 7z, SUB, SUP, and IDX. Its guidance recommends SRT or ASS and discourages archives. The Kinosail adapter should accept SRT, ASS, SSA, and bounded ZIP only. It should reject 7z and image-based files because the current capability is safe text-subtitle acquisition. [Jimaku help](https://jimaku.cc/help)

Jimaku says an account is required for API access and that it stores no email or personal information for account creation. The site has no direct account deletion flow; users must contact administrators. No public terms or subtitle-content license was found on the official site. The API specification's AGPL-3.0 label covers the API description, not user subtitle rights. These gaps are acceptable for private investigation but not for promoting a bundled commercial integration without clarification. [Jimaku API](https://jimaku.cc/api/docs), [Jimaku help](https://jimaku.cc/help)

## Automation policy

### Search order

Use one shared application operation for manual and scheduled search:

1. Check safe sidecar and embedded text tracks.
2. Search configured general providers concurrently with bounded fan-out.
3. Normalize candidates into the existing shared candidate type.
4. Deduplicate by provider checksum, normalized release, and validated subtitle content hash.
5. Select one candidate across every provider.
6. Download and validate it.
7. Try the next candidate only after a provider error or validation failure.
8. Write atomically under the existing sidecar and recovery rules.

Do not define provider order as quality order. Exact file hash and media identity must outrank provider name. When two candidates have equal match quality, prefer human work, a trusted or well-rated uploader, the requested accessibility type, a direct episode file, and a text format.

### Language and accessibility defaults

Assume the Owner wants one full dialogue subtitle in the configured preferred language. Use these defaults:

- Exclude known machine translations.
- Rank known artificial-intelligence translations below human subtitles.
- Prefer standard subtitles over hearing-impaired subtitles unless the Owner selects an accessibility preference.
- Do not reject a candidate only because an imported record lacks the hearing-impaired flag.
- Treat forced or foreign-parts subtitles as a separate track, not a substitute for a requested full subtitle.
- Keep both a full track and a forced track when both pass validation.
- Never infer `forced` from a short cue count alone. Use provider metadata or a strong filename marker.

### Rate and failure behavior

- Cache normalized search responses briefly by provider, media identity, language, and episode.
- Cache stable provider IDs longer than search results.
- Honor `Retry-After`, provider reset headers, and quota reset values.
- Apply exponential backoff with jitter when only HTTP 429 or a transient server error is available.
- Stop authentication retries after a rejected credential response.
- Open a provider circuit after repeated failures. Continue with other configured providers.
- Show the Owner which provider is limited or unavailable without exposing credentials or response bodies.
- Never let one provider outage block local inventory, embedded subtitle use, or another provider.

### Trust boundary

Every adapter must:

- enforce an HTTPS API origin and a provider-specific download-host allowlist;
- reject redirects to unlisted origins and unsafe network destinations;
- bound request values, response bytes, result count, redirects, archive members, expanded bytes, filename length, and subtitle bytes;
- reject unknown JSON fields where the provider contract is stable;
- parse language and episode identifiers once into the shared domain model;
- reject absolute paths, traversal, links, devices, executables, scripts, nested archives, encrypted archives, and null bytes;
- accept only supported text subtitle formats and bounded ZIP files;
- validate the decoded subtitle language and cue timing before any write;
- record safe provider, match, quota, and validation facts without titles, filenames, hashes, credentials, or download URLs in logs.

## Rejected alternatives

### OpenSubtitles.org

Reject the legacy XML-RPC API. OpenSubtitles announced complete shutdown for third-party applications and directs developers to OpenSubtitles.com REST. [Official shutdown notice](https://forum.opensubtitles.com/t/opensubtitles-org-api-final-shutdown-notice-for-non-vip-users/5045)

### Anime Tosho

Reject Anime Tosho. New ingestion stopped on 2026-05-09. The remaining service is a frozen archive and is planned to terminate. [Official shutdown update](https://animetosho.org/about/shutdown2)

### TsukiHime and torrent-derived mirrors

Reject TsukiHime for the default provider pool. Its own documentation says it automatically scrapes public torrent indexers, distributes files to changing third-party hosts, deletes local files after mirroring, cannot restore dead links, and warns that malicious files can appear. It also requires xz or 7z handling for some extracted content. This conflicts with stable origins, safe text-only downloads, and predictable availability. [TsukiHime wiki](https://tsukihime.org/wiki)

### Subsarr and Subscene dumps

Reject Subsarr for the supported installation. It needs another server container, a roughly 97 GB multi-part archive, an imported database, and separate subtitle storage. Kinosail requires one Server container. SubSource already exposes much Subscene-derived coverage through a hosted documented API. [Subsarr README](https://github.com/slimcdk/subsarr)

### Addic7ed, Podnapisi, TVSubtitles, YIFY, and similar websites

Do not add these without a current provider-owned API contract and automation permission. Their current official surfaces expose interactive website pages, not a documented stable public search-and-download API. An HTML parser would depend on markup, cookies, anti-bot behavior, or undocumented endpoints. That creates silent breakage and unclear terms. Reassess any provider that later publishes an authenticated, versioned API with rate and download rules. [Addic7ed current subtitle page](https://www.addic7ed.com/serie/Marco_Polo_%282014%29/2/6/Serpent%27s_Terms), [Podnapisi current site](https://www.podnapisi.net/)

### CAPTCHA or cookie brokers

Reject any provider that needs CAPTCHA solving, browser impersonation, shared cookies, or a remote scraping proxy. These mechanisms expose household activity, create a credential leak path, and have no stable API contract.

### Private trackers

Reject private-tracker subtitle providers. They require tracker membership or passkeys, expose library searches to a higher-risk account, and conflict with minimal setup. Do not store a tracker passkey in Kinosail for subtitle lookup.

### Paid translation and transcription APIs

Do not describe OpenSubtitles artificial-intelligence translation or transcription as a free provider. Those endpoints use purchased credits. Local speech analysis can continue as a match-confidence check. Full local transcription is a separate product capability with large CPU, memory, language-quality, and container-size costs. [OpenSubtitles artificial-intelligence API](https://ai.opensubtitles.com/docs), [OpenSubtitles artificial-intelligence terms](https://ai.opensubtitles.com/terms)

## Delivery recommendation

The smallest high-value expansion is:

1. Add SubSource as the third general provider.
2. Add an explicit personal-use terms acceptance next to its API key.
3. Preserve OpenSubtitles.com and SubDL as existing providers.
4. Defer Jimaku until provider terms and AniList identity support are clear.
5. Aggregate searches, then perform one validated download.
6. Keep machine translation disabled by default.
7. Keep forced subtitles as a separate optional result.
8. Do not add any scraper, proxy, tracker, archive dump, or sidecar service.

This set maximizes useful free coverage without converting Kinosail into a brittle collection of website scrapers.
