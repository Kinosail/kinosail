# Modern selection controls for Kinosail

Research date: 2026-09-04

## Recommendation

Do not replace every dropdown with one fashionable component. Replace each control only when another pattern makes the available choices, their consequences, or the current state easier to understand. Keep native HTML semantics underneath visual styling wherever possible: browsers supply keyboard behavior and accessibility mappings for standard controls, while an ARIA role alone supplies neither behavior nor styling ([W3C H91](https://www.w3.org/WAI/WCAG22/Techniques/html/H91), [WAI-ARIA APG: Read Me First](https://www.w3.org/WAI/ARIA/apg/practices/read-me-first/)).

For Kinosail, the default hierarchy should be:

1. Show a small, consequential choice directly as radios or choice cards.
2. Use a segmented control for a compact mode or view switch.
3. Use chips for optional filters or removable selections.
4. Use a searchable combobox only when search materially reduces a long-list task.
5. Use a menu for commands, not for ordinary form values.
6. Put secondary technical detail behind a disclosure, not a selection menu.
7. Use a switch only for an immediate, reversible binary setting.
8. Retain a native select for a long, fixed, low-frequency list when exposing every choice or adding search would make the task worse.

## Decision matrix

| Pattern | Use it when | Do not use it when | Kinosail examples |
| --- | --- | --- | --- |
| Radio group | One value is required from a short fixed list and seeing the alternatives helps comparison. Radios are explicitly for mutually exclusive choices; group them with a `fieldset` and `legend` and label every option ([GOV.UK radios](https://design-system.service.gov.uk/components/radios/), [W3C APG radio group](https://www.w3.org/WAI/ARIA/apg/patterns/radio/)). | The user can select several values, the list is long, or labels need search. | Playback strategy, content-rating boundary, connection behavior. |
| Segmented control | A small set of closely related, compact choices changes the current object, mode, or view. Apple recommends limiting segments to roughly five on iPhone and keeping selection controls distinct from action groups; Material describes single-select and small multi-select variants ([Apple HIG](https://developer.apple.com/design/human-interface-guidelines/segmented-controls), [Android Material 3](https://developer.android.com/develop/ui/compose/components/segmented-button)). | Labels are long, options need explanations, choices do not fit at 320 CSS px, or the segments mix state changes with commands. | Theme, library layout, dashboard time range, sort direction when the vocabulary is unambiguous. |
| Choice cards | A short set of options needs a title plus a concise consequence, capability, or recommendation. A choice card is visual presentation, not a new semantic role: use native radios for one selection and checkboxes for multiple selections. Carbon's selectable tiles preserve radio/checkbox indicators and distinct focus/selection states ([Carbon selectable tile](https://carbondesignsystem.com/components/tile/usage/), [Carbon radio guidance](https://carbondesignsystem.com/components/radio-button/usage/)). | The supporting copy is unnecessary, there are many choices, or the card would contain competing nested actions. | Direct First / Compatibility playback modes, household access policies, metadata-provider strategies. |
| Filter or input chips | Compact optional values refine visible content, or selected tokens need to remain visible and removable. Material distinguishes filter chips from input tokens and recommends a redundant selected indicator such as a checkmark ([Android Material 3 chips](https://developer.android.com/develop/ui/compose/components/chip)). | A choice is required, consequences are important, the set is large, or the chip would replace a clearly labeled form control. | Genre, status, media type, language filters; removable selected subtitle languages. |
| Searchable combobox | The user must find an allowed value in a long or changing collection, or suggestions help complete otherwise free text. An editable combobox filters its popup as the user types ([W3C APG combobox](https://www.w3.org/WAI/ARIA/apg/patterns/combobox/)). | The list is short enough to scan, all options should be compared, or the team cannot implement and test the full keyboard, focus, validation, and mobile behavior. | Subtitle language, large server/library lists, large profile or device lists. |
| Menu button | A compact trigger exposes actions or commands. The button must communicate that it opens a menu and expose its expanded state; Enter or Space opens it and focus moves into the menu ([W3C APG menu button](https://www.w3.org/WAI/ARIA/apg/patterns/menu-button/)). Apple likewise distinguishes an action-oriented pull-down from a mutually exclusive selection pop-up ([Apple HIG pull-down buttons](https://developer.apple.com/design/human-interface-guidelines/pull-down-buttons), [Apple HIG pop-up buttons](https://developer.apple.com/design/human-interface-guidelines/pop-up-buttons)). | The items are ordinary form values, persistent filters, or primary actions that should remain visible. | Overflow/More actions, item actions, administrative commands. |
| Disclosure | A page has short, secondary content that only some users need. Use a native `details`/`summary` where it fits, or a button with `aria-expanded`; Enter and Space toggle the content ([W3C APG disclosure](https://www.w3.org/WAI/ARIA/apg/patterns/disclosure/), [GOV.UK details](https://design-system.service.gov.uk/components/details/)). | The content is required for the task, most users need it, or disclosures would be nested. GOV.UK recommends testing clear headings and visible content before adding accordions ([GOV.UK accordion](https://design-system.service.gov.uk/components/accordion/)). | Advanced transcoding, provider diagnostics, raw identifiers, deployment-managed setting provenance. |
| Switch | A single binary preference applies immediately, is reversible, and is naturally described as on/off. Switches are binary; their accessible label must not change with state ([W3C APG switch](https://www.w3.org/WAI/ARIA/apg/patterns/switch/)). Carbon limits toggles to immediate, reversible settings and recommends a checkbox plus an explicit action when application is deferred ([Carbon toggle](https://carbondesignsystem.com/components/toggle/usage/)). | A Save button is required, there are more than two states, the action is destructive, or confirmation is required. | Autoplay, show subtitle background, enable a reversible local preference. |
| Native select | A single value comes from a long, fixed, low-frequency list, space is constrained, and search adds no meaningful benefit. The HTML element provides name, role, value, keyboard operation, and platform behavior when labeled correctly ([WHATWG HTML](https://html.spec.whatwg.org/multipage/form-elements.html), [W3C H91](https://www.w3.org/WAI/WCAG22/Techniques/html/H91)). | There are only a few important options, users need to compare explanations, or multiple selection is required. GOV.UK treats selects as a last resort in public-facing services and recommends checkboxes rather than `select multiple` ([GOV.UK select](https://design-system.service.gov.uk/components/select/)). | Time zone, locale, and other stable enumerations after testing confirms a searchable picker is unnecessary. |

## Interaction and accessibility contract

These requirements apply regardless of visual treatment.

### Semantics and labels

- Start with `button`, `input`, `select`, `label`, `fieldset`, `legend`, and `details` rather than recreating their behavior with generic elements. W3C states that standard form controls supply keyboard operation and accessibility API mappings ([W3C H91](https://www.w3.org/WAI/WCAG22/Techniques/html/H91)).
- Give every control and every option a visible, descriptive label. Associate labels programmatically; doing so also enlarges the clickable area ([WAI labeling controls](https://www.w3.org/WAI/tutorials/forms/labels/), [WCAG 2.2 labels or instructions](https://www.w3.org/WAI/WCAG22/Understanding/labels-or-instructions.html)).
- Treat “segmented,” “chip,” and “choice card” as appearances layered on the correct radio, checkbox, or button semantics. Selection state and keyboard focus must be visually distinct ([WAI keyboard interface](https://www.w3.org/WAI/ARIA/apg/practices/keyboard-interface/)).
- Do not communicate selected/on state by color alone; add a checkmark, radio indicator, text, shape, or another redundant cue ([Apple HIG toggles](https://developer.apple.com/design/human-interface-guidelines/toggles), [Carbon selectable tile](https://carbondesignsystem.com/components/tile/usage/)).

### Keyboard and popup behavior

- Radios and single-select segmented controls use Tab to enter/leave the group, arrow keys to move among choices, and Space to select ([W3C APG radio group](https://www.w3.org/WAI/ARIA/apg/patterns/radio/)).
- Comboboxes preserve normal text editing. Arrow keys navigate suggestions, Enter accepts, and Escape closes; JavaScript must not capture platform text-editing keys ([W3C APG combobox](https://www.w3.org/WAI/ARIA/apg/patterns/combobox/)).
- Menu buttons open with Enter or Space, move focus into the menu, expose `aria-haspopup` and `aria-expanded`, and return focus predictably when closed ([W3C APG menu button](https://www.w3.org/WAI/ARIA/apg/patterns/menu-button/)).
- Keep focus visible and separate from the selected style. A solid 2 CSS px perimeter is the simplest WCAG 2.2 focus-appearance benchmark ([WCAG 2.2 focus appearance](https://www.w3.org/WAI/WCAG22/Understanding/focus-appearance.html)).
- A selection must not unexpectedly submit, navigate, open a new window, or otherwise change context. If a context change is unavoidable, warn before the control ([WCAG 2.2 on input](https://www.w3.org/WAI/WCAG22/Understanding/on-input)).

### Mobile, touch, and responsive behavior

- Kinosail should keep its existing 44 CSS px touch-target goal. WCAG 2.2 requires at least 24 by 24 CSS px unless a documented exception applies, while Apple recommends a 44 by 44 pt default control size and adequate spacing ([WCAG 2.2 target size](https://www.w3.org/WAI/WCAG22/Understanding/target-size-minimum.html), [Apple HIG accessibility](https://developer.apple.com/design/human-interface-guidelines/accessibility)).
- At 320 CSS px, preserve every label, value, and action without two-dimensional scrolling. When a segmented row no longer fits, change its visual layout to stacked radio rows/cards rather than truncating labels or creating an essential horizontal scroller ([WCAG 2.2 reflow](https://www.w3.org/WAI/WCAG22/Understanding/reflow.html)).
- Do not rely on hover to expose choices or instructions. Any content shown on hover/focus must be dismissible, hoverable, and persistent; explicit tap/click activation is the safer mobile default ([WCAG 2.2 content on hover or focus](https://www.w3.org/WAI/WCAG22/Understanding/content-on-hover-or-focus.html)).
- Test custom ARIA controls with the browser, screen reader, and touch combinations Kinosail supports. APG explicitly warns that its examples do not establish mobile/touch compatibility and that some ARIA features lack mobile-browser support ([WAI-ARIA APG: Read Me First](https://www.w3.org/WAI/ARIA/apg/practices/read-me-first/)).

### Progressive enhancement

- Server-render the native input, its label, the current value, and validation. Styling or JavaScript may enhance the interaction, but loss of JavaScript must not remove the only way to understand or submit a setting ([W3C H91](https://www.w3.org/WAI/WCAG22/Techniques/html/H91)).
- Make a styled radio group or choice-card group submit as ordinary radios; make a styled switch submit as an ordinary checkbox; retain the native select when a custom popup cannot provide an equally robust fallback ([W3C APG radio group](https://www.w3.org/WAI/ARIA/apg/patterns/radio/), [W3C APG switch](https://www.w3.org/WAI/ARIA/apg/patterns/switch/)).
- For enhanced disclosures, keep the underlying content in the document. GOV.UK's accordion deliberately renders all sections as visible headings and content when JavaScript is unavailable ([GOV.UK accordion](https://design-system.service.gov.uk/components/accordion/)).
- For searchable selection, provide a native select or server-backed search/list fallback. Treat the custom combobox as an enhancement, not merely a reskinned select; APG requires substantial focus and keyboard behavior and does not establish mobile/touch compatibility for its examples ([W3C APG combobox](https://www.w3.org/WAI/ARIA/apg/patterns/combobox/), [WAI-ARIA APG: Read Me First](https://www.w3.org/WAI/ARIA/apg/practices/read-me-first/)).

## Kinosail rollout priorities

1. Convert small, high-consequence selects to visible radio rows or choice cards first. This produces the clearest usability gain with native semantics.
2. Convert compact view/mode choices to segmented controls only when all labels fit at every supported width; keep the DOM model a radio group.
3. Convert library filters to wrapping filter chips, with a visible selected marker and a clear “All” or reset action.
4. Introduce one shared searchable-combobox implementation for genuinely long lists instead of several page-specific versions.
5. Keep overflow actions in menus and move advanced technical explanation to disclosures; neither is a substitute for a form control.
6. Audit remaining selects individually. Retention is a valid outcome when the native control is the most robust option.

## Kinosail control audit

The implementation audit covered every production `select` in Dashboard, Player, and Subtitles, plus Player's native clients. The replacements keep native radio semantics and the existing server-side allowlists. No production picker, dropdown, or menu-based value selector was found in the native-client source.

| Area | Replaced | Deliberately retained |
| --- | --- | --- |
| Dashboard | Accent swatches, import format segments, and descriptive household-view cards. | None of the audited fixed-choice selects. |
| Player | Playback-policy cards; subtitle, conversion, theme, profile-type, import-source, content-rating, Media Share lifetime/device-limit groups; DNS-provider cards. | Long or dynamic language, profile, library, device, media-track, codec, accelerator, schedule, playback-rate, sleep, sort, and marker lists. |
| Subtitles | The Player groups above plus preferred subtitle-role segments. | Long or dynamic language, profile, library, device, track, codec, schedule, playback-rate, sleep, sort, and marker lists. |

Existing wrapping filter buttons/chips, command menus, disclosures, checkboxes with explicit Save actions, and immediate buttons already matched their intended patterns. Checkboxes that apply only after Save were not relabeled as switches because that would imply immediate application.

## Acceptance checks for each replacement

- Correct native/ARIA role, accessible name, current value, and selected/expanded/checked state.
- Full keyboard flow, including arrows where the pattern requires them and Escape for popups.
- Visible focus that is not confused with selection.
- Accurate pointer and touch behavior with a 44 CSS px Kinosail target.
- Popups remain within the viewport, do not obscure focused content, and close predictably ([WCAG 2.2 focus not obscured](https://www.w3.org/WAI/WCAG22/Understanding/focus-not-obscured-minimum)).
- 320, 390, and desktop layouts with long translated labels and populated data.
- JavaScript-disabled behavior for server-rendered forms and disclosures.
- Screen-reader smoke tests for each shared custom control; VoiceOver plus Safari is essential for Kinosail's Apple-facing use cases.
- Rejected/invalid values remain server-validated and cause no side effects; changing presentation must not weaken the existing trust boundary.

The quality bar is not “fewer dropdowns.” It is fewer hidden, ambiguous, or unnecessarily difficult decisions, with the retained controls chosen deliberately.
