# Kinosail UI / UX reference — 2026

**Research checked:** 6 September 2026\
**Purpose:** a working reference for ambitious redesigns across the Kinosail family.\
**Companion:** The implementation prototype was retired after the product design work.\
**Next evidence review:** before the next substantial redesign, and by 6 December 2026 for platform and browser claims.

## Start here

Kinosail should be beautiful, distinctive, immediately understandable, and satisfying to use. A technically functional interface with generic composition is insufficient. A fashionable interface that makes the task harder is also insufficient.

The user explicitly authorized a cutting-edge redesign, including changes to branding, layout, navigation, components, and existing design rules. **No current aesthetic is sacred.** The proposals below are revisable. They are not a mandate to preserve the present green palette, rail, cards, typography, density, or implementation if a better design warrants changing them.

Keep user intent, data integrity, accessibility, privacy, and truthful feedback intact. Retain independent app ownership and deployment, API parity, and Direct First playback unless the requested behavior explicitly changes. These are product responsibilities, not restrictions on visual ambition.

Use this reference in four steps:

1. Identify the user's task, the important decision, and the primary device/input method.
2. Consult the evidence ledger and control decision table. Separate findings from design hypotheses.
3. Produce at least two meaningfully different compositions for a substantial new direction, using the same realistic content. Compare hierarchy, character, usefulness, and cost.
4. Implement the stronger direction, exercise real interactions, and record what was actually verified. Update the affected `DESIGN.md` with the resulting decisions.

The companion is a **reference prototype**, not a production component library or a claim that the apps already implement the design. Its sample states are explicitly illustrative.

## What counts as evidence

| Label | Meaning | How to use it |
| --- | --- | --- |
| Standard | A published normative requirement, such as WCAG success criteria | Establish acceptance criteria; distinguish conformance levels and exceptions. |
| Empirical | A study or synthesis with an inspectable method | Use its findings within its sample, tasks, devices, and limitations. |
| Platform | Guidance from the owner of a design system or platform | Adapt to the actual platform; it is not a universal empirical law. |
| Proposal | A Kinosail design decision inferred from evidence and product needs | Prototype, measure, critique, and revise. |
| Experiment | A promising technique with unsettled usability or support | Isolate it, provide equivalent fallback behavior, and test before adoption. |

This was a targeted primary-source review, not a systematic review of the entire HCI literature. It prioritizes relevant 2025–2026 publications, current standards, and current first-party guidance. Foundational research remains useful, but an old publication is not described as new because a search engine crawled it recently. Abstracts, full texts, and vendor reports are distinguished below.

## Research ledger

### E1 · Expressive design can improve attention and action speed

**Bentley et al., CHI 2026, 13–17 April. Empirical; full author manuscript inspected.** Forty-eight adults compared ten pairs of mobile application screens in a randomized within-participant experiment. Expressive variants improved mean fixation time by 33%, single-tap task time by 20%, and aesthetic ratings.

**Boundary:** one laboratory Android device, single-tap tasks, participants aged 18–62 in Chicago. Multiple visual properties changed together. This does not isolate the effect of rounded buttons, validate whole workflows, or establish Kinosail, desktop, TV, or long-term gains.

**Kinosail proposal:** use strong differences in size, type, color, shape, and grouping to direct attention to the important action. Compare the primary task and secondary-task discoverability before shipping.

