# Kinosail documentation information architecture

**Status:** Architecture guidance for the public documentation site
**Reviewed:** 2026-08-28
**Scope:** Information architecture, writing, accessibility, screenshots, GitHub Pages, and maintenance

This note defines the structure for Kinosail documentation. It is a content and publishing plan, not a user guide. The future site should use the Kinosail visual contract: calm, cinematic where media gives it a reason, dense enough to scan, explicit about privacy and recovery, and usable without cloud services.

## Decisions

### Organize by user need

Use the four Diátaxis forms as the top-level content model:

| Form | User need | Kinosail examples |
| --- | --- | --- |
| Tutorials | Learn through a safe, guided success | Set up Kinosail and play your first item |
| How-to guides | Complete a known task | Add a library, enable remote access, restore a backup |
| Reference | Look up exact facts | Configuration keys, API endpoints, supported media, environment variables |
| Explanation | Understand why a behavior or design exists | Direct media, privacy boundary, playback modes, one-container architecture |

Keep one dominant form on each page. A tutorial can link to an explanation, and a how-to can link to reference, but neither should become a mixed manual. Diátaxis defines these forms by the user’s need and says reference structure should mirror the product structure. See [Diátaxis in five minutes](https://www.diataxis.fr/start-here/), [Reference](https://www.diataxis.fr/reference/), and [the tutorial/how-to distinction](https://www.diataxis.fr/tutorials-how-to/).

### Use a task-first site map

Use this initial tree. Keep labels short and use stable, human-readable URL slugs.

```text
Home
├── Get started
│   ├── Choose an installation path
│   ├── Install Kinosail
│   ├── Complete first-run setup
│   └── Play your first item
├── Guides
│   ├── Libraries and media
│   ├── Browse, search, and collections
│   ├── Playback and subtitles
│   ├── Users, owners, and sharing
│   ├── Remote access and HTTPS
│   ├── Backups and recovery
│   └── Integrations
├── Understand Kinosail
│   ├── Local-first privacy
│   ├── Direct media and playback fallback
│   ├── Server architecture
│   └── Permissions and account security
├── Reference
│   ├── Configuration
│   ├── HTTP API
│   ├── Media and client compatibility
│   ├── Deployment and container settings
│   └── Error and status reference
├── Troubleshoot
│   ├── Installation and startup
│   ├── Library scanning
│   ├── Playback
│   ├── Authentication and access
│   └── Logs and diagnostics
└── Contribute
    ├── Development setup
    ├── Testing
    ├── Documentation standards
    └── Release process
```

The home page should route readers by intent: **Install**, **Use**, **Understand**, **Look up**, or **Fix a problem**. Do not make a search box the only way to reach content. Use visible links between related pages, because Google Search guidance says that navigation and cross-page links help search systems understand site structure. See [Google’s site-structure guidance](https://developers.google.com/search/docs/specialty/ecommerce/help-google-understand-your-ecommerce-site-structure).

### Make every page independently useful

Each page should have:

1. A unique title and one level-one heading.
2. A one-sentence purpose statement and the expected result.
3. Prerequisites, when the task has them.
4. A short procedure or reference table.
5. Expected output, recovery, and next steps.
6. Links to the parent section and related content.
7. A visible “last reviewed” date and an owner or source of truth for volatile facts.

Use sentence case. Use imperative, bare-infinitive headings for tasks, such as “Add a library.” Use noun phrases for concepts, such as “Direct media.” Keep heading levels hierarchical and do not skip levels. This follows [Google’s headings and titles guidance](https://developers.google.com/style/headings) and [WAI’s headings guidance](https://www.w3.org/WAI/tutorials/page-structure/headings/).

## Navigation and visual system

### Navigation model

Provide the same navigation spine on every page:

- Skip link to the main content.
- Site name and home link.
- Primary section navigation.
- Breadcrumbs showing the current location.
- In-page table of contents for long pages.
- Previous and next links within a guide sequence.
- Related links at the end of the page.
- A visible search control that complements, rather than replaces, links.

Breadcrumbs should represent the content hierarchy, not the reader’s click history. Google describes a breadcrumb trail as a way to show a page’s position in the hierarchy and help readers move upward one level at a time. See [Breadcrumb structured data](https://developers.google.com/search/docs/appearance/structured-data/breadcrumb).

Use Kinosail’s existing role tokens, local system font stack, restrained signal color, readable long-form measure, and subtle dividers. Reuse the app’s calm dark/light themes. Do not copy media artwork into every page or turn documentation into a generic SaaS dashboard. Screenshot placeholders must occupy the same deliberate media role that the final image will occupy.

### Screenshot placeholder pattern

Until the app is complete, use a stable placeholder that preserves layout without implying that the state is real:

```markdown
> **Screenshot placeholder — replace before release**
>
> Capture: `settings/library-add`
>
> Viewport: desktop 1440 × 900; also capture compact mobile.
>
> Show: the library form with a valid local path and the successful save state.
>
> Do not show: hostnames, usernames, tokens, private media names, or real artwork.
>
> Alt text draft: “The Add library form shows a local media path, scan options, and a Save library button.”
```

Store future images in a predictable path such as `assets/screenshots/<section>/<slug>/`. Give each image a meaningful filename, an adjacent caption, and alt text that conveys its purpose. Decorative images use empty alternative text. Functional images describe the action, not their appearance. Informative images describe the essential information, not every pixel. See [WAI’s Images Tutorial](https://www.w3.org/WAI/tutorials/images/) and [Informative Images](https://www.w3.org/WAI/tutorials/images/informative/).

## Writing standard

Use Simplified Technical English for Kinosail docs:

- Put the outcome and important condition first.
- Use second person and active voice.
- Use short sentences and one idea per sentence.
- Define an acronym at first use.
- Use numbered steps for an ordered procedure.
- Use bullets for unordered choices.
- Use code formatting for commands, paths, keys, and API values.
- Use meaningful link text. Never use “click here.”
- State whether an action changes data, restarts the server, requires owner access, or sends data off the local server.
- Use examples that are safe to copy and clearly mark placeholders.

Google’s style guidance recommends conversational, respectful language, active voice, second person, sentence-case headings, descriptive links, short sentences, and a first-sentence summary. See [Google’s style highlights](https://developers.google.com/style/highlights), [voice and tone](https://developers.google.com/style/tone), and [accessible documentation](https://developers.google.com/style/accessibility).

## Accessibility baseline

Target WCAG 2.2 Level AA for the documentation site and its examples. Treat the following as release requirements:

- Use semantic landmarks and a logical heading hierarchy.
- Give every page a useful title and language declaration.
- Provide a skip link and keyboard access to navigation, search, tabs, forms, and code-copy controls.
- Keep focus visible and never hide focused content behind fixed navigation.
- Use descriptive link text and distinguish downloads or new tabs.
- Do not use color alone to communicate status or meaning.
- Keep pointer targets at least 24 × 24 CSS pixels under WCAG 2.2, with the app’s 44 × 44 coarse-pointer target convention where space permits.
- Provide text alternatives for informative and functional images; use `alt=""` for decorative images.
- Support reflow, zoom, reduced motion, high contrast, and forced-colors modes.

The normative criteria are [WCAG 2.2](https://www.w3.org/TR/WCAG22/), especially headings and labels, focus visibility, focus not obscured, consistent navigation, and target size. WAI’s [WCAG 2.2 understanding pages](https://www.w3.org/WAI/WCAG22/Understanding/) provide implementation context. Google’s accessibility guidance also recommends keyboard and screen-reader testing, short scannable content, meaningful links, and useful alt text.

## GitHub Pages publishing constraints

Treat the docs site as a public, static artifact:

- Never commit secrets, private hostnames, access URLs, user data, real library paths, or unlicensed artwork.
- Keep the site within GitHub’s published-site recommendation of 1 GB. Avoid bundling videos or full-resolution media; link to release assets when needed.
- Keep builds below GitHub’s 10-minute deployment timeout and account for the soft build and bandwidth limits.
- Use a supported Jekyll/GitHub Pages setup, or publish prebuilt static HTML when the site does not need Jekyll features.
- Use `url`, `baseurl`, and Jekyll’s `relative_url` filter so project pages work under a repository subpath.
- Use front matter with a title, layout, description, and stable permalink for each page.
- Test the exact publishing source locally with Bundler and `bundle exec jekyll serve` before publishing.
- Provide a custom 404 page with links to the home page, search, and major sections.

GitHub states that Pages sites are public on the internet, recommends local Jekyll testing, and documents project-page `baseurl` behavior. See [creating a GitHub Pages site with Jekyll](https://docs.github.com/en/pages/setting-up-a-github-pages-site-with-jekyll/creating-a-github-pages-site-with-jekyll), [adding content](https://docs.github.com/en/pages/setting-up-a-github-pages-site-with-jekyll/adding-content-to-your-github-pages-site-using-jekyll), [testing locally](https://docs.github.com/en/pages/setting-up-a-github-pages-site-with-jekyll/testing-your-github-pages-site-locally), and [Pages limits](https://docs.github.com/en/pages/getting-started-with-github-pages/github-pages-limits).

## Maintenance and validation

### Content ownership

Keep a source-of-truth field in page front matter or an adjacent inventory. Mark facts that can drift, such as supported versions, configuration keys, ports, API routes, and client behavior. Update those pages in the same change as the implementation. Prefer generated reference material where the repository already exposes a stable source, because Diátaxis notes that generated reference can stay faithful to the software.

Use a lightweight review record for each change:

```yaml
last_reviewed: 2026-08-28
review_owner: maintainers
source_paths:
  - internal/server/
  - docs/
verification:
  - jekyll build
  - link checker
  - keyboard and screen-reader smoke test
```

### Release checklist

Before publishing a docs revision:

1. Build the site with the same Jekyll/GitHub Pages dependency set used for deployment.
2. Fail on broken internal links, missing images, duplicate titles, invalid front matter, and orphan pages.
3. Inspect representative pages at wide, tablet, compact mobile, and 320-pixel widths.
4. Test keyboard-only navigation, visible focus, skip link, search, breadcrumbs, code blocks, and 200% zoom.
5. Run an accessibility audit and manually inspect heading order, link purpose, language, alt text, contrast, and reduced motion.
6. Review every screenshot placeholder. Replace or explicitly defer it before calling the page release-ready.
7. Check commands against a clean supported installation and label version-specific behavior.
8. Review the generated site for secrets, private identifiers, and stale claims.

The published site is the final evidence. A successful Markdown build alone does not prove navigation, visual composition, responsive behavior, or accessibility.

## Open implementation questions

Resolve these before the site is published:

- Which Jekyll theme or local layout will express Kinosail tokens without adding a runtime dependency?
- Which search implementation works with static Pages and does not send private content to a third party?
- Which pages require generated API/configuration reference, and what command produces it?
- Which supported installation variants need separate tutorials or a decision page?
- Which browser and screen-reader matrix will be the documented accessibility gate?
