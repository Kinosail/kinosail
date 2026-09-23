---
name: "Kinosail Subtitles"
description: "A clear Electric workspace for real subtitle status and the next file action."
colors:
  bg: "#0b0d0b"
  surface: "#151914"
  surface-2: "#20271e"
  surface-3: "#2c3428"
  text: "#f6f8f2"
  muted: "#a0a79c"
  line: "#30382c"
  line-strong: "#64705c"
  signal: "#c4ff47"
  signal-ink: "#142000"
  focus: "#daff98"
  danger: "#ffabab"
  success: "#8bd7b1"
  warning: "#f0cb8c"
  info: "#a9c7ee"
  light-bg: "#f4f8ef"
  light-surface: "#fff"
  light-surface-2: "#e9efe2"
  light-surface-3: "#dbe4d2"
  light-text: "#162011"
  light-muted: "#5b6852"
  light-line: "#d4ddcc"
  light-line-strong: "#78866e"
  light-signal: "#3c6100"
  light-signal-ink: "#fff"
  light-focus: "#304e00"
  light-danger: "#a62337"
  light-success: "#23674b"
  light-warning: "#805418"
  light-info: "#305d94"
typography:
  display:
    fontFamily: "Manrope, ui-sans-serif, -apple-system, BlinkMacSystemFont, \"Segoe UI\", sans-serif"
    fontSize: "clamp(2rem,3.5vw,3.25rem)"
    fontWeight: 800
    lineHeight: 1.1
    letterSpacing: "-.035em"
  headline:
    fontFamily: "Manrope, ui-sans-serif, -apple-system, BlinkMacSystemFont, \"Segoe UI\", sans-serif"
    fontSize: "1.65rem"
    fontWeight: 800
    lineHeight: 1.2
    letterSpacing: "-.025em"
  body:
    fontFamily: "Manrope, ui-sans-serif, -apple-system, BlinkMacSystemFont, \"Segoe UI\", sans-serif"
    fontWeight: 400
  label:
    fontFamily: "Manrope, ui-sans-serif, -apple-system, BlinkMacSystemFont, \"Segoe UI\", sans-serif"
    fontWeight: 600
rounded:
  control: "10px"
  quiet: "12px"
  panel: "16px"
  artwork: "12px"
  primary: "999px"
spacing:
  action-gap: ".75rem"
components:
  button-primary:
    backgroundColor: "{colors.signal}"
    textColor: "{colors.signal-ink}"
    rounded: "{rounded.primary}"
    padding: ".7rem 1rem"
  button-quiet:
    backgroundColor: "transparent"
    textColor: "{colors.text}"
    rounded: "{rounded.quiet}"
    padding: ".7rem 1rem"
  input:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.text}"
    rounded: "{rounded.control}"
    padding: ".68rem .85rem"
  navigation:
    textColor: "{colors.signal}"
  file-row:
    backgroundColor: "transparent"
    rounded: "{rounded.control}"
  state-label:
    textColor: "{colors.text}"
---

# Design System: Kinosail Subtitles

## Overview

**Creative North Star: "Electric"**

Electric puts the next subtitle decision beside the affected file. A clear task heading and green action lead; coverage supports that work. Open file rows expose detail on demand while retaining real statuses and recovery paths.

The user selected Electric option 1 across the active web apps, replacing the prior coral system. `packages/webassets/static/last-light.css` owns shared visual roles; `internal/server/static/subtitle-dashboard.css` owns this workspace. Root `PRODUCT.md` preserves subtitle validation, permissions and truthful operation results.

**Key Characteristics:**

- Task-first headings and a distinct green primary action.
- Open file disclosures with real state labels.
- Adaptive semantic surfaces and compact responsive coverage.

## Colors

### Primary

Electric green (`signal`) marks the main action, selected destination and saved-progress line; `signal-ink` supplies its readable foreground. The light appearance uses deep leaf green and white action ink. Pale green focus remains distinct from the selected state.

### Neutral

Black-green canvas and successively lighter green-gray surfaces separate the page, controls and overlays. Off-white text and sage-gray supporting text establish hierarchy. The light counterpart uses pale green paper, white surfaces and dark green text. Use the matching light roles together rather than carrying the dark action color into a light page.

### State

Success, warning, danger and information keep their existing semantic colors and readable labels. Green action color does not prove a service is healthy or an operation succeeded.

**The Action and State Rule.** Use Electric green for action or selection; attach actual state text to operational outcomes.

## Typography

