# Documentation preparation — September 15, 2026

## Scope

Prepare the root/app READMEs and GitHub-facing docs for a reader who has not seen the project before. Cover app choice, requirements, a complete first start, configuration, persistence, recovery, support, contribution, and accurate release boundaries. Preserve runtime behavior, release identities, licensing terms, gate policy, and unrelated work.

This is documentation preparation. It does not certify an application release or publish a documentation website.

## Patterns checked against similar projects

- [Jellyfin's README](https://github.com/jellyfin/jellyfin/blob/master/README.md) leads with product purpose and clear routes to installation, help, and contribution. Apply that pattern to Kinosail's root app chooser and entry links.
- [Jellyfin documentation](https://jellyfin.org/docs/) separates getting started, administration, clients, and contribution. Keep user tasks separate from engineering policy and source reference.
- [Navidrome installation](https://www.navidrome.org/docs/installation/) makes host/install choices and required tools explicit. State the app directory, container engine, media access, and startup address beside each Kinosail command.
- [Immich's quick start](https://docs.immich.app/overview/quick-start/) provides a short path through prerequisites, configuration, launch, and first use. Give Kinosail readers one complete local start before optional network/integration detail.

These are information-architecture patterns. Their features, licensing, hardware requirements, release claims, and deployment commands are not Kinosail facts.

Provider setup also uses the official [SubDL API guide](https://subdl.com/api-doc), [OpenSubtitles getting-started guide](https://opensubtitles.tawk.help/article/getting-started), and the SubSource account/documentation URLs already used in Kinosail's setup UI. Provider plans and quotas are deliberately not promised by the docs.

## Corrections made

- Replaced the root's repository-only summary with app selection, source quickstarts, first-run outcomes, requirements, persistent-storage warnings, release availability, documentation links, and development entry points.
- Added monorepo contribution, security, and support entry points, issue templates, and a PR template that recognizes the separate restricted public HTTPS gateway.
- Removed the Player README's stale live-TV claim and the app security docs' stale enabled-by-default Jellyfin and running release-CI claims.
- Made source backup-key configuration explicit. Source Compose does not mount the release installer key; CLI backups without a key are unencrypted portable archives, while automatic backups require encryption.
- Corrected backup filenames, added explicit verification before restore, and distinguished app state from external configuration, secret material, media, and subtitle sidecars.
- Expanded Dashboard first setup, HTTPS/public-origin configuration, data protection, updates, and troubleshooting.
- Corrected native README distribution state against the current `implemented` Info-plist value while preserving device/signing acceptance boundaries.
- Replaced copied Player workflows in the Subtitles guide with provider setup, coverage, matching, maintenance, sidecar protection, inspection, recovery, API, and MCP instructions.
- Documented that the supplied Compose files do not forward SubSource variables; Owner Settings or an explicit deployment override is required.
- Added the missing configuration-reference entries: the documented key inventories now match the 65 Player and 78 Subtitles typed source settings. Application variables and Compose interpolation variables are distinguished.
- Fixed internal site links and GitHub edit/issue links for the monorepo. Removed unfilled screenshot placeholders from user-facing pages while retaining the screenshot include and historical planning notes.
- Added a shared pinned Jekyll/Bundler environment for reproducible documentation rendering. Hosting origin and base path remain deployment choices; no live site is asserted.

## README inventory

All 20 tracked READMEs present at the start were reviewed. Nineteen were updated; the dated dependency-provenance record was retained. Added three entry points for shared packages, cross-app engineering, and the documentation renderer.

| README | Role and disposition |
| --- | --- |
| `README.md` | Root app chooser and complete getting-started path. Rewritten. |
| `.codex/skills/README.md` | Agent-tooling entry point. Added scope and root navigation. |
| `apps/player/README.md` | Player setup and operations. Corrected current behavior and prerequisites. |
| `apps/subtitles/README.md` | Subtitles setup and operations. Expanded permissions, provider wiring, first success, and recovery. |
| `apps/dashboard/README.md` | Complete Dashboard operator entry point. Expanded. |
| `apps/player/apps/native/README.md` | Swift client connection/build/distribution boundary. Corrected. |
| `apps/player/apps/native/patches/README.md` | Retained Expo patch reference. Explicitly marked legacy. |
| `apps/player/docs/README.md` | User-guide map and reproducible local rendering. Expanded. |
| `apps/subtitles/docs/README.md` | Subtitle guide map and reproducible local rendering. Expanded. |
| `apps/player/engineering/README.md` | Actual architecture/release/research navigation. Expanded. |
| `apps/subtitles/engineering/README.md` | Actual architecture/release/research navigation. Expanded. |
| `apps/dashboard/engineering/README.md` | Removed references to nonexistent docs/ADR/checklist locations. |
| `apps/player/engineering/architecture-explorer/README.md` | Monorepo generator context and publication boundary. Clarified. |
| `apps/subtitles/engineering/architecture-explorer/README.md` | Monorepo generator context and publication boundary. Clarified. |
| `apps/player/engineering/design/supporter-badge-vector-samples/README.md` | Working directory, design-study scope, and gate policy. Clarified. |
| `apps/subtitles/engineering/design/supporter-badge-vector-samples/README.md` | Working directory, design-study scope, and gate policy. Clarified. |
| `apps/player/engineering/design/supporter-collections/README.md` | Existing complete study retained; command context and gate policy clarified. |
| `apps/player/testdata/README.md` | Fixture provenance, links, disposable output, and gate policy. Expanded. |
| `apps/subtitles/testdata/README.md` | Fixture provenance, links, disposable output, and gate policy. Expanded. |
| `engineering/research/dependency-provenance/README.md` | Dated technical evidence. Reviewed and retained without rewriting historical claims. |

## Source and artifact evidence

- Current Compose files, example configuration, runtime configuration definitions, backup commands, subtitle route/authentication policies, Dashboard setup/configuration, native Info plists, Makefiles, installer, and release checklist were read for the affected instructions.
- `gh release list --repo MikeO7/kinosail --limit 5` returned no releases on this date. Release availability is recorded as a dated observation.
- Both sites rendered with the locked Jekyll 4.4.1 bundle under distinct nonempty base paths (`/preview/player` and `/preview/subtitles`). The render produced 78 HTML files across both outputs.
- Inspection of rendered local link targets and fragment anchors found no unresolved destinations. Configuration inventory review found no missing or extra source-defined keys.
- Manual browser review confirmed Player backup search and navigation, correct GitHub issue/edit URLs, Subtitles mobile-menu navigation, and provider-table containment at 390 pixels in light and dark themes. The provider page had equal document/client widths of 390 pixels. Keyboard Tab then Enter on the install page moved focus through the skip link to `main-content`. The root README rendered with the GFM parser and its Getting started anchor navigated correctly; this is local preview evidence, not an already published GitHub render.

## Remaining release boundaries

Application/container/browser quality suites were not run while `.gates-disabled` was present. No app install, provider download, destructive restore, physical-device acceptance, signed release, or production deployment was performed by this documentation task. Those commands were reviewed against source; they were not all executed on a clean production host.

GitHub Actions and signed-release publishing remain blocked as described by the Player release checklist. The docs now surface that limitation rather than advertising unavailable downloads. Hosting a public documentation site requires a chosen origin/path and publication authorization; the local renderer does not enable Pages or change repository visibility.
