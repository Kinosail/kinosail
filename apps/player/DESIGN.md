---
name: "Kinosail Player"
description: "Electric actions, untinted artwork and open resume rows for a personal media library."
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
    fontSize: "clamp(2rem,3.8vw,4rem)"
    fontWeight: 750
    lineHeight: 1.08
    letterSpacing: "-.035em"
  headline:
    fontFamily: "Manrope, ui-sans-serif, -apple-system, BlinkMacSystemFont, \"Segoe UI\", sans-serif"
    fontSize: "clamp(1.35rem,1.7vw,1.75rem)"
    fontWeight: 800
    letterSpacing: "-.025em"
  body:
    fontFamily: "Manrope, ui-sans-serif, -apple-system, BlinkMacSystemFont, \"Segoe UI\", sans-serif"
    fontWeight: 400
    fontSize: "15px"
    lineHeight: 1.5
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
  artwork-card:
    rounded: "{rounded.artwork}"
  resume-row:
    backgroundColor: "transparent"
    textColor: "{colors.text}"
  watch-progress:
    backgroundColor: "{colors.signal}"
    height: "4px"
---

# Design System: Kinosail Player

## Overview

**Creative North Star: "Electric"**

Electric gives household media a direct, personal presentation: recognize the title, see the saved position and use the green action. Artwork stays in its own space; copy and progress remain readable without a scrim. Open continuation rows make the next title easy to find.

The user selected Electric option 1 and authorized replacing the previous coral system. `packages/webassets/static/last-light.css` owns shared visual roles; `internal/server/static/home.css` owns Player composition and the mobile tab editor. Root `PRODUCT.md` preserves privacy, Direct First playback, permissions and real progress. The native contract is `apps/native/DESIGN.md`.

**Key Characteristics:**

- Untinted artwork with separate title, progress and action.
- Electric green actions on flat adaptive surfaces.
- Personal mobile tabs, open resume rows and responsive shelves.

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

The library shell retains its 96rem base width cap and 100rem maximum-width ceiling, centered margins and horizontal padding `clamp(1.25rem,4vw,4rem)`. Above 900px, the sticky header places brand, search and utilities over horizontal navigation. At 900px and below, personal bottom navigation respects the safe area.

Home presents a 1.25:1 artwork/copy column ratio with a fluid 1.5–4rem gap. Artwork occupies an in-flow 16:9 box; there is no forced hero height or image overlay. The title is bounded by its column and wraps. At 900px and below the composition stacks with a 0.75rem gap after the tabs and between artwork and copy; the feature title becomes `clamp(2rem,9vw,3rem)` with a 0.375rem gap before metadata. The progress label sits beside its track, and unknown duration retains the position on its own line. Missing artwork collapses the unused image column. Home sections use 2–2.5rem separation.

Continue watching uses two columns of open rows on wide screens and one below 900px. Each row places a 7rem-wide landscape thumbnail beside the title and saved position, with a thin bar for known watch duration and the action at the far edge. Unknown duration leaves the saved position visible without a fabricated percentage. Poster shelves preserve their media geometry; below 600px their track columns occupy 42% to reveal the next item.

Detail backdrops also remain in flow, beside copy at a 1.2:1 ratio with a 3rem gap; below 700px they stack. Detail titles use `clamp(2.5rem,5vw,5rem)` and 1.05 line height. Setup retains a 16rem progress column and a form capped at 36rem, collapsing below 700px. Base body type becomes 14px below 700px; text-entry sizing remains separately protected.

## Elevation & Depth

Tonal surfaces, fine dividers and real content provide depth. Ordinary panels and resting content have no shadow or backdrop blur. Bounded menus and dialogs use the shared overlay shadow; its dark and light values live in the sidecar extensions.

**The Stable Content Rule.** Web hover changes color or framing without lifting content. Shared color transitions last 180ms with `cubic-bezier(.16,1,.3,1)`; reduced-motion rules suppress animation and transitions.

## Shapes

The strong primary action uses the capsule radius. Ordinary fields and controls use the smaller control radius; quiet buttons use their separate radius. Panels and artwork have modest corners. Open rows use dividers and transparent backgrounds instead of a rounded card around every object. Native system components retain their own geometry.

## Components

### Buttons

Play or Resume is a green capsule with action ink and a minimum 50px height. View details is a quieter raised-surface button with the quiet radius. The action group keeps a 0.75rem gap and the primary uses a content-sized width. Actions wrap onto separate rows when enlarged text needs more space. Hover slightly brightens the action without moving it. Shared primary actions retain their actual routes and permissions.

### Inputs / Fields

Fields keep visible labels, native form behavior and a minimum 44px control height. Shared keyboard focus is a 2px outline with a 4px offset; fields use a 2px offset. At 900px and below, text entry uses at least 16px type. Disabled controls retain their existing disabled semantics and reduced visual emphasis.

### Navigation and personal tabs

Desktop destinations use a green selected rule and existing utility/More menus. Mobile web defaults to Home, TV Shows, Movies and Search plus persistent More. Customize tabs permits adding, removing and reordering one to four destinations; unpinned destinations remain reachable through More. Choices are scoped to the Viewer Profile in this browser. This presentation preference does not alter library access or the desktop navigation.

The tab editor is a native HTML dialog capped at 34rem wide and 85dvh high. Its action rows have 44px minimum controls, action-specific accessible labels and focus restoration. Reorder icons reuse the authored back icon, rotated for earlier/later. Reset restores Home, TV Shows, Movies and Search. Keep the skip link and visible keyboard focus.

### Cards / Containers

Posters retain real media artwork, the artwork radius and media-specific proportions; hover or keyboard focus outlines the poster without moving it. The home feature uses a compact portrait and details composition, with a landscape fallback when only a backdrop is available. Resume rows use portrait artwork and bounded widths, preserving the title, real saved position, direct media action and separate removal action. The featured resume title is omitted from the shelf and keeps its removal action beside the featured controls. Browse destinations are open divider rows. The app's approved sail mark keeps its geometry in Electric green.

### Progress, pending and recovery

Home and detail progress use a 4px green track when valid duration is available and tabular saved-position text. Unknown duration keeps the saved position without inventing a percentage.

During an actual pending navigation request, the existing feature and card boxes become neutral placeholders and retain their layout. A 3px action line marks the pending state. Empty, settled and failed responses must not stay in a skeleton. Keep recovery and permission feedback attached to the real operation.

### Asset provenance

The active app mark is authored SVG geometry, recolored to the approved Electric family. Web PNGs derive from the app's SVG assets; the native raster pipeline uses the authored files under `apps/player/apps/native/assets/source/` and `scripts/generate-brand-assets.sh` in that workspace, with `sips` and `ffmpeg`. No generated raster is a source of truth for logo geometry. Family provenance and active paths are recorded in `engineering/design/cinema-direction.md`.

## Do's and Don'ts

### Do:

- Do use semantic roles in both appearances and retain visible keyboard focus.
- Do keep status, empty, pending and failure states tied to the underlying operation.
- Do preserve real content, accessible labels, native form behavior and reduced-motion support.

### Don't:

- Don't recolor the shell from individual artwork.
- Don't add decorative background imagery to administration or lift web browsing content on hover.
- Don't turn leftover kicker, glyph-icon or legacy style declarations into new shared patterns.
