# Bazarr language and provider parity

Date: 2026-08-30

Bazarr reference commit: [`da73aeaf5e4d89ad86c8d559d3abd0e4129b24b2`](https://github.com/morpheus65535/bazarr/tree/da73aeaf5e4d89ad86c8d559d3abd0e4129b24b2)

## Decision

Kinosail Subtitles should exceed Bazarr's language model without copying its provider count.

Use these targets:

1. Store canonical BCP 47 language tags.
2. Offer all 184 ISO 639-1 base languages.
3. Add useful script and region variants.
4. Sort common choices first. Keep the full catalog searchable.
5. Store an ordered list of wanted languages, not one language.
6. Map each canonical tag at each provider boundary.
7. Use only documented provider APIs. Do not scrape subtitle websites.

This approach gives a wider and more precise language-identity catalog than Bazarr. It keeps Kinosail's smaller security and maintenance surface. Standard, forced, and subtitles for the deaf and hard of hearing remain a separate profile-parity project.

## Verified comparison

### Language model

Bazarr loads each `pycountry` language that has an ISO 639-1 code. Its vendored `pycountry` version is 26.2.16. That data contains 184 such records. Bazarr then adds three distinct regional records for Brazilian Portuguese, Traditional Chinese, and Latin American Spanish. Its effective catalog contains 187 records. [Bazarr language loader](https://github.com/morpheus65535/bazarr/blob/da73aeaf5e4d89ad86c8d559d3abd0e4129b24b2/bazarr/languages/get_languages.py) [Bazarr custom languages](https://github.com/morpheus65535/bazarr/blob/da73aeaf5e4d89ad86c8d559d3abd0e4129b24b2/bazarr/languages/custom_lang.py) [Bazarr dependency versions](https://github.com/morpheus65535/bazarr/blob/da73aeaf5e4d89ad86c8d559d3abd0e4129b24b2/libs/version.txt)

Bazarr language profiles can request multiple languages. Its search path also keeps forced and hearing-impaired requirements separate. [Bazarr manual search](https://github.com/morpheus65535/bazarr/blob/da73aeaf5e4d89ad86c8d559d3abd0e4129b24b2/bazarr/subtitles/manual.py)

Kinosail currently stores one lower-case language string. It accepts any two-letter or three-letter base code with an optional two-letter suffix. It does not verify registry membership. It rejects valid script tags such as `zh-Hant` and numeric regions such as `es-419`. [settings store](../../internal/server/settings_subtitles.go) [server validation](../../internal/server/subtitle_provider.go) [deployment validation](../../internal/configuration/subtitle.go)

Kinosail's candidate normalization recognizes a small fixed alias set. It does not provide a complete ISO 639-2 or provider-code conversion layer. [candidate normalization](../../internal/server/subtitle_candidate.go)

### Provider model

Bazarr exposes 59 provider choices at the reference commit. That list includes embedded extraction and local Whisper. Several entries require website cookies, user agents, tracker passkeys, or site credentials. [Bazarr provider list](https://github.com/morpheus65535/bazarr/blob/da73aeaf5e4d89ad86c8d559d3abd0e4129b24b2/frontend/src/pages/Settings/Providers/list.ts)

Kinosail currently uses embedded tracks plus three network APIs: SubDL, OpenSubtitles, and SubSource. [provider orchestration](../../internal/server/subtitle_provider.go)

| Provider | Documented automation | Free access evidence | Language evidence | Decision |
| --- | --- | --- | --- | --- |
| SubDL | Its official API supports identifiers, Movie and Episode fields, languages, release data, hearing-impaired flags, and direct file links. | The official page offers free keys with 2,000 requests each day. Anonymous downloads allow 300 files each day per IP. | SubDL reports files in 92 languages and 111 supported catalog languages. Bazarr maps 59. | Keep the verified 59-code mapping. Extend it only from an official code list. |
| OpenSubtitles.com | Its current REST API supports language discovery, search, login, download, hashes, and role filters. | The official help center documents 5 daily downloads without an account and 20 with a free account. | `/infos/languages` returns the current provider catalog. | Keep a reviewed static snapshot. Refresh it from the official endpoint during a release. |
| SubSource | Its official API provides Movie search, subtitle search, subtitle details, and downloads. | A profile can create a key. The documented limits are 60 requests per minute, 1,800 per hour, and 7,200 per day. | Bazarr maps 87 SubSource language names. Kinosail currently maps 35. | Keep for personal use. Expand only verified mappings. Preserve downloaded subtitle bytes. |

Sources: [SubDL API](https://subdl.com/api-doc), [SubDL catalog facts](https://subdl.com/about), [SubDL terms](https://subdl.com/terms), [Bazarr SubDL converter](https://github.com/morpheus65535/bazarr/blob/da73aeaf5e4d89ad86c8d559d3abd0e4129b24b2/custom_libs/subliminal_patch/converters/subdl.py), [OpenSubtitles API documentation](https://ai.opensubtitles.com/docs), [OpenSubtitles free limits](https://opensubtitles.tawk.help/article/about-the-api), [SubSource API](https://subsource.net/api-docs), [SubSource terms](https://subsource.net/terms), [Bazarr SubSource converter](https://github.com/morpheus65535/bazarr/blob/da73aeaf5e4d89ad86c8d559d3abd0e4129b24b2/custom_libs/subliminal_patch/converters/subsource.py), [Kinosail SubSource mapping](../../internal/server/subsource_candidate.go).

SubDL's API documentation links to a machine-readable language list. That linked resource returned HTTP 404 on 2026-08-30. The 111-language total does not identify the missing provider codes. The implementation therefore uses the 59 codes verified by Bazarr's converter and rejects unverified SubDL languages.

OpenSubtitles uses a reviewed 106-code static snapshot from its official language response. Runtime discovery would make language support change without a Kinosail release and could make the UI depend on credentials or provider availability. Kinosail therefore does not claim current live catalog coverage. A count and digest test lock the snapshot. A reviewed release can refresh it from `/infos/languages`.

SubSource states that subtitle rights remain with each translator. It prohibits third-party alteration. Kinosail must not clean, retime, remove credits, or rewrite a SubSource subtitle. It can extract the selected file without changing its bytes. [SubSource terms](https://subsource.net/terms) [current preservation path](../../internal/server/subtitle_provider.go)

## Important mapping defects

Syntax support is not provider support.

- SubDL uses provider codes such as `BR_PT` and `ZH_BG`. Sending upper-case BCP 47 values is not sufficient. [Bazarr SubDL converter](https://github.com/morpheus65535/bazarr/blob/da73aeaf5e4d89ad86c8d559d3abd0e4129b24b2/custom_libs/subliminal_patch/converters/subdl.py)
- OpenSubtitles distinguishes `pt-br`, `pt-pt`, `zh-cn`, and `zh-tw`. Its language endpoint is the authority for reviewed snapshot updates. [OpenSubtitles language endpoint](https://ai.opensubtitles.com/docs)
- SubSource uses language names, including `Brazillian Portuguese` in Bazarr's current converter. Provider spellings belong only in the adapter. [Bazarr SubSource converter](https://github.com/morpheus65535/bazarr/blob/da73aeaf5e4d89ad86c8d559d3abd0e4129b24b2/custom_libs/subliminal_patch/converters/subsource.py)
- A broad provider value such as `ZH` must not silently satisfy an exact `zh-Hant` request. The user must permit a base-language fallback.

Use explicit mappings for the most important variants:

| Canonical Kinosail tag | OpenSubtitles | SubDL | SubSource | Safe fallback |
| --- | --- | --- | --- | --- |
| `pt-BR` | `pt-br` | `BR_PT` | `Brazillian Portuguese` | None |
| `pt-PT` | `pt-pt` | No exact mapping; `PT` is broad | No exact mapping; `Portuguese` is broad | None |
| `zh-Hans` | `zh-cn` | No exact mapping; `ZH` is broad | No exact mapping; `Chinese BG code` is broad | None |
| `zh-Hant` | `zh-tw` | `ZH_BG` | No exact mapping | Only with base fallback enabled |
| `es-419` | `ea` | No exact mapping; `ES` is broad | No exact mapping; `Spanish` is broad | None |
| `es-ES` | `sp` | No exact mapping; `ES` is broad | No exact mapping; `Spanish` is broad | None |

The reviewed OpenSubtitles snapshot comes from its official language response. SubDL and SubSource mappings come from Bazarr's current provider converters. [OpenSubtitles languages](https://ai.opensubtitles.com/docs) [SubDL converter](https://github.com/morpheus65535/bazarr/blob/da73aeaf5e4d89ad86c8d559d3abd0e4129b24b2/custom_libs/subliminal_patch/converters/subdl.py) [SubSource converter](https://github.com/morpheus65535/bazarr/blob/da73aeaf5e4d89ad86c8d559d3abd0e4129b24b2/custom_libs/subliminal_patch/converters/subsource.py)

## Comprehensive common-first catalog

BCP 47 is the internal format. IANA maintains the valid subtag registry. Unicode CLDR supplies user-facing display names and case normalization. Tags are case-insensitive, but canonical display uses forms such as `pt-BR` and `zh-Hant`. Kinosail keeps IANA's registered `tl` and `sh` identities distinct from `fil` and `sr-Latn`; it does not apply CLDR legacy alias collapsing to stored subtitle identities. [RFC 5646](https://www.rfc-editor.org/rfc/rfc5646) [IANA registry](https://www.iana.org/assignments/language-subtags-tags-extensions) [Unicode LDML](https://unicode.org/reports/tr35/)

Use this visible order. The order is a product launch order, not a claim about exact speaker totals.

### Tier 1: show first

`en`, `es`, `es-419`, `fr`, `de`, `pt-BR`, `pt-PT`, `it`, `nl`, `pl`, `ru`, `uk`, `tr`, `ar`, `fa`, `he`, `hi`, `bn`, `ur`, `id`, `ms`, `vi`, `th`, `zh-Hans`, `zh-Hant`, `ja`, `ko`, `fil`, `tl`, `ta`, `te`, `sw`, `ro`

### Tier 2: prominent regional choices

`cs`, `sk`, `hu`, `bg`, `el`, `sr-Cyrl`, `sr-Latn`, `hr`, `bs`, `sl`, `mk`, `sq`, `ca`, `eu`, `gl`, `sv`, `da`, `nb`, `nn`, `fi`, `is`, `et`, `lv`, `lt`, `ka`, `hy`, `az`, `kk`, `uz`, `km`, `my`, `ne`, `si`, `mr`, `gu`, `pa`, `ml`, `kn`, `am`, `af`

### Tier 3: full catalog

Include every remaining ISO 639-1 language. Add registered three-letter languages when they have practical subtitle demand, including `fil`, `yue`, `ckb`, and `ceb`. Keep unsupported choices visible. Show “No configured provider” instead of removing them.

This catalog has more than Bazarr's 187 choices once the script variants are included. Provider availability remains a separate fact.

## Data model

Use one bounded ordered list for this language-identity change:

```text
tag       canonical BCP 47 tag
priority  stable list order
```

Recommended limits:

- Accept 1 to 20 preferences.
- Reject duplicates after canonicalization.
- Reject unknown or deprecated tags unless canonicalization produces a current tag.
- Search providers only when an exact adapter mapping exists.
- Do not broaden an exact regional or script request to a base provider value.
- Keep the existing single language as the migration's first list item.

Role-specific preferences and explicit base-language fallback are separate follow-up work. This change does not silently approximate either feature.

The IANA registry and Unicode canonicalization rules define valid and deprecated tags. [IANA registry](https://www.iana.org/assignments/language-subtags-tags-extensions) [Unicode canonicalization](https://unicode.org/reports/tr35/)

## Additional free providers

Do not add another broad provider now. SubDL, OpenSubtitles, and SubSource already provide three documented APIs.

Consider Jimaku later for Japanese anime. It has a documented API and Bazarr limits the provider to Japanese. Its public API documentation does not state a stable free quota or complete automation terms. Obtain written permission before enabling unattended library scans. [Jimaku API](https://jimaku.cc/api/docs) [Bazarr Jimaku provider](https://github.com/morpheus65535/bazarr/blob/da73aeaf5e4d89ad86c8d559d3abd0e4129b24b2/custom_libs/subliminal_patch/providers/jimaku.py)

Do not add Anime Tosho. Its official shutdown notice says new ingestion ended in May 2026. It says the existing site and API will continue only for several months. [Anime Tosho shutdown notice](https://animetosho.org/about/shutdown)

Do not add BetaSeries without a product and legal decision. Its terms allow API use, but prohibit subtitle downloads in paid applications. [BetaSeries terms](https://www.betaseries.com/legal/terms)

Do not add Bazarr providers that require scraped HTML, copied browser cookies, user-agent impersonation, tracker passkeys, or undocumented endpoints. Bazarr's own provider settings show these requirements. [Bazarr provider list](https://github.com/morpheus65535/bazarr/blob/da73aeaf5e4d89ad86c8d559d3abd0e4129b24b2/frontend/src/pages/Settings/Providers/list.ts)

## Acceptance and quality assurance

The implementation is ready only when these checks pass:

1. Every catalog entry canonicalizes and renders with a CLDR display name.
2. Every provider mapping has an exact positive test and an unsupported negative test.
3. `pt-BR`, `pt-PT`, `zh-Hans`, `zh-Hant`, `es-419`, and `sr-Latn` pass end-to-end contract tests.
4. Malformed, unknown, duplicate, oversized, and conflicting preferences cause no settings write or provider call.
5. Multi-language maintenance writes distinct sidecars and reports readiness per language.
6. A missing exact script or region does not use a broad result.
7. SubSource downloads retain byte-for-byte subtitle content.
8. Each operation stays request-bounded. Shared `429` backoff for SubDL and SubSource remains a pre-existing provider-resilience follow-up.
9. The picker works with keyboard input, screen readers, long names, right-to-left names, and 320-pixel reflow.
10. Populated browser checks cover no providers, one provider, mixed support, unavailable languages, and saved multi-language profiles.
11. Live credential smoke tests remain a separate provider-availability boundary when private credentials are available.
12. Repository checks remain separate from live provider availability and physical-client playback proof.

## Result

This work targets language-identity parity: catalog breadth, canonical tags, provider maps, and multiple preferences. It does not claim full Bazarr profile parity because roles remain separate.

The immediate work is:

1. Replace free-form input with the common-first full catalog.
2. Replace one language with an ordered bounded preference list.
3. Preserve the verified SubDL map. Extend provider maps only from authoritative code data.
4. Add full negative API and web coverage.
5. Run populated responsive and provider-contract quality assurance.