Sources: [author manuscript](https://frankbentley.com/wp-content/uploads/2026/03/chi26-110-cr.pdf), [publication record](https://research.google/pubs/usability-hasnt-peaked-exploring-how-expressive-design-overcomes-the-usability-plateau/).

### E2 · Beauty deserves evaluation alongside performance

**Schlamann, Nestler & Thielsch, IJHCI, 22 June 2026. Empirical synthesis.** Preregistered meta-analysis: 31 studies, 234 effects, 18,794 participants. The average performance effect favored more attractive functional interfaces: Hedges' g = 0.29, 95% CI [0.08, 0.51]. Heterogeneity was high (I² = 89.68%); the prediction interval [−1.07, 1.66] includes negative effects.

**Boundary:** a new synthesis of studies from 2001–2025, not exclusively new experiments. Designs and measurements vary; no particular aesthetic is validated. Publisher-indexed methods and results were accessible; direct full-page retrieval was blocked.

**Kinosail proposal:** evaluate visual appeal and distinctiveness explicitly, while checking task success, errors, and recovery. Do not infer usability from beauty alone.

Sources: [paper](https://www.tandfonline.com/doi/full/10.1080/10447318.2026.2664081), [open supplementary materials](https://zenodo.org/records/18777943).

### E3 · AI authorship does not establish design quality

**Romero et al., SEMISH 2026; arXiv version 14 May. Empirical; full HTML inspected.** Ninety-two students rated ten static desktop prototypes with authorship concealed: five AI-generated and five created by one researcher. Practical UX ratings were generally stronger than originality-related ratings.

**Boundary:** static images, one educational planning scenario, predominantly male computer-science students, one human designer, and no objective interaction task. The paper also reports presentation order inconsistently. It does not prove that AI interfaces work well or that human-designed ones are better.

**Kinosail proposal:** compare alternative art directions on their merits. Reject generic composition even when source tests pass; reject attractive compositions when task evidence fails.

Sources: [full text](https://arxiv.org/html/2605.15124v1), [publication status and proceedings DOI](https://arxiv.org/abs/2605.15124).

### E4 · Accessibility must include the interaction after activation

**Huq et al., CHI 2026, 13–17 April. Empirical; full paper inspected.** The authors reviewed 31 papers and conducted 20 app-testing sessions with **13 distinct blind TalkBack users** across four Android apps. Findings include feedback-related failures and interacting problems involving labels, navigation, activation, and dynamic changes.

**Boundary:** selected Android apps, some injected defects, and North American participants. Findings are not prevalence estimates or web/TV certification. The authors' MCAG framework is a research proposal, not a W3C standard.

**Kinosail proposal:** test whether a user can identify the control, activate it, understand the result, and recover. A named button or clean axe report alone is insufficient.

Sources: [author-hosted paper](https://ics.uci.edu/~seal/publications/2026_CHI.pdf), [proceedings DOI](https://doi.org/10.1145/3772318.3791293).

### E5 · More space helps some users but can also add effort

**Wickramathilaka et al., Automated Software Engineering, 11 August 2025. Empirical; version of record inspected.** AdaptForge was evaluated with 18 developers and three focus groups involving 22 older adults, mean age 72.1, using video demonstrations. Participants favored readable text, stronger boundaries, and staged forms, while identifying extra scrolling and concerns about voice interaction.

**Boundary:** prototype reactions, not measured hands-on task improvements; Australian community sample and Flutter-specific tooling.

**Kinosail proposal:** provide readable text and generous targets, but measure information access and scrolling. Offer user-controlled density when warranted. Do not infer that all older people need wizards, oversized cards, or voice controls.

Source: [journal article](https://link.springer.com/article/10.1007/s10515-025-00547-z).

### E6 · Current expressive design guidance is broader than visual minimalism

**Google Material 3 Expressive, introduced May 2025. Platform/vendor research.** Google's report describes 46 studies and more than 18,000 participants across its development program. Its tactics use hierarchy, containment, shape, color, and motion. Its own counterexamples show that unlabeled actions and an unfamiliar music-list arrangement can damage usability.

**Boundary:** the program-level report is not a single controlled experiment or independent replication. Do not combine its “up to” figures with E1's averages.

**Kinosail proposal:** create recognizable, emotionally engaging task compositions. Preserve intelligible labels and relationships while exploring a bolder visual vocabulary.

Source: [Google design research report](https://design.google/library/expressive-material-design-google-research).

## What is current, and what is merely fashionable

| Topic | Verified guidance as of 6 September 2026 | Kinosail decision |
| --- | --- | --- |
| Connected button groups | Material's May 2025 expressive update replaces its earlier segmented-button recommendation with connected groups, with selected/pressed shape behavior. Its catalog lists expressive web implementation as unavailable. | Use the visual idea where appropriate; preserve native radio or checkbox semantics on the web. Do not install a framework simply to imitate it. |
| Apple segmented controls | Apple's guidance still supports related choices and view changes, with platform-specific behavior. | Do not describe all segmented controls as obsolete. Choose according to context, labels, input, and platform. |
| Split buttons | Material includes a primary action with a related options trigger. | Useful when Play or Resume has meaningful alternatives. Keep the main action directly available and independently named. |
| Liquid Glass | Apple's current design separates controls/navigation from content and responds to accessibility preferences. | Consider on appropriate native chrome or overlays. Guarantee text contrast; provide opaque/reduced-transparency treatment. A blur behind every settings section is not a requirement. |
| WCAG 2.2 | Current published Recommendation dated 12 December 2024. | Use applicable AA success criteria as the web baseline, and stronger product targets where useful. |
| WCAG 3.0 | Current published Working Draft dated 3 March 2026. | Track research and evolution; do not claim WCAG 3 compliance. |

Sources: [Material segmented buttons](https://m3.material.io/components/segmented-buttons/overview), [button groups](https://m3.material.io/components/button-groups/overview), [split buttons](https://m3.material.io/components/split-button), [Apple segmented controls](https://developer.apple.com/design/human-interface-guidelines/segmented-controls), [Apple Liquid Glass](https://developer.apple.com/documentation/technologyoverviews/liquid-glass), [Apple materials](https://developer.apple.com/design/human-interface-guidelines/materials), [WCAG 2.2](https://www.w3.org/TR/WCAG22/), [WCAG 3 draft](https://www.w3.org/TR/wcag-3.0/).

Material's JavaScript-rendered segmented/button-group pages were read in a live browser because the text retrieval returned only the site shell. Platform guidance can change; verify these links before asserting that a component is deprecated.

## Choose controls by the decision they represent

“Dropdown” is ambiguous: a form selection, command menu, autocomplete, and navigation disclosure are different interactions. Establish which problem is being solved before replacing it. Counts below are **starting heuristics**, not accessibility rules or findings of a universal optimum.

| User decision | Preferred starting point | Avoid / reconsider | Required behavior |
| --- | --- | --- | --- |
| One of roughly 2–5 short related choices | Connected choice group; native radios underneath | Hiding ordinary choices in a menu; forcing long labels into equal tiny segments | One selected value; group name; arrow-key behavior; selection distinguishable from focus. |
| One of a few consequential policies | Descriptive choice rows/cards with concise effects | A bare dropdown that hides consequences | One selection; full label target; effects and current value visible. |
| Several independent selections saved together | Checkbox rows, or checkbox-based filter chips | Switches that imply an immediate save | Each choice independent; clear Apply/Save timing; mixed state only when meaningful. |
| Immediate binary setting | Labeled switch | A switch followed by an unexplained Save requirement | Stable label; on/off state; pending feedback; server failure restores truth. |
| A small compact selection with long/dynamic labels | Native select or platform picker | Expanding dozens of options into permanent cards | Real label; usable selected value; type-ahead where supported; keyboard and mobile operation. |
| A long searchable set | Searchable combobox or focused search-and-select dialog | A scrolling wall of chips; unvalidated free text | Query distinct from chosen value; no-results and loading; stale-query protection; Escape and focus recovery. |
| Several choices from a long set | Searchable multi-select with visible removable selections | Hiding all chosen values behind a count | Readable selection summary; accessible remove controls; no accidental clearing. |
| A reversible view filter | Selection chips or a connected group | Giving navigation links button semantics, or changing layout order unpredictably | Visible active filters; clear reset; sensible focus after filtering. |
| Main action with related variants | Primary button plus separately named options trigger | Making the main action open a menu; nesting controls inside a button | Main action works in one activation; options are secondary; menu closes predictably. |
| Related commands | Menu button; native popover/dialog where suitable | Using a form select to execute verbs | Visible label where needed; keyboard navigation; Escape; return focus. |
| App destinations | Links, tabs, rail, or bottom navigation appropriate to platform | One giant overflow menu; gesture-only destinations | Current location; browser Back/URL behavior; recognizable labels. |
| Numeric value requiring precision | Number/text field with units; stepper if useful | Slider alone for exact bandwidth, delay, or limits | Server range/format validation; value and units; keyboard entry. |
| Approximate bounded value | Slider with visible value and keyboard control | Unlabeled slider; tiny thumb as the only target | Named range; bounds; increments; value text; optional exact entry. |
| Dates and schedules | Native picker or structured date/time entry | Separate menus for every date part without task justification | Locale clarity; timezone; keyboard entry; invalid/conflicting range errors. |
| Optional technical detail | Disclosure/details or a contextual panel | Hiding prerequisites, errors, or essential actions | Honest summary; expanded state; no data loss on collapse. |
| Destructive action | Explicit action, clear scope, confirmation where consequences warrant it | Confirmation for every harmless action; misleading Undo | Server authorization; recovery when real; describe what will be removed. |
| Reordering | Drag plus explicit move controls | Drag-only or hover-only editing | Keyboard/remote alternatives; preserved focus; saved-order/error announcement. |
| Waiting or background work | Local pending state, named progress, cancellation where possible | Fake percentages, blocking skeletons after content is usable, layout jumping | Truthful state; repeated-submission protection; retry and recovery. |

The switch/save distinction is explicit in [Fluent 2 switch guidance](https://fluent2.microsoft.design/components/web/react/core/switch/usage). Its [dropdown](https://fluent2.microsoft.design/components/web/react/core/dropdown/usage) guidance favors native selects for many mobile forms; [combobox guidance](https://fluent2.microsoft.design/components/web/react/core/combobox/usage) discusses searchable sets and showing selections. Use the [WAI radio](https://www.w3.org/WAI/ARIA/apg/patterns/radio/), [combobox](https://www.w3.org/WAI/ARIA/apg/patterns/combobox/), and [switch](https://www.w3.org/WAI/ARIA/apg/patterns/switch/) patterns for implementation details. These are guidance, not substitutes for actual assistive-technology testing.

### Semantics stay independent of styling

A connected group can look new while remaining a native radio group. A broad checkbox row can be easier to tap without becoming a switch. A custom-looking select can still be a real select. Do not use `role="menu"` for ordinary site links or `role="button"` on an element that should be a link. Prefer existing native semantics before constructing keyboard behavior manually.

For a custom combobox, verify typing, text editing, arrow navigation, Enter, Escape, pointer selection, screen-reader output, loading, no results, and invalid values. On the server, validate the selected identifier against current allowed values. A polished client control never weakens the trust boundary.

## Proposed visual direction

**Proposal, not an empirical finding:** expressive, cinematic, precise. The interface should have authored character without resembling a template assembled from interchangeable cards.

### Composition

- Give each screen a dominant purpose: resume media, select a service, resolve a subtitle exception, request a title, or configure a policy.
- Use asymmetric compositions when they clarify priority. A large title or artwork region needs a task reason, not a blanket prohibition or blanket permission.
- Expose essential actions near their object. Secondary information can be quiet without becoming undiscoverable.
- Use deliberate containment: a primary decision can occupy a strongly differentiated surface; repeated content can use open rows or shelves. Neither “everything is a card” nor “all cards are bad” is a rule.
- Build compact layouts as compositions in their own right. Keep the same task and information relationships, while changing arrangement, containment, and action placement.

### Type, color, shape, and material

The following are **prototype starting values**, not frozen global requirements:

| Role | Starting treatment | Verification |
| --- | --- | --- |
| Main task title | Fluid 32–64px, weight around 600–700, tight but readable tracking | Two-line titles and localized expansion; first useful action remains reachable. |
| Body / form text | 15–17px, line height 1.45–1.6 | Real long copy, user text scaling, 200% zoom. |
| Metadata | 12–14px; tabular numbers for operational facts | Contrast and readable density; do not shrink to repair overflow. |
| Primary action | Clearly stronger than neighbors through size, fill, or shape | At most one dominant action per immediate decision; no color-only meaning. |
| Connected choices | Shared geometry, stronger selected surface, check/state marker | Focus remains distinct; label length does not collide with state marker. |
| Palette | Deep neutral canvas, high-contrast text, one deliberate accent family | Dark/light/state contrast; real artwork; color-vision and forced-color modes. |
| Shape | Small radii for dense rows; larger radii for prominent touch controls or media | Radius communicates hierarchy; consistent nesting and clipping. |
| Material | Opaque reading surfaces; selective translucent control layer over media | Stable contrast, reduced transparency, low-powered devices. |

The current Kinosail signal palette is a useful prototype starting point, not a protected brand constraint. Explore alternatives against the same media and operational content. A new font must have a product-wide role, appropriate license, local hosting, and a loading/fallback plan. Typography is not a reason to add a third-party request to a private media app.

### Motion

Use motion to make state changes legible: selection, expansion, focus movement, reordering, and continuity from item to detail. Prototype around 120–200ms for repeated feedback and 180–280ms for a panel or spatial transition; these are tuning ranges, not scientific thresholds.

Never delay the underlying action until decoration finishes. Keep repeated controls spatially stable. Honor reduced motion and do not use perpetual ambient animation. A restrained shape change on a connected choice can communicate selection; a moving target can also impair acquisition, so test the actual implementation. Keep pending and error states intelligible with animation disabled.

## Surface recipes for the family

| Surface | Direction to explore | What success looks like |
| --- | --- | --- |
| Player home / browse | A clearly authored resume composition when progress exists; rich artwork, legible media names, focused shelf hierarchy; visibly accessible search/filtering | People resume or find a title quickly; empty/missing-art states remain intentional. |
| Player detail / playback | Strong title/artwork relationship; one Play/Resume action; secondary options nearby; chapter/episode structure easy to scan | Playback dominates; tracks, device routing, recovery, and policy remain understandable. |
| Native Player | Platform-adapted controls, typography, safe areas, and navigation; meaningful tablet split compositions and remote focus | Same task quality on phone, tablet, desktop, and TV; web rendering is not device proof. |
| Dashboard | A confident household service board with compact status, readable destinations, and direct editing; compare tile and list density | Primary services appear early; no fake monitoring metrics; complete keyboard reordering. |
| Subtitles | Coverage and exceptions lead; language priorities and provider state close to the work; concise action feedback | Owners understand what is missing, what will run, and whether files changed. |
| Supporter surfaces | Beautiful ownership and badge presentation tied to real entitlement; clear selected/locked/available states | Recognition is understandable and private; appearance never implies ownership that does not exist. |
| Home Assistant integration | Native Home Assistant selectors, field copy, and state feedback; improve integration-owned flows | Setup errors are actionable; the integration fits its host. Replacing the host's entire visual system is a separate product decision. |

Repository boundaries verified during this review: Player/Subtitles/Dashboard and native Player live here; Supporter is an API service with visible surfaces in consuming apps; Home Assistant provides the integration's host UI. Recheck this map before future work if repository ownership changes.

## Modern web techniques worth evaluating

| Technique | Current evidence | Adoption rule |
| --- | --- | --- |
| Native customizable select | Browser support remains limited; `appearance: base-select` can customize real select controls. | Progressive enhancement only. Preserve a usable native fallback and verify the actual browser/assistive-technology combination. |
| Same-document View Transitions | Core support reached Baseline Newly available in October 2025; subfeatures differ. | Feature-detect; update state whether or not animation runs; honor reduced motion. Do not assume cross-document or newer subfeature parity. |
| Native popover / dialog | Platform primitives can reduce custom overlay infrastructure. | Pick the correct modality; manage naming, focus, dismissal, and overflow. A popover does not automatically implement a menu or combobox. |
| Container queries and modern layout | Useful for components reused in sidebars, panels, and full pages. | Use available container width to change composition; retain workable fallback layout where required by the supported device matrix. |

Sources: [MDN customizable select](https://developer.mozilla.org/en-US/docs/Learn_web_development/Extensions/Forms/Customizable_select), [Web Platform DX support record](https://web-platform-dx.github.io/web-features-explorer/features/customizable-select/), [Google's same-document transition announcement](https://web.dev/blog/same-document-view-transitions-are-now-baseline-newly-available), [MDN ViewTransition](https://developer.mozilla.org/en-US/docs/Web/API/ViewTransition), [HTML popover specification](https://html.spec.whatwg.org/multipage/popover.html), [CSS containment specification](https://www.w3.org/TR/css-contain-3/).

These are implementation opportunities, not a requirement to adopt a particular framework or rewrite. A rewrite should earn its cost by improving task behavior, design consistency, maintainability, and verification.

## Acceptance and evaluation

### Three separate questions

1. **Does it look authored and compelling?** Compare hierarchy, rhythm, alignment, typography, color, material, imagery, density, and distinctiveness. Ask whether the primary task is apparent and the screen belongs to this product.
2. **Does it work better?** Observe task completion, errors, backtracking, action discovery, comprehension, and recovery. Compare the same tasks/data against the previous design; report participant count and study limitations.
3. **Can people access it?** Test semantics, focus, touch/remote operation, scaling, contrast, announcements, and full journeys with assistive technology. Automated scans are one layer.

### Reproducible engineering evidence

- Inspect populated dark and light renders around 1440×900, 1024×768, 720×450, 390×844, and 320 CSS px. These are test probes; layout breakpoints should follow content.
- Cover long titles/labels, missing artwork, empty/large data, validation errors, slow/offline responses, pending saves, repeated activation, success, and recovery.
- Check keyboard order, visible focus, Escape/Back behavior, dialogs, reduced motion, forced colors, text zoom, and reflow. Record actual browser versions.
- Use WCAG 2.2 AA criteria accurately. Its target minimum is 24×24 CSS px with exceptions; Kinosail's proposed general touch target is at least 44×44 CSS px. Android uses dp and Apple uses points: do not equate these units mechanically.
- For TV, test directional navigation, one clear focus, selection, and Back using a real remote or an explicitly identified simulator. Keep focus and selection distinct where the platform expects them.
- Exercise VoiceOver/TalkBack journeys and announce resulting state. Record real disabled-user testing separately from an agent exercising a screen reader.
- Compare loading and interaction responsiveness on representative hardware. Prefer field p75 LCP/INP/CLS when available; laboratory data is not a field distribution. Keep source tests, browser evidence, deployment revision, health, TLS, and device proof separate.

Sources: [W3C target-size explanation](https://www.w3.org/WAI/WCAG22/Understanding/target-size-minimum.html), [Android TV focus system](https://developer.android.com/design/ui/tv/guides/styles/focus-system), [Web Vitals](https://web.dev/articles/vitals).

### Design decision record

For each substantial change, leave a short record:

> **Task and audience:** …\
> **Prior problem and baseline:** …\
> **Alternatives compared:** …\
> **Chosen direction and reason:** …\
> **Evidence used:** source IDs/links and limitations …\
> **Behavior that must remain:** …\
> **Results:** rendered states, test commands, task evidence …\
> **Tradeoffs and unverified boundaries:** …\
> **Revision trigger:** what new evidence would make us change this choice …

Do not score a design “cutting edge” because it uses recent APIs, or “accessible” solely because it passes axe. Do not require preserving an old rule to pass a subjective rubric. The result must be beautiful and useful in the actual product.

## Maintenance

Keep the research ledger and the visual proposal separate. Update source dates/status when facts change, and attach contrary findings rather than deleting uncertainty. Revisit browser support and platform recommendations before implementation. Revisit empirical claims when stronger studies or relevant Kinosail task evidence appears.

Future agents should read this reference and the affected app's design contract, inspect the current UI, then make the best supported change. Neither this document nor today's design creates a sacred cow.

### Companion verification

The retired prototype’s historical verification is recorded below. Current interface verification uses the app-specific suites.

On 6 September 2026 the companion passed Chromium 151.0.7922.34, Firefox 153.0, and WebKit 26.5 checks at 1440, 1024, 720, 390, and 320 CSS px in dark and light appearances: 30 axe scans, horizontal reflow, unobscured mobile anchor headings, scenario/density selection, save timing, immediate switch feedback, search/no-results/retained selection, native selection, radio arrow keys, and dialog Escape/focus recovery. Reduced-motion preference changes were checked, and forced-color renders were also produced. Desktop and narrow responsive screenshots were visually inspected. These are prototype engineering checks, not disabled-user research, app-wide certification, or physical-device evidence.