Manrope is the self-hosted variable web family (weights 200–800), served at `/static/manrope.woff2?v=1` with `font-display: swap`. Its source and license live in `packages/webassets/static/fonts/`. Compact negative tracking and balanced wrapping belong to headings; body copy and controls use normal tracking. Supporting counts and time values use tabular numerals.

The frontmatter records reused roles from the current cascade. Apple clients have a separate rounded system-type contract; do not export fixed web sizes into native controls.

## Layout

The sticky desktop header is at least 80px high. The main workspace is capped at 96rem, with `clamp(1.25rem,4vw,4rem)` horizontal padding. The overview combines flexible task copy with a 16rem coverage column and a 3rem gap. Its heading uses `clamp(1.75rem,3vw,2.75rem)`, weight 750 and 1.18 line height; the supporting paragraph uses 1rem with 1.65 line height.

At 900px and below, the header becomes 72px high and the three-destination navigation moves to the safe-area-aware bottom edge. Filters remain sticky beneath the header. Below 700px, overview columns stack, coverage becomes compact supporting text, the large number/meter recedes and primary task actions span the available width. Pending or unavailable state remains explicit.

File disclosures keep at least 100px summary height, a compact artwork strip and adjacent status. Narrow layouts reduce padding and gaps while keeping the title, disclosure and file operations reachable. Filter controls wrap instead of forcing the page wider.

## Elevation & Depth

Tonal surfaces, fine dividers and real content provide depth. Ordinary panels and resting content have no shadow or backdrop blur. Bounded menus and dialogs use the shared overlay shadow; its dark and light values live in the sidecar extensions.

**The Stable Content Rule.** Web hover changes color or framing without lifting content. Shared color transitions last 180ms with `cubic-bezier(.16,1,.3,1)`; reduced-motion rules suppress animation and transitions.

## Shapes

The strong primary action uses the capsule radius. Ordinary fields and controls use the smaller control radius; quiet buttons use their separate radius. Panels and artwork have modest corners. Open rows use dividers and transparent backgrounds instead of a rounded card around every object. Native system components retain their own geometry.

## Components

### Buttons

The main subtitle action uses a green capsule, action ink and a minimum 48px height. It expands across the overview on narrow screens. Header and supporting controls use quiet bounded shapes. Do not use a filled primary treatment for every row or state label.

### Inputs / Fields

Fields keep visible labels, native form behavior and a minimum 44px control height. Shared keyboard focus is a 2px outline with a 4px offset; fields use a 2px offset. At 900px and below, text entry uses at least 16px type. Disabled controls retain their existing disabled semantics and reduced visual emphasis.

### Navigation and filters

The selected destination has green text and a bottom rule. Desktop navigation is horizontal; mobile keeps the three actual destinations at the bottom. Search and filter fields are 50px high, use the surface role and retain their labels and native selection behavior. File disclosure remains a semantic details/summary control with its existing keyboard behavior.

### Cards / Containers

File rows remain open, with separators and a surface-2 fill when expanded. Title and metadata share the available width; the artwork strip uses its compact local geometry (44×64px, 6px corners). Coverage uses a divider rather than a boxed statistic card. The approved caption mark keeps its authored geometry in Electric green.

### Progress, pending and recovery

Ready, wanted, pending and unavailable keep real text alongside their state colors. Coverage numbers and meters describe confirmed server data, not a successful subtitle installation inferred from appearance.

Busy filtering dims the existing controls; loaded, empty and failed results replace the pending state. Keep placeholders only while the corresponding request is pending, preserving the final row geometry. Errors and update notices retain their actual recovery actions.

### Asset provenance

The active app mark is authored SVG geometry, recolored to the approved Electric family. Web PNGs derive from the app's SVG assets; the native raster pipeline uses the authored files under `apps/player/apps/native/assets/source/` and `scripts/generate-brand-assets.sh` in that workspace, with `sips` and `ffmpeg`. No generated raster is a source of truth for logo geometry. Family provenance and active paths are recorded in `engineering/design/cinema-direction.md`.

## Do's and Don'ts

### Do:

- Do use semantic roles in both appearances and retain visible keyboard focus.
- Do keep status, empty, pending and failure states tied to the underlying operation.
- Do preserve real content, accessible labels, native form behavior and reduced-motion support.

### Don't:

- Don't recolor the shell from individual artwork.
- Use only the shared green CinemaSail artwork on the main workspace, dimmed below content. Keep forms, light mode, increased contrast, reduced transparency and forced colors on plain surfaces. Do not lift content on hover.
- Don't turn leftover kicker, glyph-icon or legacy style declarations into new shared patterns.
